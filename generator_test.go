package openapi_test

import (
	"encoding/json"
	"testing"

	rx "github.com/kryovyx/rextension"
	openapi "github.com/kryovyx/rextension-openapi"
	rxroute "github.com/kryovyx/rextension/route"
)

// infos wraps bare routes as RouteInfo on the default router with no base
// path — the shape most of these tests assert about. The base-path cases
// below build RouteInfo directly.
func infos(routes ...rxroute.Route) []rx.RouteInfo {
	out := make([]rx.RouteInfo, 0, len(routes))
	for _, rt := range routes {
		out = append(out, rx.RouteInfo{Route: rt, Router: rx.DefaultRouterName})
	}
	return out
}

// ---- mock types ----

// mockRoute implements route.Route + OpenAPIRoute
type mockRoute struct {
	method      string
	path        string
	operationID string
	summary     string
	description string
	tags        []string
}

func (r *mockRoute) Handler() rxroute.HandlerFunc { return func(rxroute.Context) {} }

func (r *mockRoute) Method() string      { return r.method }
func (r *mockRoute) Path() string        { return r.path }
func (r *mockRoute) OperationID() string { return r.operationID }
func (r *mockRoute) Summary() string     { return r.summary }
func (r *mockRoute) Description() string { return r.description }
func (r *mockRoute) Tags() []string      { return r.tags }

// mockSecuredRoute implements route.Route + OpenAPIRoute + SecuredRouteAccessor
type mockSecuredRoute struct {
	mockRoute
	schemes []string
}

func (r *mockSecuredRoute) RequiredSchemes() []string { return r.schemes }

// mockSecurityScheme implements SecuritySchemeAccessor
type mockSecurityScheme struct {
	name        string
	typ         string
	description string
	challenge   string
}

func (s *mockSecurityScheme) Name() string        { return s.name }
func (s *mockSecurityScheme) Type() string        { return s.typ }
func (s *mockSecurityScheme) Description() string { return s.description }
func (s *mockSecurityScheme) Challenge() string   { return s.challenge }

// mockBearerScheme also implements BearerFormat()
type mockBearerScheme struct {
	mockSecurityScheme
	bearerFormat string
}

func (s *mockBearerScheme) BearerFormat() string { return s.bearerFormat }

// mockAPIKeyScheme implements ParamName + Location
type mockAPIKeyScheme struct {
	mockSecurityScheme
	paramName string
	location  string
}

func (s *mockAPIKeyScheme) ParamName() string { return s.paramName }
func (s *mockAPIKeyScheme) Location() string  { return s.location }

// plainRoute only implements route.Route, not OpenAPIRoute (should be skipped)
type plainRoute struct {
	method string
	path   string
}

func (r *plainRoute) Method() string               { return r.method }
func (r *plainRoute) Path() string                 { return r.path }
func (r *plainRoute) Handler() rxroute.HandlerFunc { return func(rxroute.Context) {} }

// BodySchema mocks for validation enrichment.
//
// Kind returns rextension.SchemaKind, and the route returns
// rextension.BodySchema — the real contract. These used to be duck-typed
// (Kind() int, RequestBody() *testBodySchema), and passed only because the
// generator reached them through reflect.MethodByName, which does not care
// about types. A mock that cannot satisfy the interface real routes implement
// is not testing the same code path (D20).
type testBodySchema struct {
	kind  openapi.SchemaKind
	types []interface{}
}

func (b *testBodySchema) Kind() openapi.SchemaKind { return b.kind }
func (b *testBodySchema) Types() []interface{}     { return b.types }

var _ openapi.BodySchemaAccessor = (*testBodySchema)(nil)

// mockValidatedRoute implements route.Route + OpenAPIRoute + the body-schema
// contract.
type mockValidatedRoute struct {
	mockRoute
	reqBody   *testBodySchema
	responses map[int]*testBodySchema
}

func (r *mockValidatedRoute) RequestBody() openapi.BodySchemaAccessor {
	if r.reqBody == nil {
		// A typed nil in an interface is not nil, so return an untyped one:
		// "no request body" has to read as nil to the generator.
		return nil
	}
	return r.reqBody
}

