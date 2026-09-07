// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: © 2026 Kryovyx

// Package openapi provides a Rex extension that generates an OpenAPI 3.1
// specification document from routes implementing the OpenAPIRoute interface.
//
// This file implements the document generator that assembles the full OpenAPI 3.1
// specification from registered routes, optionally enriching it with validation
// schemas and security requirements via duck-typed interface assertions.
package openapi

import (
	"fmt"
	"strings"

	rx "github.com/kryovyx/rextension"
	rxevent "github.com/kryovyx/rextension/event"
)

// wildcardParamDescription explains the one place a Rex path has no exact
// OpenAPI equivalent.
const wildcardParamDescription = "Matches the remainder of the path, including any \"/\" separators. " +
	"OpenAPI has no notation for a multi-segment path parameter, so tooling may " +
	"percent-encode \"/\" as %2F when constructing requests from this document; " +
	"the server accepts the unencoded form."

// joinBasePath prefixes a router's base path onto a route's own path (D48).
//
// A route registered on a router configured with BaseURL "/internal" is served
// at "/internal/schemas", not at "/schemas" — the router strips the prefix
// before matching, so the route declares the bare path. The document has to
// put it back, or every path in it is wrong by that prefix and every generated
// client calls a URL that 404s.
//
// The framework hands over an already normalised base — no trailing slash, ""
// for the root (rex's normalizeBaseURL) — but this does not rely on that: a
// generator called directly may be handed anything. It guarantees a leading
// slash and exactly one separator at the seam.
func joinBasePath(base, path string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base != "" && !strings.HasPrefix(base, "/") {
		base = "/" + base
	}

	path = strings.TrimSpace(path)
	if path == "" || path == "/" {
		if base == "" {
			return "/"
		}
		return base
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

// extractPathParams parses a Rex route path and returns the OpenAPI path plus a
// Parameter for every variable segment.
//
// Two segment kinds:
//
//   - "{name}" is already OpenAPI notation and passes through as a required
//     string path parameter.
//   - "*" is a Rex wildcard matching the remainder of the path, separators
//     included. OpenAPI has no notation for that, so it is emitted as
//     "{path}" with a description saying what it really matches (D42/O15).
//
// The wildcard case is a deliberate approximation rather than an omission.
// Silently dropping a route that exists is worse: a document that does not
// mention /files/* leaves a client with no way to discover the endpoint at
// all, whereas an approximate parameter with an honest description leaves it
// with a workable one. "{path}" is the closest legal representation.
func extractPathParams(path string) (openAPIPath string, params []*Parameter) {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		switch {
		case strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}"):
			params = append(params, &Parameter{
				Name:     p[1 : len(p)-1],
				In:       "path",
				Required: true,
				Schema:   &SchemaObject{Type: "string"},
			})

		case p == "*":
			// Rendered as a named parameter, because a bare "*" is not valid
			// in an OpenAPI path template.
			parts[i] = "{path}"
			params = append(params, &Parameter{
				Name:        "path",
				In:          "path",
				Required:    true,
				Description: wildcardParamDescription,
				Schema:      &SchemaObject{Type: "string"},
			})
		}
	}
	return strings.Join(parts, "/"), params
}

// Generator assembles an OpenAPI 3.1 Document from a set of routes.
type Generator struct {
	config          Config
	schemaGen       *SchemaGenerator
	securitySchemes []SecuritySchemeAccessor
}

// NewGenerator creates a new OpenAPI document generator.
func NewGenerator(cfg Config, securitySchemes []SecuritySchemeAccessor) *Generator {
	return &Generator{
		config:          cfg,
		schemaGen:       NewSchemaGenerator(),
		securitySchemes: securitySchemes,
	}
}

