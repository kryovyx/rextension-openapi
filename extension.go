// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: © 2026 Kryovyx

// Package openapi provides a Rex extension that generates an OpenAPI 3.1
// specification document from routes implementing the OpenAPIRoute interface.
//
// The extension provides:
//   - Automatic OpenAPI 3.1 document generation from registered routes
//   - Rich schemas via reflection on request/response body types
//   - OneOf/AnyOf/AllOf union type support
//   - Per-status-code response documentation
//   - Security scheme documentation (soft-dep on rextension-security)
//   - Served at a configurable route (default: /openapi.json)
//   - Document generated once at startup and held in memory
package openapi

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	rx "github.com/kryovyx/rextension"
	rxevent "github.com/kryovyx/rextension/event"
)

// OpenAPIExtension implements the Rex extension contract for OpenAPI generation.
type OpenAPIExtension struct {
	cfg    Config
	logger rx.Logger

	mu     sync.Mutex
	routes []rxevent.Route

	// securitySchemes are resolved once, when the document is generated.
	//
	// Guarded by mu, and written exactly once. It used to be written from
	// whichever request happened to generate the document first, with no
	// synchronisation at all (D40).
	securitySchemes []SecuritySchemeAccessor

	// spec is the route serving the document. The document itself is
	// published by atomic swap; see specRoute.
	spec *specRoute

	// container is captured in OnInitialize so the security scheme registry
	// can be resolved at generation time.
	container rx.Container
}

// NewOpenAPIExtension constructs a openapi extension instance.
//
// A nil cfg takes the defaults. A non-nil cfg is used **verbatim** (D22/O6).
//
// It used to merge field by field — "copy this one if it is non-zero" — which
// looked like a convenience and behaved as a trap:
//
//   - Fields the merge simply forgot were silently ignored. Any value an
//     application set for them was discarded, and there was nothing to see:
//     no error, no log line, just the default.
//   - Fields copied *unconditionally* had the opposite problem. A partial
//     struct literal like &Config{LivePath: "/x"} zeroed them, so setting one
//     unrelated field turned another off.
//   - Zero and "unset" were indistinguishable, so a deliberate zero could not
//     be expressed at all.
//
// Build the config with NewConfig and the With* options; that is the shape
// that cannot drift out of step with the struct:
//
//	openapi.NewConfig(openapi.WithSomething(...))
//
// ⚠ A partial struct literal now leaves the rest of the fields at their zero
// values rather than at their defaults. That is the breaking half of this
// change, and it is deliberate: silently substituting a default for a field
// the caller wrote is what produced the traps above.
func NewOpenAPIExtension(cfg *Config) rx.Extension {
	c := NewDefaultConfig()
	if cfg != nil {
		c = cfg
	}
	return &OpenAPIExtension{cfg: *c}
}

// WithOpenAPI is a helper Option to attach the OpenAPI extension to Rex.
func WithOpenAPI(cfg *Config) rx.Option {
	return rx.WithExtension(NewOpenAPIExtension(cfg))
}

// SetSecuritySchemes is called by other extensions (like the security extension)
// to provide security scheme definitions for OpenAPI documentation.
// OnInitialize subscribes to route registration events to collect routes.
func (e *OpenAPIExtension) OnInitialize(ctx context.Context, r rx.Rex) error {
	e.logger = r.Logger()
	e.container = r.Container()

	// Routes are NOT collected from the route-registered event.
	//
	// They were, and it raced: that event is delivered asynchronously — the
	// bus dispatches on worker goroutines — while document generation happens
	// synchronously when the route table is built. The generated document
	// could therefore be missing any number of routes, depending purely on
	// scheduling. It was not detectable in a small application, because a
	// handful of events are usually drained before the build finishes.
	//
	// The framework hands the complete route table to ValidateRoutes instead,
	// with the router each route belongs to. See ValidateRoutes.

	e.logger.Info("OpenAPI extension initialized, serving at %s", e.cfg.ServePath)
	return nil
}

