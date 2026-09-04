// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: (c) 2026 Kryovyx

package openapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	rx "github.com/kryovyx/rextension"
	openapi "github.com/kryovyx/rextension-openapi"
	"github.com/kryovyx/rextension/di"
	rxevent "github.com/kryovyx/rextension/event"
	rxroute "github.com/kryovyx/rextension/route"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

type mockLogger struct{}

func (mockLogger) Info(string, ...interface{})               {}
func (mockLogger) Debug(string, ...interface{})              {}
func (mockLogger) Trace(string, ...interface{})              {}
func (mockLogger) Warn(string, ...interface{})               {}
func (mockLogger) Error(string, ...interface{})              {}
func (mockLogger) SetLogLevel(rx.LogLevel)                   {}
func (l mockLogger) WithField(string, interface{}) rx.Logger { return l }
func (l mockLogger) WithFields(map[string]interface{}) rx.Logger {
	return l
}
func (l mockLogger) WithError(error) rx.Logger { return l }

type mockBus struct{}

func (mockBus) Subscribe(string, rxevent.EventHandler) {}
func (mockBus) Emit(rxevent.Event)                     {}
func (mockBus) SetLogger(rxevent.BusLogger)            {}
func (mockBus) Close()                                 {}

// mockContainer resolves nothing, which is the no-security-extension case.
type mockContainer struct{}

func (mockContainer) Resolve(any) error        { return http.ErrNoLocation }
func (mockContainer) ResolveAll(any) error     { return nil }
func (mockContainer) Singleton(any) error      { return nil }
func (mockContainer) Scoped(any) error         { return nil }
func (mockContainer) Transient(any) error      { return nil }
func (mockContainer) Instance(any) error       { return nil }
func (mockContainer) Unbind(any) (bool, error) { return false, nil }

type registered struct {
	route  rx.Route
	router string
}

type mockRex struct {
	registered  []registered
	registerErr error
}

func newMockRex() *mockRex { return &mockRex{} }

func (m *mockRex) Logger() rx.Logger                        { return mockLogger{} }
func (m *mockRex) Container() rx.Container                  { return mockContainer{} }
func (m *mockRex) EventBus() rxevent.EventBus               { return mockBus{} }
func (m *mockRex) Use(rx.Middleware)                        {}
func (m *mockRex) UseOnRouter(string, rx.Middleware, int)   {}
func (m *mockRex) UsePerRoute(rx.PerRouteMiddleware, int)   {}
func (m *mockRex) UsePerRouter(rx.PerRouterMiddleware, int) {}
func (m *mockRex) RegisterRoute(rt rx.Route) error {
	return m.RegisterRouteToRouter(rt, rx.DefaultRouterName)
}
func (m *mockRex) RegisterRouteToRouter(rt rx.Route, router string) error {
	if m.registerErr != nil {
		return m.registerErr
	}
	m.registered = append(m.registered, registered{route: rt, router: router})
	return nil
}
func (m *mockRex) CreateRouter(string, rx.RouterConfig) error { return nil }

var _ rx.Rex = (*mockRex)(nil)

// stubContext is the minimum rxroute.Context the spec route touches.
type stubContext struct {
	context.Context
	w http.ResponseWriter
	r *http.Request
}

func (c *stubContext) ResponseWriter() http.ResponseWriter { return c.w }
func (c *stubContext) Request() *http.Request              { return c.r }
func (c *stubContext) Resolver() di.Resolver               { return nil }
func (c *stubContext) Respond(status int, contentType string, body interface{}) error {
	c.w.Header().Set("Content-Type", contentType)
	c.w.WriteHeader(status)
	if b, ok := body.([]byte); ok {
		_, err := c.w.Write(b)
		return err
	}
	return json.NewEncoder(c.w).Encode(body)
}
func (c *stubContext) Text(int, string) error             { return nil }
func (c *stubContext) JSON(int, interface{}) error        { return nil }
func (c *stubContext) OpenMetrics(int, interface{}) error { return nil }
func (c *stubContext) Param(string) string                { return "" }
func (c *stubContext) SetValue(interface{}, interface{})  {}
func (c *stubContext) GetValue(k interface{}) interface{} { return c.Value(k) }

var _ rxroute.Context = (*stubContext)(nil)

