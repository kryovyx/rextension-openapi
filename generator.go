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
	"reflect"
	"strings"

	"github.com/kryovyx/rex/route"
)

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
func (g *Generator) Generate(routes []route.Route) (*Document, error) {
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
			// Extract name and in from the scheme if it exposes them.
			if ak, ok := ss.(interface {
				ParamName() string
				Location() string
			}); ok {
				sso.Name = ak.ParamName()
				sso.In = ak.Location()
			}
		}
		doc.Components.SecuritySchemes[ss.Name()] = sso
	}

	// Process each route.
	for _, rt := range routes {
		oar, ok := rt.(OpenAPIRoute)
		if !ok {
			continue // Not an OpenAPI route — skip.
		}
		// Debug: log which routes are being processed
		_ = oar // Use variable to avoid unused error

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

		// Ensure at least a default response.
		if len(op.Responses) == 0 {
			op.Responses["200"] = &Response{Description: "Successful response"}
		}

		// Add operation to path item.
		path := rt.Path()
		if doc.Paths[path] == nil {
			doc.Paths[path] = &PathItem{}
		}
		doc.Paths[path].setOperation(strings.ToUpper(rt.Method()), op)
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

// enrichWithValidation uses reflection to call RequestBody() and Responses()
// on the route. This works with any type implementing those methods (e.g.,
// validation.ValidatableRoute) without importing the validation package.
func (g *Generator) enrichWithValidation(rt route.Route, op *Operation) {
	rv := reflect.ValueOf(rt)

	// Try RequestBody() → returns some BodySchema-like interface.
	if reqMethod := rv.MethodByName("RequestBody"); reqMethod.IsValid() {
		results := reqMethod.Call(nil)
		if len(results) == 1 && !results[0].IsNil() {
			schema := g.extractBodySchema(results[0].Interface())
			if schema != nil {
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

	// Try Responses() → returns map[int]BodySchema-like.
	if respMethod := rv.MethodByName("Responses"); respMethod.IsValid() {
		results := respMethod.Call(nil)
		if len(results) == 1 && !results[0].IsNil() {
			// Iterate the map via reflection.
			mapVal := results[0]
			if mapVal.Kind() == reflect.Map {
				for _, key := range mapVal.MapKeys() {
					statusCode := int(key.Int())
					val := mapVal.MapIndex(key).Interface()
					schema := g.extractBodySchema(val)
					if schema != nil {
						sKey := fmt.Sprintf("%d", statusCode)
						mt := MediaType{Schema: schema}
						// Apply named examples for this status code if available.
						if namedRespExamples != nil {
							if examples, ok := namedRespExamples[statusCode]; ok && len(examples) > 0 {
								mt.Examples = make(map[string]*ExampleObject, len(examples))
								for name, ex := range examples {
									ex := ex // capture
									mt.Examples[name] = &ex
								}
							}
						}
						// Resolve response description.
						desc := fmt.Sprintf("Response for status %d", statusCode)
						if respDescriptions != nil {
							if d, ok := respDescriptions[statusCode]; ok && d != "" {
								desc = d
							}
						}
						op.Responses[sKey] = &Response{
							Description: desc,
							Content:     map[string]MediaType{"application/json": mt},
						}
					}
				}
			}
		}
	}
}

// extractBodySchema extracts Kind() and Types() from a value via reflection.
// Works with any type that has these two methods (e.g., validation.BodySchema).
func (g *Generator) extractBodySchema(v interface{}) *SchemaObject {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)

	kindMethod := rv.MethodByName("Kind")
	typesMethod := rv.MethodByName("Types")
	if !kindMethod.IsValid() || !typesMethod.IsValid() {
		return nil
	}

	kindResults := kindMethod.Call(nil)
	typesResults := typesMethod.Call(nil)
	if len(kindResults) != 1 || len(typesResults) != 1 {
		return nil
	}

	kind := SchemaKind(kindResults[0].Int())
	typesSlice := typesResults[0].Interface().([]interface{})

	if len(typesSlice) == 0 {
		return &SchemaObject{Type: "object"}
	}

	switch kind {
	case SchemaScalar:
		return g.schemaGen.Generate(typesSlice[0])
	case SchemaOneOf:
		schemas := make([]*SchemaObject, len(typesSlice))
		for i, t := range typesSlice {
			schemas[i] = g.schemaGen.Generate(t)
		}
		return &SchemaObject{OneOf: schemas}
	case SchemaAnyOf:
		schemas := make([]*SchemaObject, len(typesSlice))
		for i, t := range typesSlice {
			schemas[i] = g.schemaGen.Generate(t)
		}
		return &SchemaObject{AnyOf: schemas}
	case SchemaAllOf:
		schemas := make([]*SchemaObject, len(typesSlice))
		for i, t := range typesSlice {
			schemas[i] = g.schemaGen.Generate(t)
		}
		return &SchemaObject{AllOf: schemas}
	default:
		return g.schemaGen.Generate(typesSlice[0])
	}
}

// enrichWithSecurity checks if the route also implements SecuredRouteAccessor
// and adds security requirements to the operation.
func (g *Generator) enrichWithSecurity(rt route.Route, op *Operation) {
	sr, ok := rt.(SecuredRouteAccessor)
	if !ok {
		return
	}

	schemes := sr.RequiredSchemes()
	if len(schemes) == 0 {
		return
	}

	for _, schemeName := range schemes {
		op.Security = append(op.Security, SecurityRequirement{
			schemeName: {},
		})
	}
}