func (r *mockValidatedRoute) Responses() map[int]openapi.BodySchemaAccessor {
	if r.responses == nil {
		return nil
	}
	out := make(map[int]openapi.BodySchemaAccessor, len(r.responses))
	for status, schema := range r.responses {
		if schema == nil {
			// Same reason as RequestBody above: a typed nil in an interface
			// is not nil, so a nil entry has to be stored untyped for "no
			// schema for this status" to read as nil to the generator.
			out[status] = nil
			continue
		}
		out[status] = schema
	}
	return out
}

var _ openapi.ValidatableRouteAccessor = (*mockValidatedRoute)(nil)

// mockRouteWithExamples implements RequestBodyExamplesProvider + ResponseExamplesProvider + ResponseDescriptionProvider
type mockRouteWithExamples struct {
	mockValidatedRoute
	reqExamples  map[string]openapi.ExampleObject
	respExamples map[int]map[string]openapi.ExampleObject
	respDescs    map[int]string
}

func (r *mockRouteWithExamples) RequestBodyExamples() map[string]openapi.ExampleObject {
	return r.reqExamples
}
func (r *mockRouteWithExamples) ResponseExamples() map[int]map[string]openapi.ExampleObject {
	return r.respExamples
}
func (r *mockRouteWithExamples) ResponseDescriptions() map[int]string {
	return r.respDescs
}

// ---- Generator tests ----

func TestGenerator_Generate_BasicRoute(t *testing.T) {
	cfg := openapi.Config{
		Title:   "Test API",
		Version: "1.0.0",
	}
	gen := openapi.NewGenerator(cfg, nil)

	routes := []rxroute.Route{
		&mockRoute{
			method:      "GET",
			path:        "/users",
			operationID: "listUsers",
			summary:     "List users",
			description: "Returns all users",
			tags:        []string{"users"},
		},
	}

	doc, err := gen.Generate(infos(routes...))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if doc.OpenAPI != "3.1.0" {
		t.Errorf("expected openapi 3.1.0, got %q", doc.OpenAPI)
	}
	if doc.Info.Title != "Test API" {
		t.Errorf("expected title 'Test API', got %q", doc.Info.Title)
	}
	if doc.Info.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %q", doc.Info.Version)
	}

	pi, ok := doc.Paths["/users"]
	if !ok {
		t.Fatal("expected /users path")
	}
	if pi.Get == nil {
		t.Fatal("expected GET operation")
	}
	if pi.Get.OperationID != "listUsers" {
		t.Errorf("expected operationID 'listUsers', got %q", pi.Get.OperationID)
	}
	if pi.Get.Summary != "List users" {
		t.Errorf("expected summary 'List users', got %q", pi.Get.Summary)
	}
}

func TestGenerator_Generate_SkipsNonOpenAPIRoute(t *testing.T) {
	cfg := openapi.Config{Title: "T", Version: "1"}
	gen := openapi.NewGenerator(cfg, nil)

	routes := []rxroute.Route{
		&plainRoute{method: "GET", path: "/health"},
	}

	doc, err := gen.Generate(infos(routes...))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(doc.Paths) != 0 {
		t.Errorf("expected no paths, got %d", len(doc.Paths))
	}
}

func TestGenerator_Generate_DefaultResponse(t *testing.T) {
	cfg := openapi.Config{Title: "T", Version: "1"}
	gen := openapi.NewGenerator(cfg, nil)

	routes := []rxroute.Route{
		&mockRoute{method: "GET", path: "/test", operationID: "test"},
	}

	doc, _ := gen.Generate(infos(routes...))

	op := doc.Paths["/test"].Get
	if _, ok := op.Responses["200"]; !ok {
		t.Error("expected default 200 response")
	}
	if op.Responses["200"].Description != "Successful response" {
		t.Errorf("unexpected response description: %q", op.Responses["200"].Description)
	}
}

func TestGenerator_Generate_AllHTTPMethods(t *testing.T) {
	cfg := openapi.Config{Title: "T", Version: "1"}
	gen := openapi.NewGenerator(cfg, nil)

	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	routes := make([]rxroute.Route, len(methods))
	for i, m := range methods {
		routes[i] = &mockRoute{method: m, path: "/test", operationID: m + "Op"}
	}

	doc, _ := gen.Generate(infos(routes...))

	pi := doc.Paths["/test"]
	if pi.Get == nil {
		t.Error("expected GET operation")
	}
	if pi.Post == nil {
		t.Error("expected POST operation")
	}
	if pi.Put == nil {
		t.Error("expected PUT operation")
	}
	if pi.Patch == nil {
		t.Error("expected PATCH operation")
	}
	if pi.Delete == nil {
		t.Error("expected DELETE operation")
	}
	if pi.Head == nil {
		t.Error("expected HEAD operation")
	}
	if pi.Options == nil {
		t.Error("expected OPTIONS operation")
	}
}

