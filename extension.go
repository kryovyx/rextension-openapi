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
	"sync"

	"github.com/kryovyx/rex/event"
	"github.com/kryovyx/rex/route"
	rx "github.com/kryovyx/rextension"
)

// OpenAPIExtension implements the Rex extension contract for OpenAPI generation.
type OpenAPIExtension struct {
	cfg    Config
	logger rx.Logger
	rex    rx.Rex // Store reference to Rex for lazy generation

	mu     sync.Mutex
	routes []route.Route

	// securitySchemes are discovered from DI at generation time.
	securitySchemes []SecuritySchemeAccessor

	// Lazy generation tracking
	generatedOnce bool
	docBytes      []byte
}

// NewOpenAPIExtension constructs an OpenAPI extension instance.
func NewOpenAPIExtension(cfg *Config) rx.Extension {
	c := NewDefaultConfig()
	if cfg != nil {
		if cfg.Title != "" {
			c.Title = cfg.Title
		}
		if cfg.Version != "" {
			c.Version = cfg.Version
		}
		if cfg.Description != "" {
			c.Description = cfg.Description
		}
		if cfg.ServePath != "" {
			c.ServePath = cfg.ServePath
		}
		if len(cfg.Tags) > 0 {
			c.Tags = cfg.Tags
		}
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

	// Subscribe to route registration events.
	r.EventBus().Subscribe(event.RouterRouteRegisteredEventType, func(ev event.Event) {
		if routeEv, ok := event.As[event.RouterRouteRegisteredEvent](ev); ok {
			e.logger.Debug("OpenAPI: Route registered: %s %s", routeEv.Route.Method(), routeEv.Route.Path())
			// Only collect routes that implement OpenAPIRoute.
			if oar, isOA := routeEv.Route.(OpenAPIRoute); isOA {
				e.mu.Lock()
				e.routes = append(e.routes, routeEv.Route)
				e.mu.Unlock()
				e.logger.Info("OpenAPI: Collected route %s %s (opid=%s)",
					routeEv.Route.Method(), routeEv.Route.Path(), oar.OperationID())
			} else {
				e.logger.Debug("OpenAPI: Route %s %s does not implement OpenAPIRoute",
					routeEv.Route.Method(), routeEv.Route.Path())
			}
		}
	})

	e.logger.Info("OpenAPI extension initialized, serving at %s", e.cfg.ServePath)
	return nil
}

// OnStart registers the OpenAPI spec route. Actual generation happens in OnReady
// once all routes are registered, but we need the route placeholder now.
func (e *OpenAPIExtension) OnStart(ctx context.Context, r rx.Rex) error {
	return nil
}

// OnReady registers the spec route handler (document generation is deferred until first request).
// By this point all routes have been registered.
func (e *OpenAPIExtension) OnReady(ctx context.Context, r rx.Rex) error {
	e.rex = r // Store Rex reference for lazy generation

	// Register the spec route with lazy generation (document will be generated on first request).
	specRoute := newSpecRouteWithGenerator(e.cfg.ServePath, e)
	if err := r.RegisterRoute(specRoute); err != nil {
		e.logger.Error("Failed to register OpenAPI spec route: %v", err)
		return err
	}

	e.logger.Info("OpenAPI 3.1 extension ready, spec will be served at %s (generated on first request)",
		e.cfg.ServePath)

	return nil
}

// ensureGenerated generates the OpenAPI document on first access (lazy generation).
// This ensures all routes and extensions have had time to process.
func (e *OpenAPIExtension) ensureGenerated() ([]byte, error) {
	e.mu.Lock()
	if e.generatedOnce {
		defer e.mu.Unlock()
		return e.docBytes, nil
	}
	e.mu.Unlock()

	routes := func() []route.Route {
		e.mu.Lock()
		defer e.mu.Unlock()
		dst := make([]route.Route, len(e.routes))
		copy(dst, e.routes)
		return dst
	}()

	if e.logger != nil {
		e.logger.Info("OpenAPI: Lazily generating document with %d collected routes", len(routes))
		for i, rt := range routes {
			e.logger.Debug("OpenAPI: Processing route %d: %s %s", i, rt.Method(), rt.Path())
		}
	}

	// Try to discover security schemes from DI (soft dependency).
	// The security extension exposes its scheme registry; we try to extract schemes.
	e.discoverSecuritySchemes(e.rex)

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

	e.mu.Lock()
	e.generatedOnce = true
	e.docBytes = docBytes
	e.mu.Unlock()

	e.logger.Info("OpenAPI 3.1 document generated with %d operations", len(doc.Paths))

	return docBytes, nil
}

// discoverSecuritySchemes reads security schemes from the shared rextension
// registry. The security extension (or any other extension) publishes there
// via rextension.RegisterSecuritySchemes without importing this package.
func (e *OpenAPIExtension) discoverSecuritySchemes(_ rx.Rex) {
	schemes := rx.GetSecuritySchemes()
	if len(schemes) > 0 {
		e.securitySchemes = schemes
		if e.logger != nil {
			e.logger.Info("OpenAPI: Using %d security schemes from rextension registry", len(schemes))
		}
		return
	}
	if e.logger != nil {
		e.logger.Debug("OpenAPI: No security schemes registered")
	}
}

// OnStop is a no-op for the OpenAPI extension.
func (e *OpenAPIExtension) OnStop(ctx context.Context, r rx.Rex) error { return nil }

// OnShutdown is a no-op for the OpenAPI extension.
func (e *OpenAPIExtension) OnShutdown(ctx context.Context, r rx.Rex) error { return nil }