// Generate builds the full OpenAPI 3.1 document from the given routes.
// Only routes implementing OpenAPIRoute are included.
//
// It takes RouteInfo rather than bare routes because a route's documented path
// is not its own path: the router it sits on contributes a base prefix, and
// only RouteInfo carries it (D48). The signature changed for that reason — a
// caller passing []event.Route had no way to supply the prefix, so the
// omission was not a bug that could be fixed in place.
//
// Two routes resolving to the same method and documented path are a **startup
// error** rather than a silent overwrite. setOperation takes the last writer,
// so the losing operation used to vanish from the document with no diagnostic
// anywhere — and prefixing makes the collision expressible across routers.
func (g *Generator) Generate(routes []rx.RouteInfo) (*Document, error) {
	doc := &Document{
		OpenAPI: "3.1.0",
		Info: Info{
			Title:       g.config.Title,
			Version:     g.config.Version,
			Description: g.config.Description,
		},
		Tags:  g.config.Tags,
		Paths: make(map[string]*PathItem),
		Components: &Components{
			Schemas:         make(map[string]*SchemaObject),
			SecuritySchemes: make(map[string]*SecuritySchemeObject),
		},
	}

	// Register security schemes in components.
	for _, ss := range g.securitySchemes {
		sso := &SecuritySchemeObject{
			Type:        ss.Type(),
			Description: ss.Description(),
		}
		// Determine scheme-specific fields.
		switch ss.Type() {
		case "http":
			challenge := strings.ToLower(ss.Challenge())
			if strings.HasPrefix(challenge, "bearer") {
				sso.Scheme = "bearer"
				// Try to extract bearer format if scheme implements it.
				if bearer, ok := ss.(interface{ BearerFormat() string }); ok {
					sso.BearerFormat = bearer.BearerFormat()
				}
			} else if strings.HasPrefix(challenge, "basic") {
				sso.Scheme = "basic"
			}
		case "apiKey":
			// One interface assertion for both fields. This used to read
			// ParamName through an inline interface and Location through
			// reflect.MethodByName, because Location returned a named string
			// type that no interface could declare. It returns a plain string
			// now, so both come from ParameterizedScheme (D20).
			if ps, ok := ss.(ParameterizedScheme); ok {
				sso.Name = ps.ParamName()
				sso.In = ps.Location()
			}
		}
		if rcp, ok := ss.(RoleClaimProvider); ok {
			if claim := rcp.RolesClaim(); claim != "" {
				sso.XRolesClaim = claim
			}
		}
		doc.Components.SecuritySchemes[ss.Name()] = sso
	}

	// claimed remembers which route took each method+path, so a collision can
	// name both sides instead of silently dropping one of them.
	type origin struct{ router, operationID string }
	claimed := make(map[string]origin, len(routes))

	// Process each route.
	for _, info := range routes {
		rt := info.Route
		if rt == nil {
			continue
		}
		oar, ok := rt.(OpenAPIRoute)
		if !ok {
			continue // Not an OpenAPI route — skip.
		}

		op := &Operation{
			OperationID: oar.OperationID(),
			Summary:     oar.Summary(),
			Description: oar.Description(),
			Tags:        oar.Tags(),
			Responses:   make(map[string]*Response),
		}

		// Enrich with request/response schemas from validation.
		g.enrichWithValidation(rt, op)

		// Enrich with security requirements.
		g.enrichWithSecurity(rt, op)

		// Prefix the router's base path, then rewrite to OpenAPI {param}
		// syntax and collect the path parameters.
		openAPIPath, pathParams := extractPathParams(joinBasePath(info.BaseURL, rt.Path()))
		if len(pathParams) > 0 {
			op.Parameters = append(op.Parameters, pathParams...)
		}

		// Ensure at least a default response.
		if len(op.Responses) == 0 {
			op.Responses["200"] = &Response{Description: "Successful response"}
		}

		// Document the error format (P3.6/P4.16).
		//
		// Every error the framework and its extensions produce is an RFC 9457
		// problem document served as application/problem+json. A client
		// generated from a document that does not say so will try to parse
		// errors as whatever the success media type was, and fail — so the
		// spec has to carry it.
		g.addProblemResponses(op)

		// Add operation to path item.
		method := strings.ToUpper(rt.Method())
		key := method + " " + openAPIPath
		if prev, taken := claimed[key]; taken {
			return nil, fmt.Errorf(
				"duplicate operation %s: router %q operation %q and router %q operation %q resolve to the same documented path",
				key, prev.router, prev.operationID,
				routerNameOf(info.Router), oar.OperationID())
		}
		claimed[key] = origin{router: routerNameOf(info.Router), operationID: oar.OperationID()}

		if doc.Paths[openAPIPath] == nil {
			doc.Paths[openAPIPath] = &PathItem{}
		}
		doc.Paths[openAPIPath].setOperation(method, op)
	}

	// Copy component schemas from the generator.
	for name, schema := range g.schemaGen.Components() {
		if schema != nil {
			doc.Components.Schemas[name] = schema
		}
	}

	// Clean up empty components.
	if len(doc.Components.Schemas) == 0 && len(doc.Components.SecuritySchemes) == 0 {
		doc.Components = nil
	} else {
		if len(doc.Components.Schemas) == 0 {
			doc.Components.Schemas = nil
		}
		if len(doc.Components.SecuritySchemes) == 0 {
			doc.Components.SecuritySchemes = nil
		}
	}

	return doc, nil
}