func TestGenerator_Generate_MultiplePaths(t *testing.T) {
	cfg := openapi.Config{Title: "T", Version: "1"}
	gen := openapi.NewGenerator(cfg, nil)

	routes := []rxroute.Route{
		&mockRoute{method: "GET", path: "/a", operationID: "getA"},
		&mockRoute{method: "POST", path: "/b", operationID: "postB"},
	}

	doc, _ := gen.Generate(infos(routes...))

	if len(doc.Paths) != 2 {
		t.Errorf("expected 2 paths, got %d", len(doc.Paths))
	}
	if doc.Paths["/a"] == nil {
		t.Error("expected /a path")
	}
	if doc.Paths["/b"] == nil {
		t.Error("expected /b path")
	}
}

// ---- Security enrichment ----

func TestGenerator_Generate_SecuritySchemeHTTPBearer(t *testing.T) {
	cfg := openapi.Config{Title: "T", Version: "1"}
	schemes := []openapi.SecuritySchemeAccessor{
		&mockBearerScheme{
			name:         "bearer",
			typ:          "http",
			description:  "Bearer token auth",
			challenge:    "Bearer",
			bearerFormat: "JWT",
		},
	}
	gen := openapi.NewGenerator(cfg, schemes)

	doc, _ := gen.Generate(nil)

	if doc.Components == nil || doc.Components.SecuritySchemes == nil {
		t.Fatal("expected security schemes in components")
	}
	ss, ok := doc.Components.SecuritySchemes["bearer"]
	if !ok {
		t.Fatal("expected 'bearer' scheme")
	}
	if ss.Type != "http" {
		t.Errorf("expected type 'http', got %q", ss.Type)
	}
	if ss.Scheme != "bearer" {
		t.Errorf("expected scheme 'bearer', got %q", ss.Scheme)
	}
	if ss.BearerFormat != "JWT" {
		t.Errorf("expected bearerFormat 'JWT', got %q", ss.BearerFormat)
	}
}

func TestGenerator_Generate_SecuritySchemeHTTPBasic(t *testing.T) {
	cfg := openapi.Config{Title: "T", Version: "1"}
	schemes := []openapi.SecuritySchemeAccessor{
		&mockSecurityScheme{
			name:      "basic",
			typ:       "http",
			challenge: "Basic",
		},
	}
	gen := openapi.NewGenerator(cfg, schemes)

	doc, _ := gen.Generate(nil)

	ss := doc.Components.SecuritySchemes["basic"]
	if ss.Scheme != "basic" {
		t.Errorf("expected scheme 'basic', got %q", ss.Scheme)
	}
}

func TestGenerator_Generate_SecuritySchemeAPIKey(t *testing.T) {
	cfg := openapi.Config{Title: "T", Version: "1"}
	schemes := []openapi.SecuritySchemeAccessor{
		&mockAPIKeyScheme{
			name:      "apiKey",
			typ:       "apiKey",
			paramName: "X-API-Key",
			location:  "header",
		},
	}
	gen := openapi.NewGenerator(cfg, schemes)

	doc, _ := gen.Generate(nil)

	ss := doc.Components.SecuritySchemes["apiKey"]
	if ss.Name != "X-API-Key" {
		t.Errorf("expected name 'X-API-Key', got %q", ss.Name)
	}
	if ss.In != "header" {
		t.Errorf("expected in 'header', got %q", ss.In)
	}
}

func TestGenerator_Generate_SecuritySchemeAPIKeyCookie(t *testing.T) {
	cfg := openapi.Config{Title: "T", Version: "1"}
	schemes := []openapi.SecuritySchemeAccessor{
		&mockAPIKeyScheme{
			name:        "sessionCookie",
			typ:         "apiKey",
			description: "BFF session cookie",
			paramName:   "session_id",
			location:    "cookie",
		},
	}
	gen := openapi.NewGenerator(cfg, schemes)

	doc, _ := gen.Generate(nil)

	ss, ok := doc.Components.SecuritySchemes["sessionCookie"]
	if !ok {
		t.Fatal("expected 'sessionCookie' in SecuritySchemes")
	}
	if ss.Type != "apiKey" {
		t.Errorf("expected type 'apiKey', got %q", ss.Type)
	}
	if ss.In != "cookie" {
		t.Errorf("expected in 'cookie', got %q", ss.In)
	}
	if ss.Name != "session_id" {
		t.Errorf("expected name 'session_id', got %q", ss.Name)
	}
}