// OnStart registers the OpenAPI spec route.
//
// It used to register from OnReady, on the reasoning that "by this point all
// routes have been registered". That was true, and it was also too late: by
// OnReady the listeners are bound and serving, so the registration mutated a
// route trie that in-flight requests were already reading (D12).
//
// OnStart is both early enough — the route table is built after every
// extension's OnStart has run — and late enough, because route collection
// happens through the route-registered event rather than by scanning at this
// moment. Routes declared by extensions whose OnStart runs after this one are
// still collected, since the subscription set up in OnInitialize is what
// gathers them.
func (e *OpenAPIExtension) OnStart(ctx context.Context, r rx.Rex) error {
	e.spec = newSpecRoute(e.cfg.ServePath)

	serveOn := e.cfg.ServeOnRouter
	if serveOn == "" {
		serveOn = rx.DefaultRouterName
	}
	if err := r.RegisterRouteToRouter(e.spec, serveOn); err != nil {
		e.logger.Error("Failed to register the OpenAPI spec route on router %s: %v", serveOn, err)
		return err
	}

	e.logger.Info("OpenAPI 3.1 extension will serve its spec at %s on router %s",
		e.cfg.ServePath, serveOn)
	return nil
}

// includesRouter reports whether routes from routerName belong in the
// document.
//
// The default — an empty IncludeRouters — is the default router only. That is
// deliberately narrow: a document that omits a route can be widened by
// configuration, whereas a document that published a private route has already
// published it (D41).
func (e *OpenAPIExtension) includesRouter(routerName string) bool {
	if routerName == "" {
		routerName = rx.DefaultRouterName
	}

	for _, excluded := range e.cfg.ExcludeRouters {
		if excluded == routerName {
			return false
		}
	}

	if len(e.cfg.IncludeRouters) == 0 {
		return routerName == rx.DefaultRouterName
	}
	for _, included := range e.cfg.IncludeRouters {
		if included == "*" || included == routerName {
			return true
		}
	}
	return false
}

// ValidateRoutes implements rextension.RouteValidator, and is where the
// document is generated (D40/P4.14).
//
// The framework calls it once, when the route table is built: every route is
// known, every extension has configured itself, and nothing is serving yet.
// That is the moment laziness was standing in for.
//
// Two consequences worth naming:
//
//   - A generation failure is a **startup** error. It used to be a 500 on
//     every request to the spec path, discoverable only by making one.
//   - The document, and the security schemes it embeds, are written once from
//     one goroutine. The lazy version wrote both from whichever request
//     arrived first, unsynchronised.
func (e *OpenAPIExtension) ValidateRoutes(routes []rx.RouteInfo) error {
	if e.spec == nil {
		// OnStart did not run, so there is nothing to serve. Not an error:
		// the extension may simply be disabled.
		return nil
	}

	e.collect(routes)

	doc, err := e.generate()
	if err != nil {
		return fmt.Errorf("failed to generate the OpenAPI document: %w", err)
	}
	e.spec.setDocument(doc)
	return nil
}

// collect selects the routes that belong in the document.
//
// Two filters, in order:
//
//  1. The router. Only routers the configuration includes — by default, the
//     default router alone (D41). Every router used to be collected, so a
//     route on an internal-only listener was published in the document served
//     on the public one.
//  2. The interface. Only routes implementing OpenAPIRoute; anything else has
//     nothing to document.
func (e *OpenAPIExtension) collect(routes []rx.RouteInfo) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.routes = e.routes[:0]
	for _, info := range routes {
		if !e.includesRouter(info.Router) {
			e.logger.Debug("OpenAPI: Skipping route %s %s on router %s, which is not included",
				info.Route.Method(), info.Route.Path(), info.Router)
			continue
		}

		oar, isOA := info.Route.(OpenAPIRoute)
		if !isOA {
			e.logger.Debug("OpenAPI: Route %s %s does not implement OpenAPIRoute",
				info.Route.Method(), info.Route.Path())
			continue
		}

		e.routes = append(e.routes, info.Route)
		e.logger.Debug("OpenAPI: Collected route %s %s from router %s (opid=%s)",
			info.Route.Method(), info.Route.Path(), info.Router, oar.OperationID())
	}
	e.logger.Info("OpenAPI: Collected %d route(s) for the document", len(e.routes))
}