// serve invokes a route's handler and returns the recorded response.
// rx.Route is deliberately narrower than rxroute.Route — it carries Method and
// Path only — so serving the spec means asserting back to the handler-bearing
// interface the router actually stores.
func serve(t *testing.T, rt rx.Route) *httptest.ResponseRecorder {
	t.Helper()
	h, ok := rt.(rxroute.Route)
	if !ok {
		t.Fatalf("the registered spec route does not implement rxroute.Route: %T", rt)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	h.Handler()(&stubContext{Context: req.Context(), w: rec, r: req})
	return rec
}

// ---------------------------------------------------------------------------
// Documented routes
// ---------------------------------------------------------------------------

type docRoute struct {
	method string
	path   string
	opID   string
}

func (d docRoute) Method() string               { return d.method }
func (d docRoute) Path() string                 { return d.path }
func (d docRoute) Handler() rxroute.HandlerFunc { return func(rxroute.Context) {} }
func (d docRoute) OperationID() string          { return d.opID }
func (d docRoute) Summary() string              { return "summary of " + d.opID }
func (d docRoute) Description() string          { return "description of " + d.opID }
func (d docRoute) Tags() []string               { return []string{"test"} }

// undocumentedRoute implements Route but not OpenAPIRoute.
type undocumentedRoute struct{ path string }

func (p undocumentedRoute) Method() string               { return http.MethodGet }
func (p undocumentedRoute) Path() string                 { return p.path }
func (p undocumentedRoute) Handler() rxroute.HandlerFunc { return func(rxroute.Context) {} }

// boot runs the startup sequence and returns the extension and the spec route.
func boot(t *testing.T, cfg *openapi.Config, routes []rx.RouteInfo) (*mockRex, rx.Route) {
	t.Helper()
	ext := openapi.NewOpenAPIExtension(cfg)
	r := newMockRex()
	if err := ext.OnInitialize(context.Background(), r); err != nil {
		t.Fatalf("OnInitialize: %v", err)
	}
	if err := ext.OnStart(context.Background(), r); err != nil {
		t.Fatalf("OnStart: %v", err)
	}
	v, ok := ext.(rx.RouteValidator)
	if !ok {
		t.Fatal("the extension must implement rextension.RouteValidator — that is how it receives the route table")
	}
	if err := v.ValidateRoutes(routes); err != nil {
		t.Fatalf("ValidateRoutes: %v", err)
	}
	if len(r.registered) != 1 {
		t.Fatalf("expected exactly one registered route, got %d", len(r.registered))
	}
	return r, r.registered[0].route
}

// paths decodes the served document and returns its "paths" object.
func paths(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("spec route returned %d: %s", rec.Code, rec.Body.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("document is not valid JSON: %v\n%s", err, rec.Body.String())
	}
	p, _ := doc["paths"].(map[string]any)
	return p
}

// ---------------------------------------------------------------------------
// The regression this file exists for
// ---------------------------------------------------------------------------

// The document is generated from the route table handed to ValidateRoutes.
//
// It used to be built from routes gathered off the route-registered event,
// which the bus dispatches asynchronously while generation ran synchronously.
// The spec came back with zero paths depending purely on scheduling — and in a
// small app the events usually drained in time, so it looked fine.
func TestValidateRoutes_generates_a_document_containing_every_route(t *testing.T) {
	routes := []rx.RouteInfo{
		{Route: docRoute{http.MethodGet, "/users/{userId}", "getUser"}, Router: rx.DefaultRouterName},
		{Route: docRoute{http.MethodPost, "/users", "createUser"}, Router: rx.DefaultRouterName},
		{Route: docRoute{http.MethodGet, "/health", "health"}, Router: ""}, // empty == default
	}
	_, spec := boot(t, openapi.NewConfig(), routes)

	got := paths(t, serve(t, spec))
	for _, want := range []string{"/users/{userId}", "/users", "/health"} {
		if _, ok := got[want]; !ok {
			t.Errorf("path %q missing from the document; got %v", want, keys(got))
		}
	}
	if len(got) != 3 {
		t.Errorf("expected 3 paths, got %d: %v", len(got), keys(got))
	}
}

// An empty route table is not an error — but it must produce an empty document
// rather than a failure, and the route must still serve.
func TestValidateRoutes_with_no_routes(t *testing.T) {
	_, spec := boot(t, openapi.NewConfig(), nil)
	if got := paths(t, serve(t, spec)); len(got) != 0 {
		t.Fatalf("expected no paths, got %v", keys(got))
	}
}

// ---------------------------------------------------------------------------
// Router filtering (D41)
// ---------------------------------------------------------------------------

// The default is the default router alone. A route on an internal listener
// used to be published in the document served on the public one.
func TestCollect_excludes_other_routers_by_default(t *testing.T) {
	routes := []rx.RouteInfo{
		{Route: docRoute{http.MethodGet, "/public", "public"}, Router: rx.DefaultRouterName},
		{Route: docRoute{http.MethodGet, "/admin/secrets", "secrets"}, Router: "internal"},
	}
	_, spec := boot(t, openapi.NewConfig(), routes)

	got := paths(t, serve(t, spec))
	if _, leaked := got["/admin/secrets"]; leaked {
		t.Fatal("a route from the internal router was published in the document")
	}
	if _, ok := got["/public"]; !ok {
		t.Fatalf("the default router's route is missing: %v", keys(got))
	}
}

func TestCollect_router_filters(t *testing.T) {
	routes := []rx.RouteInfo{
		{Route: docRoute{http.MethodGet, "/public", "public"}, Router: rx.DefaultRouterName},
		{Route: docRoute{http.MethodGet, "/internal", "internal"}, Router: "internal"},
		{Route: docRoute{http.MethodGet, "/admin", "admin"}, Router: "admin"},
	}
	cases := []struct {
		name string
		cfg  *openapi.Config
		want []string
	}{
		{"include a named router", openapi.NewConfig(openapi.WithIncludeRouters("internal")), []string{"/internal"}},
		{"include several", openapi.NewConfig(openapi.WithIncludeRouters(rx.DefaultRouterName, "admin")), []string{"/public", "/admin"}},
		{"include everything", openapi.NewConfig(openapi.WithIncludeRouters("*")), []string{"/public", "/internal", "/admin"}},
		// Exclude is applied after include, so it wins over "*".
		{"exclude wins over wildcard", openapi.NewConfig(
			openapi.WithIncludeRouters("*"),
			openapi.WithExcludeRouters("internal"),
		), []string{"/public", "/admin"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, spec := boot(t, tc.cfg, routes)
			got := paths(t, serve(t, spec))
			if len(got) != len(tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, keys(got))
			}
			for _, w := range tc.want {
				if _, ok := got[w]; !ok {
					t.Errorf("missing %q; got %v", w, keys(got))
				}
			}
		})
	}
}