func TestGenerator_Generate_RouteWithSecurity(t *testing.T) {
	cfg := openapi.Config{Title: "T", Version: "1"}
	schemes := []openapi.SecuritySchemeAccessor{
		&mockSecurityScheme{name: "bearer", typ: "http", challenge: "Bearer"},
	}
	gen := openapi.NewGenerator(cfg, schemes)

	routes := []rxroute.Route{
		&mockSecuredRoute{
			method: "GET", path: "/secure", operationID: "secureOp",
			schemes: []string{"bearer"},
		},
	}

	doc, _ := gen.Generate(infos(routes...))

	op := doc.Paths["/secure"].Get
	if len(op.Security) != 1 {
		t.Fatalf("expected 1 security requirement, got %d", len(op.Security))
	}
	if _, ok := op.Security[0]["bearer"]; !ok {
		t.Error("expected 'bearer' in security requirement")
	}
}

func TestGenerator_Generate_RouteWithEmptySchemes(t *testing.T) {
	cfg := openapi.Config{Title: "T", Version: "1"}
	gen := openapi.NewGenerator(cfg, nil)

	routes := []rxroute.Route{
		&mockSecuredRoute{
			method: "GET", path: "/public", operationID: "publicOp",
			schemes: nil,
		},
	}

	doc, _ := gen.Generate(infos(routes...))

	op := doc.Paths["/public"].Get
	if len(op.Security) != 0 {
		t.Errorf("expected no security for empty schemes, got %d", len(op.Security))
	}
}

// ---- Validation enrichment ----

type ReqBody struct {
	Name string `json:"name"`
}

type RespBody struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func TestGenerator_Generate_WithValidation(t *testing.T) {
	cfg := openapi.Config{Title: "T", Version: "1"}
	gen := openapi.NewGenerator(cfg, nil)

	routes := []rxroute.Route{
		&mockValidatedRoute{
			method: "POST", path: "/users", operationID: "createUser",
			reqBody: &testBodySchema{kind: openapi.SchemaScalar, types: []interface{}{ReqBody{}}},
			responses: map[int]*testBodySchema{
				201: {kind: openapi.SchemaScalar, types: []interface{}{RespBody{}}},
			},
		},
	}

	doc, err := gen.Generate(infos(routes...))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	op := doc.Paths["/users"].Post
	if op.RequestBody == nil {
		t.Fatal("expected request body")
	}
	if !op.RequestBody.Required {
		t.Error("expected required request body")
	}
	if _, ok := op.RequestBody.Content["application/json"]; !ok {
		t.Error("expected application/json content type")
	}

	if _, ok := op.Responses["201"]; !ok {
		t.Error("expected 201 response")
	}
}

func TestGenerator_Generate_NilRequestBody(t *testing.T) {
	cfg := openapi.Config{Title: "T", Version: "1"}
	gen := openapi.NewGenerator(cfg, nil)

	routes := []rxroute.Route{
		&mockValidatedRoute{
			method: "GET", path: "/items", operationID: "listItems",
			reqBody:   nil,
			responses: nil,
		},
	}

	doc, _ := gen.Generate(infos(routes...))

	op := doc.Paths["/items"].Get
	if op.RequestBody != nil {
		t.Error("expected no request body for nil")
	}
}

// ---- Examples providers ----