var _ rx.RouteValidator = (*OpenAPIExtension)(nil)

// OnReady is a no-op; the spec route is registered in OnStart.
func (e *OpenAPIExtension) OnReady(ctx context.Context, r rx.Rex) error { return nil }

// generate builds the OpenAPI document from the collected routes.
//
// Called once, from ValidateRoutes, under the extension's own lock. The
// check-then-build-then-store dance the lazy version needed is gone: there is
// no cache to check and no second caller to race.
func (e *OpenAPIExtension) generate() ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	routes := make([]rxevent.Route, len(e.routes))
	copy(routes, e.routes)

	if e.logger != nil {
		e.logger.Info("OpenAPI: Generating the document from %d collected route(s)", len(routes))
		for i, rt := range routes {
			e.logger.Debug("OpenAPI: Processing route %d: %s %s", i, rt.Method(), rt.Path())
		}
	}

	// Resolve security schemes from the container (soft dependency). Written
	// under the same lock as everything else here, once.
	e.discoverSecuritySchemesLocked()

	// Generate the document.
	gen := NewGenerator(e.cfg, e.securitySchemes)
	doc, err := gen.Generate(routes)
	if err != nil {
		e.logger.Error("Failed to generate OpenAPI document: %v", err)
		return nil, err
	}

	docBytes, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		e.logger.Error("Failed to marshal OpenAPI document: %v", err)
		return nil, err
	}

	// Log generated paths
	e.logger.Info("OpenAPI: Generated %d paths", len(doc.Paths))
	for path, pi := range doc.Paths {
		methods := []string{}
		if pi.Get != nil {
			methods = append(methods, "GET")
		}
		if pi.Post != nil {
			methods = append(methods, "POST")
		}
		if pi.Put != nil {
			methods = append(methods, "PUT")
		}
		if pi.Patch != nil {
			methods = append(methods, "PATCH")
		}
		if pi.Delete != nil {
			methods = append(methods, "DELETE")
		}
		if pi.Head != nil {
			methods = append(methods, "HEAD")
		}
		if pi.Options != nil {
			methods = append(methods, "OPTIONS")
		}
		e.logger.Debug("OpenAPI: Path %s methods: %v", path, methods)
	}

	e.logger.Info("OpenAPI 3.1 document generated with %d operation(s)", len(doc.Paths))
	return docBytes, nil
}

// discoverSecuritySchemes resolves the security scheme registry from the DI
// container.
//
// This used to read a package-level slice in rextension, published by
// RegisterSecuritySchemes. The registry is an instance now (D21), resolved
// through the rextension.SchemeRegistry interface — so neither extension
// imports the other, and the schemes belong to *this* application rather than
// to the process.
//
// A soft dependency: an application with no security extension resolves
// nothing and documents no schemes, which is not an error.
func (e *OpenAPIExtension) discoverSecuritySchemesLocked() {
	if e.container == nil {
		return
	}

	var registry rx.SchemeRegistry
	if err := e.container.Resolve(&registry); err != nil || registry == nil {
		if e.logger != nil {
			e.logger.Debug("OpenAPI: No security scheme registry available: %v", err)
		}
		return
	}

	schemes := registry.Schemes()
	if len(schemes) == 0 {
		if e.logger != nil {
			e.logger.Debug("OpenAPI: The security scheme registry is empty")
		}
		return
	}

	e.securitySchemes = schemes
	if e.logger != nil {
		e.logger.Info("OpenAPI: Using %d security scheme(s) from the container registry", len(schemes))
	}
}

// OnStop is a no-op for the OpenAPI extension.
func (e *OpenAPIExtension) OnStop(ctx context.Context, r rx.Rex) error { return nil }

// OnShutdown is a no-op for the OpenAPI extension.
func (e *OpenAPIExtension) OnShutdown(ctx context.Context, r rx.Rex) error { return nil }