// enrichWithValidation reads the route's declared request and response body
// schemas through rextension.BodySchemaProvider.
//
// This was reflection — reflect.MethodByName("RequestBody") and
// ("Responses"), plus MapKeys/MapIndex to walk the returned map — because the
// openapi and validation extensions had no shared type. They do now, so the
// whole thing is a type assertion (D20). A route that declares no schemas
// simply does not implement the interface.
func (g *Generator) enrichWithValidation(rt rxevent.Route, op *Operation) {
	provider, ok := rt.(ValidatableRouteAccessor)
	if !ok {
		return
	}

	if body := provider.RequestBody(); body != nil {
		if schema := g.extractBodySchema(body); schema != nil {
			mt := MediaType{Schema: schema}
			// Apply named request body examples if the route provides them.
			if ep, ok := rt.(RequestBodyExamplesProvider); ok {
				if namedExamples := ep.RequestBodyExamples(); len(namedExamples) > 0 {
					mt.Examples = make(map[string]*ExampleObject, len(namedExamples))
					for name, ex := range namedExamples {
						ex := ex // capture
						mt.Examples[name] = &ex
					}
				}
			}
			op.RequestBody = &RequestBody{
				Required: true,
				Content:  map[string]MediaType{"application/json": mt},
			}
		}
	}

	// Collect per-status named examples if the route provides them.
	var namedRespExamples map[int]map[string]ExampleObject
	if ep, ok := rt.(ResponseExamplesProvider); ok {
		namedRespExamples = ep.ResponseExamples()
	}
	// Collect per-status descriptions if the route provides them.
	var respDescriptions map[int]string
	if dp, ok := rt.(ResponseDescriptionProvider); ok {
		respDescriptions = dp.ResponseDescriptions()
	}

	// Response schemas, keyed by status code. A plain map range, where this
	// was reflect MapKeys/MapIndex.
	for statusCode, body := range provider.Responses() {
		if body == nil {
			continue
		}
		schema := g.extractBodySchema(body)
		if schema == nil {
			continue
		}

		mt := MediaType{Schema: schema}
		// Apply named examples for this status code if available.
		if examples, ok := namedRespExamples[statusCode]; ok && len(examples) > 0 {
			mt.Examples = make(map[string]*ExampleObject, len(examples))
			for name, ex := range examples {
				ex := ex // capture
				mt.Examples[name] = &ex
			}
		}
		// Resolve response description.
		desc := fmt.Sprintf("Response for status %d", statusCode)
		if d, ok := respDescriptions[statusCode]; ok && d != "" {
			desc = d
		}
		op.Responses[fmt.Sprintf("%d", statusCode)] = &Response{
			Description: desc,
			Content:     map[string]MediaType{"application/json": mt},
		}
	}
}

// extractBodySchema turns a declared body schema into an OpenAPI schema
// object.
//
// It is one line now. It used to be a full copy of the Scalar/OneOf/AnyOf/
// AllOf switch that corex/schema's GenerateBodySchema already implements —
// two implementations of one wire format inside one module, with the shared
// one reached only by its own tests. That is exactly the drift W23 exists to
// prevent, so the copy went and this delegates.
//
// Before that it was four reflect calls — MethodByName("Kind"),
// MethodByName("Types"), two Call(nil)s — ending in
// `typesResults[0].Interface().([]interface{})`, an unchecked assertion that
// panicked inside document generation if Types ever returned a
// differently-typed slice. The interface makes both the lookup and the
// assertion unnecessary (D20).
func (g *Generator) extractBodySchema(bs BodySchemaAccessor) *SchemaObject {
	return g.schemaGen.GenerateBodySchema(bs)
}