func TestGenerator_Generate_WithExamples(t *testing.T) {
	cfg := openapi.Config{Title: "T", Version: "1"}
	gen := openapi.NewGenerator(cfg, nil)

	routes := []rxroute.Route{
		&mockRouteWithExamples{
			method:      "POST",
			path:        "/pay",
			operationID: "pay",
			reqBody:     &testBodySchema{kind: openapi.SchemaScalar, types: []interface{}{ReqBody{}}},
			responses: map[int]*testBodySchema{
				200: {kind: openapi.SchemaScalar, types: []interface{}{RespBody{}}},
			},
			reqExamples: map[string]openapi.ExampleObject{
				"example1": {Summary: "Example 1", Value: map[string]string{"name": "foo"}},
			},
			respExamples: map[int]map[string]openapi.ExampleObject{
				200: {
					"success": {Summary: "Success", Value: "ok"},
				},
			},
			respDescs: map[int]string{
				200: "Payment processed",
			},
		},
	}

	doc, err := gen.Generate(infos(routes...))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	op := doc.Paths["/pay"].Post
	// Request body examples
	if op.RequestBody == nil {
		t.Fatal("expected request body")
	}
	mt := op.RequestBody.Content["application/json"]
	if mt.Examples == nil {
		t.Fatal("expected request body examples")
	}
	if _, ok := mt.Examples["example1"]; !ok {
		t.Error("expected 'example1' in request body examples")
	}

	// Response examples and description
	resp, ok := op.Responses["200"]
	if !ok {
		t.Fatal("expected 200 response")
	}
	if resp.Description != "Payment processed" {
		t.Errorf("expected description 'Payment processed', got %q", resp.Description)
	}
	respMT := resp.Content["application/json"]
	if respMT.Examples == nil {
		t.Fatal("expected response examples")
	}
	if _, ok := respMT.Examples["success"]; !ok {
		t.Error("expected 'success' in response examples")
	}
}

// ---- Components cleanup ----

func TestGenerator_Generate_EmptyComponents(t *testing.T) {
	cfg := openapi.Config{Title: "T", Version: "1"}
	gen := openapi.NewGenerator(cfg, nil)

	// Route with no validation, no security - components should be nil
	routes := []rxroute.Route{
		&mockRoute{method: "GET", path: "/test", operationID: "test"},
	}

	doc, _ := gen.Generate(infos(routes...))

	if doc.Components != nil {
		t.Error("expected nil components when no schemas or security schemes")
	}
}

func TestGenerator_Generate_OnlySchemasNoSecurity(t *testing.T) {
	cfg := openapi.Config{Title: "T", Version: "1"}
	gen := openapi.NewGenerator(cfg, nil)

	routes := []rxroute.Route{
		&mockValidatedRoute{
			method: "POST", path: "/users", operationID: "createUser",
			reqBody: &testBodySchema{kind: openapi.SchemaScalar, types: []interface{}{ReqBody{}}},
		},
	}

	doc, _ := gen.Generate(infos(routes...))

	if doc.Components == nil {
		t.Fatal("expected components")
	}
	if doc.Components.Schemas == nil {
		t.Error("expected schemas")
	}
	if doc.Components.SecuritySchemes != nil {
		t.Error("expected nil security schemes")
	}
}

func TestGenerator_Generate_OnlySecurityNoSchemas(t *testing.T) {
	cfg := openapi.Config{Title: "T", Version: "1"}
	schemes := []openapi.SecuritySchemeAccessor{
		&mockSecurityScheme{name: "bearer", typ: "http", challenge: "Bearer"},
	}
	gen := openapi.NewGenerator(cfg, schemes)

	routes := []rxroute.Route{
		&mockRoute{method: "GET", path: "/test", operationID: "test"},
	}

	doc, _ := gen.Generate(infos(routes...))

	if doc.Components == nil {
		t.Fatal("expected components")
	}
	if doc.Components.SecuritySchemes == nil {
		t.Error("expected security schemes")
	}
	if doc.Components.Schemas != nil {
		t.Error("expected nil schemas")
	}
}

func TestGenerator_Generate_Description(t *testing.T) {
	cfg := openapi.Config{
		Title:       "T",
		Version:     "1",
		Description: "A test API description",
	}
	gen := openapi.NewGenerator(cfg, nil)

	doc, _ := gen.Generate(nil)

	if doc.Info.Description != "A test API description" {
		t.Errorf("expected description, got %q", doc.Info.Description)
	}
}

func TestGenerator_Generate_LowercaseMethod(t *testing.T) {
	cfg := openapi.Config{Title: "T", Version: "1"}
	gen := openapi.NewGenerator(cfg, nil)

	// The generator calls strings.ToUpper on the method, so lowercase input should work
	routes := []rxroute.Route{
		&mockRoute{method: "get", path: "/test", operationID: "test"},
	}

	doc, _ := gen.Generate(infos(routes...))

	pi := doc.Paths["/test"]
	if pi.Get == nil {
		t.Error("expected GET operation from lowercase method")
	}
}