// A route that does not implement OpenAPIRoute has nothing to document.
func TestCollect_skips_routes_that_are_not_documented(t *testing.T) {
	routes := []rx.RouteInfo{
		{Route: docRoute{http.MethodGet, "/documented", "documented"}, Router: rx.DefaultRouterName},
		{Route: undocumentedRoute{path: "/undocumented"}, Router: rx.DefaultRouterName},
	}
	_, spec := boot(t, openapi.NewConfig(), routes)

	got := paths(t, serve(t, spec))
	if _, ok := got["/undocumented"]; ok {
		t.Fatal("a route not implementing OpenAPIRoute was documented")
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 path, got %v", keys(got))
	}
}

// ValidateRoutes is called on every build. A second call must replace the
// previous route set, not append to it.
func TestValidateRoutes_is_idempotent(t *testing.T) {
	routes := []rx.RouteInfo{
		{Route: docRoute{http.MethodGet, "/a", "a"}, Router: rx.DefaultRouterName},
	}
	ext := openapi.NewOpenAPIExtension(openapi.NewConfig())
	r := newMockRex()
	if err := ext.OnInitialize(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if err := ext.OnStart(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	v := ext.(rx.RouteValidator)
	for i := 0; i < 3; i++ {
		if err := v.ValidateRoutes(routes); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	got := paths(t, serve(t, r.registered[0].route))
	if len(got) != 1 {
		t.Fatalf("routes accumulated across calls: %v", keys(got))
	}
}

// ---------------------------------------------------------------------------
// Registration and serving
// ---------------------------------------------------------------------------

func TestOnStart_registers_the_spec_route_on_the_configured_router(t *testing.T) {
	ext := openapi.NewOpenAPIExtension(openapi.NewConfig(
		openapi.WithServePath("/docs/openapi.json"),
		openapi.WithServeOnRouter("internal"),
	))
	r := newMockRex()
	if err := ext.OnInitialize(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if err := ext.OnStart(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if len(r.registered) != 1 {
		t.Fatalf("expected one registration, got %d", len(r.registered))
	}
	if r.registered[0].router != "internal" {
		t.Errorf("registered on router %q, want internal", r.registered[0].router)
	}
	if got := r.registered[0].route.Path(); got != "/docs/openapi.json" {
		t.Errorf("spec path = %q", got)
	}
	if got := r.registered[0].route.Method(); got != http.MethodGet {
		t.Errorf("spec method = %q", got)
	}
}

func TestOnStart_propagates_a_registration_failure(t *testing.T) {
	ext := openapi.NewOpenAPIExtension(openapi.NewConfig())
	r := newMockRex()
	r.registerErr = http.ErrHandlerTimeout
	if err := ext.OnInitialize(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if err := ext.OnStart(context.Background(), r); err == nil {
		t.Fatal("a failed spec-route registration must fail the boot, not be swallowed")
	}
}

// Serving before the document is generated cannot happen through the normal
// ordering, but a truncated spec is worse than none — so the route says so
// rather than emitting a half-built document.
func TestSpecRoute_before_generation_returns_503(t *testing.T) {
	ext := openapi.NewOpenAPIExtension(openapi.NewConfig())
	r := newMockRex()
	if err := ext.OnInitialize(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if err := ext.OnStart(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	rec := serve(t, r.registered[0].route)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}
}

// ValidateRoutes before OnStart is not an error: the extension may be present
// but never started.
func TestValidateRoutes_without_OnStart_is_a_noop(t *testing.T) {
	ext := openapi.NewOpenAPIExtension(openapi.NewConfig())
	r := newMockRex()
	if err := ext.OnInitialize(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if err := ext.(rx.RouteValidator).ValidateRoutes(nil); err != nil {
		t.Fatalf("expected a no-op, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Construction and the remaining lifecycle
// ---------------------------------------------------------------------------

// A nil config takes the defaults; a non-nil one is used verbatim (D22/O6).
func TestNewOpenAPIExtension_nil_config_uses_defaults(t *testing.T) {
	ext := openapi.NewOpenAPIExtension(nil)
	r := newMockRex()
	if err := ext.OnInitialize(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if err := ext.OnStart(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if got := r.registered[0].route.Path(); got != openapi.NewDefaultConfig().ServePath {
		t.Fatalf("spec path = %q, want the default", got)
	}
}

func TestWithOpenAPI_returns_an_option(t *testing.T) {
	if openapi.WithOpenAPI(openapi.NewConfig()) == nil {
		t.Fatal("WithOpenAPI returned a nil Option")
	}
}

func TestLifecycle_noops(t *testing.T) {
	ext := openapi.NewOpenAPIExtension(openapi.NewConfig())
	r := newMockRex()
	ctx := context.Background()
	if err := ext.OnReady(ctx, r); err != nil {
		t.Errorf("OnReady: %v", err)
	}
	if err := ext.OnStop(ctx, r); err != nil {
		t.Errorf("OnStop: %v", err)
	}
	if err := ext.OnShutdown(ctx, r); err != nil {
		t.Errorf("OnShutdown: %v", err)
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// ---------------------------------------------------------------------------
// Security scheme discovery (a soft dependency)
// ---------------------------------------------------------------------------

type stubScheme struct{ name string }

func (s stubScheme) Name() string        { return s.name }
func (s stubScheme) Type() string        { return "http" }
func (s stubScheme) Description() string { return "stub" }
func (s stubScheme) Challenge() string   { return "Bearer" }

// stubRegistry is the rextension.SchemeRegistry an application with a security
// extension would publish into the container.
type stubRegistry struct{ schemes []rx.SecuritySchemeAccessor }

func (r *stubRegistry) Register(s ...rx.SecuritySchemeAccessor) { r.schemes = append(r.schemes, s...) }
func (r *stubRegistry) Schemes() []rx.SecuritySchemeAccessor    { return r.schemes }
func (r *stubRegistry) Lookup(name string) (rx.SecuritySchemeAccessor, bool) {
	for _, s := range r.schemes {
		if s.Name() == name {
			return s, true
		}
	}
	return nil, false
}

// registryContainer resolves a SchemeRegistry, the way a container does once
// the security extension has registered one.
type registryContainer struct {
	mockContainer
	registry rx.SchemeRegistry
}

func (c *registryContainer) Resolve(target any) error {
	ptr, ok := target.(*rx.SchemeRegistry)
	if !ok || c.registry == nil {
		return http.ErrNoLocation
	}
	*ptr = c.registry
	return nil
}

type registryRex struct {
	*mockRex
	container rx.Container
}

func (r *registryRex) Container() rx.Container { return r.container }

func bootWithContainer(t *testing.T, c rx.Container, routes []rx.RouteInfo) rx.Route {
	t.Helper()
	ext := openapi.NewOpenAPIExtension(openapi.NewConfig())
	base := newMockRex()
	r := &registryRex{mockRex: base, container: c}

	if err := ext.OnInitialize(context.Background(), r); err != nil {
		t.Fatalf("OnInitialize: %v", err)
	}
	if err := ext.OnStart(context.Background(), r); err != nil {
		t.Fatalf("OnStart: %v", err)
	}
	if err := ext.(rx.RouteValidator).ValidateRoutes(routes); err != nil {
		t.Fatalf("ValidateRoutes: %v", err)
	}
	return base.registered[0].route
}

// Schemes registered by the security extension reach the document through the
// container, so neither module imports the other (D21).
func TestSecuritySchemes_are_documented_from_the_container(t *testing.T) {
	c := &registryContainer{registry: &stubRegistry{
		schemes: []rx.SecuritySchemeAccessor{stubScheme{name: "bearer"}, stubScheme{name: "apikey"}},
	}}
	spec := bootWithContainer(t, c, []rx.RouteInfo{
		{Route: docRoute{http.MethodGet, "/private", "getPrivate"}, Router: rx.DefaultRouterName},
	})

	var doc map[string]any
	if err := json.Unmarshal(serve(t, spec).Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	components, _ := doc["components"].(map[string]any)
	schemes, _ := components["securitySchemes"].(map[string]any)
	for _, want := range []string{"bearer", "apikey"} {
		if _, ok := schemes[want]; !ok {
			t.Errorf("scheme %q is missing from the document: %v", want, schemes)
		}
	}
}

// An empty registry documents no schemes, and is not an error.
func TestSecuritySchemes_empty_registry(t *testing.T) {
	c := &registryContainer{registry: &stubRegistry{}}
	spec := bootWithContainer(t, c, []rx.RouteInfo{
		{Route: docRoute{http.MethodGet, "/x", "getX"}, Router: rx.DefaultRouterName},
	})
	if serve(t, spec).Code != http.StatusOK {
		t.Fatal("an empty scheme registry broke document generation")
	}
}

// An application with no security extension resolves nothing and documents no
// schemes, which is a soft dependency working as intended rather than a
// failure.
func TestSecuritySchemes_absent_registry(t *testing.T) {
	spec := bootWithContainer(t, &registryContainer{registry: nil}, []rx.RouteInfo{
		{Route: docRoute{http.MethodGet, "/x", "getX"}, Router: rx.DefaultRouterName},
	})

	var doc map[string]any
	if err := json.Unmarshal(serve(t, spec).Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if components, ok := doc["components"].(map[string]any); ok {
		if schemes, ok := components["securitySchemes"]; ok {
			t.Fatalf("schemes were documented with no registry available: %v", schemes)
		}
	}
}

// ---------------------------------------------------------------------------
// The spec route's default path
// ---------------------------------------------------------------------------

// An empty ServePath falls back to /openapi.json rather than registering a
// route at "", which the router would reject.
func TestSpecRoute_empty_path_falls_back(t *testing.T) {
	ext := openapi.NewOpenAPIExtension(openapi.NewConfig(openapi.WithServePath("")))
	r := newMockRex()
	if err := ext.OnInitialize(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if err := ext.OnStart(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if got := r.registered[0].route.Path(); got != "/openapi.json" {
		t.Fatalf("spec path = %q, want the /openapi.json fallback", got)
	}
}