// enrichWithSecurity checks if the route also implements SecuredRouteAccessor
// and adds security requirements to the operation.
// If the route also implements ScopedSecuredRouteAccessor the declared scopes
// are included in each security requirement entry instead of an empty slice.
// If the route implements RoleGuardedRouteAccessor the declared roles are
// emitted as x-required-roles on the operation (keyed by scheme name).
func (g *Generator) enrichWithSecurity(rt rxevent.Route, op *Operation) {
	sr, ok := rt.(SecuredRouteAccessor)
	if !ok {
		return
	}

	schemes := sr.RequiredSchemes()
	if len(schemes) == 0 {
		return
	}

	var scopeMap map[string][]string
	if ss, ok := rt.(ScopedSecuredRouteAccessor); ok {
		scopeMap = ss.RequiredScopes()
	}

	for _, schemeName := range schemes {
		scopes := []string{}
		if scopeMap != nil {
			if s, ok := scopeMap[schemeName]; ok {
				scopes = s
			}
		}
		op.Security = append(op.Security, SecurityRequirement{schemeName: scopes})
	}

	if rg, ok := rt.(RoleGuardedRouteAccessor); ok {
		if roleMap := rg.RequiredRoles(); len(roleMap) > 0 {
			op.XRequiredRoles = roleMap
		}
	}
}

// problemSchema is the RFC 9457 problem details schema, referenced by every
// documented error response.
//
// Declared inline rather than as a component reference, because the generator
// has no component-schema registry and a $ref to something absent is worse
// than a repeated inline schema.
func problemSchema() *SchemaObject {
	return &SchemaObject{
		Type: "object",
		Properties: map[string]*SchemaObject{
			"type": {
				Type:        "string",
				Description: "A URI identifying the problem type.",
			},
			"title": {
				Type:        "string",
				Description: "A short, human-readable summary of the problem type. Stable across occurrences.",
			},
			"status": {
				Type:        "integer",
				Description: "The HTTP status code.",
			},
			"detail": {
				Type:        "string",
				Description: "A human-readable explanation specific to this occurrence.",
			},
			"instance": {
				Type:        "string",
				Description: "A URI identifying this specific occurrence, carrying the request identifier.",
			},
		},
		Required: []string{"type", "title", "status"},
	}
}

// problemResponse builds a documented error response.
func problemResponse(description string) *Response {
	return &Response{
		Description: description,
		Content: map[string]MediaType{
			"application/problem+json": {Schema: problemSchema()},
		},
	}
}

// addProblemResponses documents the error statuses an operation can produce
// (P3.6/P4.16).
//
// Two categories, with different precedence, and the distinction matters:
//
//   - **Framework-owned** statuses are answered by middleware. The route's
//     handler never runs, so whatever the route declared for that status is
//     simply not what a client receives. These **override** the route's own
//     declaration.
//   - **Handler-owned** statuses can come from the handler itself, so a
//     route's own declaration is authoritative and these are only added when
//     absent.
//
// Getting the precedence backwards produces a document that is confidently
// wrong: the sandbox's routes declared a 422 with their own error DTO, while
// the validation middleware answers 422 with an RFC 9457 problem document. A
// generated client would have parsed the wrong shape, and the document would
// have looked like the authority on it.
func (g *Generator) addProblemResponses(op *Operation) {
	// Answered by middleware; the handler is never reached.
	frameworkOwned := map[string]string{}

	// Only when the operation is actually secured does 401/403 apply.
	if len(op.Security) > 0 {
		frameworkOwned["401"] = "Authentication is required and was not provided, or was not accepted."
		frameworkOwned["403"] = "The authenticated principal is not permitted to perform this operation."
	}
	// Only when there is a request body can its media type or shape be wrong.
	if op.RequestBody != nil {
		frameworkOwned["415"] = "The request body's Content-Type is not supported."
		frameworkOwned["422"] = "The request body failed validation."
	}

	for status, description := range frameworkOwned {
		op.Responses[status] = problemResponse(description)
	}

	// May come from the handler, so only documented when the route did not.
	handlerOwned := map[string]string{
		"400": "The request is malformed.",
		"500": "An unexpected server-side failure.",
	}
	for status, description := range handlerOwned {
		if _, declared := op.Responses[status]; declared {
			continue
		}
		op.Responses[status] = problemResponse(description)
	}
}