func TestGenerator_Generate_MultipleSecuritySchemes(t *testing.T) {
	cfg := openapi.Config{Title: "T", Version: "1"}
	schemes := []openapi.SecuritySchemeAccessor{
		&mockSecurityScheme{name: "bearer", typ: "http", challenge: "Bearer"},
		&mockSecurityScheme{name: "basic", typ: "http", challenge: "Basic"},
	}
	gen := openapi.NewGenerator(cfg, schemes)

	routes := []rxroute.Route{
		&mockSecuredRoute{
			method: "GET", path: "/secure", operationID: "secureOp",
			schemes: []string{"bearer", "basic"},
		},
	}

	doc, _ := gen.Generate(infos(routes...))

	op := doc.Paths["/secure"].Get
	if len(op.Security) != 2 {
		t.Fatalf("expected 2 security requirements, got %d", len(op.Security))
	}
}

func TestGenerator_Generate_NoRoutes(t *testing.T) {
	cfg := openapi.Config{Title: "T", Version: "1"}
	gen := openapi.NewGenerator(cfg, nil)

	doc, err := gen.Generate(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(doc.Paths) != 0 {
		t.Errorf("expected empty paths, got %d", len(doc.Paths))
	}
}

func TestGenerator_Generate_WithTagsInConfig(t *testing.T) {
	cfg := openapi.Config{
		Title:   "Tagged API",
		Version: "1",
		Tags: []openapi.Tag{
			{Name: "users", Description: "User management"},
			{Name: "products", Description: "Product catalog"},
		},
	}
	gen := openapi.NewGenerator(cfg, nil)

	doc, err := gen.Generate(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(doc.Tags) != 2 {
		t.Fatalf("expected 2 tags in document, got %d", len(doc.Tags))
	}
	if doc.Tags[0].Name != "users" {
		t.Errorf("expected first tag %q, got %q", "users", doc.Tags[0].Name)
	}
	if doc.Tags[0].Description != "User management" {
		t.Errorf("expected first tag description %q, got %q", "User management", doc.Tags[0].Description)
	}
	if doc.Tags[1].Name != "products" {
		t.Errorf("expected second tag %q, got %q", "products", doc.Tags[1].Name)
	}
}

func TestGenerator_Generate_NoTags_DocumentTagsNil(t *testing.T) {
	cfg := openapi.Config{Title: "T", Version: "1"}
	gen := openapi.NewGenerator(cfg, nil)

	doc, err := gen.Generate(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(doc.Tags) != 0 {
		t.Errorf("expected no tags when none configured, got %d", len(doc.Tags))
	}
}

func TestGenerator_Generate_WithResponseEmptyBodySchema(t *testing.T) {
	cfg := openapi.Config{Title: "T", Version: "1"}
	gen := openapi.NewGenerator(cfg, nil)

	routes := []rxroute.Route{
		&mockValidatedRoute{
			method: "POST", path: "/items", operationID: "createItem",
			reqBody: &testBodySchema{kind: openapi.SchemaScalar, types: []interface{}{}}, // empty types
		},
	}

	doc, _ := gen.Generate(infos(routes...))

	op := doc.Paths["/items"].Post
	// Empty types should produce an object schema for request body.
	if op.RequestBody == nil {
		t.Fatal("expected request body")
	}
	mt := op.RequestBody.Content["application/json"]
	if mt.Schema == nil {
		t.Fatal("expected schema")
	}
	if mt.Schema.Type != "object" {
		t.Errorf("expected type 'object' for empty types, got %q", mt.Schema.Type)
	}
}

func TestGenerator_PathParams_SingleParam(t *testing.T) {
	// SingleParam ensures a `{id}` segment is recognised and a required
	// path parameter is emitted with schema type "string".
	cfg := openapi.Config{Title: "T", Version: "1"}
	gen := openapi.NewGenerator(cfg, nil)

	routes := []rxroute.Route{
		&mockRoute{method: "GET", path: "/users/{id}", operationID: "getUser"},
	}

	doc, err := gen.Generate(infos(routes...))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pi, ok := doc.Paths["/users/{id}"]
	if !ok {
		t.Fatal("expected /users/{id} path in document")
	}
	op := pi.Get
	if op == nil {
		t.Fatal("expected GET operation")
	}
	if len(op.Parameters) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(op.Parameters))
	}
	p := op.Parameters[0]
	if p.Name != "id" {
		t.Errorf("expected parameter name 'id', got %q", p.Name)
	}
	if p.In != "path" {
		t.Errorf("expected parameter in 'path', got %q", p.In)
	}
	if !p.Required {
		t.Error("path parameter should be required")
	}
	if p.Schema == nil || p.Schema.Type != "string" {
		t.Error("path parameter schema should be string")
	}
}

func TestGenerator_PathParams_MultipleParams(t *testing.T) {
	// MultipleParams ensures that all {param} segments in a path are captured.
	cfg := openapi.Config{Title: "T", Version: "1"}
	gen := openapi.NewGenerator(cfg, nil)

	routes := []rxroute.Route{
		&mockRoute{method: "GET", path: "/users/{userID}/posts/{postID}", operationID: "getPost"},
	}

	doc, err := gen.Generate(infos(routes...))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pi, ok := doc.Paths["/users/{userID}/posts/{postID}"]
	if !ok {
		t.Fatal("expected /users/{userID}/posts/{postID} path in document")
	}
	op := pi.Get
	if op == nil {
		t.Fatal("expected GET operation")
	}
	if len(op.Parameters) != 2 {
		t.Fatalf("expected 2 parameters, got %d", len(op.Parameters))
	}
	names := map[string]bool{}
	for _, p := range op.Parameters {
		names[p.Name] = true
	}
	if !names["userID"] || !names["postID"] {
		t.Errorf("expected parameters 'userID' and 'postID', got %v", names)
	}
}

func TestGenerator_PathParams_NoParams(t *testing.T) {
	// NoParams ensures a plain path without {params} produces no path parameters.
	cfg := openapi.Config{Title: "T", Version: "1"}
	gen := openapi.NewGenerator(cfg, nil)

	routes := []rxroute.Route{
		&mockRoute{method: "GET", path: "/health", operationID: "health"},
	}

	doc, err := gen.Generate(infos(routes...))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pi, ok := doc.Paths["/health"]
	if !ok {
		t.Fatal("expected /health path in document")
	}
	op := pi.Get
	if op == nil {
		t.Fatal("expected GET operation")
	}
	if len(op.Parameters) != 0 {
		t.Errorf("expected no parameters, got %d", len(op.Parameters))
	}
}

// ---------------------------------------------------------------------------
// Body schema composition
// ---------------------------------------------------------------------------

type schemaReqA struct {
	Alpha string `json:"alpha"`
}

type schemaReqB struct {
	Beta int `json:"beta"`
}

// Every composition strategy has to reach the document. This was four reflect
// calls ending in an unchecked assertion that panicked inside document
// generation if Types ever returned a differently-typed slice (D20).
func TestGenerator_body_schema_composition(t *testing.T) {
	cases := []struct {
		name   string
		schema *testBodySchema
		check  func(t *testing.T, s map[string]any)
	}{
		{
			name:   "scalar",
			schema: &testBodySchema{kind: openapi.SchemaScalar, types: []interface{}{schemaReqA{}}},
			check: func(t *testing.T, s map[string]any) {
				// A named struct is emitted once into components and
				// referenced, so the operation carries a $ref rather than an
				// inline copy.
				if ref, _ := s["$ref"].(string); ref != "#/components/schemas/schemaReqA" {
					t.Fatalf("scalar schema = %v, want a $ref to schemaReqA", s)
				}
			},
		},
		{
			name:   "oneOf",
			schema: &testBodySchema{kind: openapi.SchemaOneOf, types: []interface{}{schemaReqA{}, schemaReqB{}}},
			check:  expectUnion(t, "oneOf", 2),
		},
		{
			name:   "anyOf",
			schema: &testBodySchema{kind: openapi.SchemaAnyOf, types: []interface{}{schemaReqA{}, schemaReqB{}}},
			check:  expectUnion(t, "anyOf", 2),
		},
		{
			name:   "allOf",
			schema: &testBodySchema{kind: openapi.SchemaAllOf, types: []interface{}{schemaReqA{}, schemaReqB{}}},
			check:  expectUnion(t, "allOf", 2),
		},
		{
			// An unknown kind falls back to the first type rather than
			// producing nothing, so a schema kind added later still documents
			// something.
			name:   "unknown kind falls back to the first type",
			schema: &testBodySchema{kind: openapi.SchemaKind(99), types: []interface{}{schemaReqA{}}},
			check: func(t *testing.T, s map[string]any) {
				if ref, _ := s["$ref"].(string); ref != "#/components/schemas/schemaReqA" {
					t.Fatalf("the fallback produced %v, want the first type's schema", s)
				}
			},
		},
		{
			// No types at all is a plain object, not a nil schema — a nil
			// would drop the request body from the document entirely.
			name:   "no types",
			schema: &testBodySchema{kind: openapi.SchemaOneOf, types: nil},
			check: func(t *testing.T, s map[string]any) {
				if s["type"] != "object" {
					t.Fatalf("expected a bare object schema, got %v", s)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rt := &mockValidatedRoute{
				method: "POST", path: "/things", operationID: "createThing",
				reqBody: tc.schema,
			}
			doc := generateDoc(t, rt)
			schema := requestSchema(t, doc, "/things", "post")
			tc.check(t, schema)
		})
	}
}

// Response schemas take the same path, keyed by status code.
func TestGenerator_response_schema_composition(t *testing.T) {
	rt := &mockValidatedRoute{
		method: "GET", path: "/things", operationID: "listThings",
		responses: map[int]*testBodySchema{
			200: {kind: openapi.SchemaScalar, types: []interface{}{schemaReqA{}}},
			409: {kind: openapi.SchemaOneOf, types: []interface{}{schemaReqA{}, schemaReqB{}}},
		},
	}
	doc := generateDoc(t, rt)

	responses := dig(t, doc, "paths", "/things", "get", "responses")
	for _, status := range []string{"200", "409"} {
		if _, ok := responses[status]; !ok {
			t.Fatalf("status %s is missing from the document: %v", status, responses)
		}
	}
	oneOf := dig(t, responses, "409", "content", "application/json", "schema")
	if _, ok := oneOf["oneOf"]; !ok {
		t.Fatalf("the 409 response lost its oneOf composition: %v", oneOf)
	}
}

// A nil response schema for a status is skipped rather than documented as an
// empty body.
func TestGenerator_skips_a_nil_response_schema(t *testing.T) {
	rt := &mockValidatedRoute{
		method: "GET", path: "/things", operationID: "listThings",
		responses: map[int]*testBodySchema{200: nil},
	}
	doc := generateDoc(t, rt)
	responses := dig(t, doc, "paths", "/things", "get", "responses")
	if content, ok := responses["200"].(map[string]any)["content"]; ok {
		t.Fatalf("a nil schema produced content: %v", content)
	}
}

// --- helpers ---

func expectUnion(t *testing.T, key string, want int) func(*testing.T, map[string]any) {
	t.Helper()
	return func(t *testing.T, s map[string]any) {
		raw, ok := s[key]
		if !ok {
			t.Fatalf("schema has no %s: %v", key, s)
		}
		list, ok := raw.([]any)
		if !ok {
			t.Fatalf("%s is %T, want a list", key, raw)
		}
		if len(list) != want {
			t.Fatalf("%s has %d entries, want %d", key, len(list), want)
		}
	}
}

func generateDoc(t *testing.T, rt rxroute.Route) map[string]any {
	t.Helper()
	gen := openapi.NewGenerator(*openapi.NewDefaultConfig(), nil)
	generated, err := gen.Generate(infos(rt))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	raw, err := json.Marshal(generated)
	if err != nil {
		t.Fatalf("the document does not marshal: %v", err)
	}
	// Asserted on the wire form rather than the Go struct: that is what
	// tooling consumes, and omitempty decides what actually appears.
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("the document is not valid JSON: %v", err)
	}
	return doc
}

func requestSchema(t *testing.T, doc map[string]any, path, method string) map[string]any {
	t.Helper()
	return dig(t, doc, "paths", path, method, "requestBody", "content", "application/json", "schema")
}

func dig(t *testing.T, m map[string]any, keys ...string) map[string]any {
	t.Helper()
	cur := m
	for i, k := range keys {
		next, ok := cur[k]
		if !ok {
			t.Fatalf("key %q missing at depth %d; have %v", k, i, keysOf(cur))
		}
		cur, ok = next.(map[string]any)
		if !ok {
			t.Fatalf("key %q at depth %d is %T, want an object", k, i, next)
		}
	}
	return cur
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
