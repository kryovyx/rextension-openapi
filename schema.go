// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: © 2026 Kryovyx

// Package openapi provides a Rex extension that generates an OpenAPI 3.1
// specification document from routes implementing the OpenAPIRoute interface.
//
// This file implements a reflect-based JSON Schema generator that converts
// Go struct types into JSON Schema 2020-12 objects. It handles:
//   - Primitive types (string, int, float, bool)
//   - Structs with json tags
//   - Slices/arrays
//   - Pointers (nullable)
//   - Maps
//   - validate tags → required, min/max, enum, minLength/maxLength
//   - Nested struct $ref hoisting into components/schemas
package openapi

import (
	"reflect"
	"strconv"
	"strings"
)

// SchemaGenerator builds JSON Schema objects from Go types and manages
// the shared component schemas for $ref deduplication.
type SchemaGenerator struct {
	// components accumulates named schemas for reuse via $ref.
	components map[string]*SchemaObject
}

// NewSchemaGenerator creates a new schema generator.
func NewSchemaGenerator() *SchemaGenerator {
	return &SchemaGenerator{
		components: make(map[string]*SchemaObject),
	}
}

// Components returns the accumulated component schemas.
func (g *SchemaGenerator) Components() map[string]*SchemaObject {
	return g.components
}

// Generate produces a JSON Schema for the given Go value's type.
// Structs are registered in components and referenced via $ref.
func (g *SchemaGenerator) Generate(v interface{}) *SchemaObject {
	t := reflect.TypeOf(v)
	if t == nil {
		return &SchemaObject{Type: "object"}
	}
	return g.generateType(t)
}

func (g *SchemaGenerator) generateType(t reflect.Type) *SchemaObject {
	// Dereference pointers.
	nullable := false
	for t.Kind() == reflect.Ptr {
		nullable = true
		t = t.Elem()
	}

	schema := g.generateNonPtr(t)
	if nullable {
		schema.Nullable = true
	}
	return schema
}

func (g *SchemaGenerator) generateNonPtr(t reflect.Type) *SchemaObject {
	switch t.Kind() {
	case reflect.String:
		return &SchemaObject{Type: "string"}
	case reflect.Bool:
		return &SchemaObject{Type: "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &SchemaObject{Type: "integer"}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &SchemaObject{Type: "integer"}
	case reflect.Float32:
		return &SchemaObject{Type: "number", Format: "float"}
	case reflect.Float64:
		return &SchemaObject{Type: "number", Format: "double"}
	case reflect.Slice, reflect.Array:
		items := g.generateType(t.Elem())
		return &SchemaObject{Type: "array", Items: items}
	case reflect.Map:
		// Maps become object with no fixed properties.
		return &SchemaObject{Type: "object"}
	case reflect.Struct:
		return g.generateStruct(t)
	case reflect.Interface:
		return &SchemaObject{Type: "object"}
	default:
		return &SchemaObject{Type: "object"}
	}
}

func (g *SchemaGenerator) generateStruct(t reflect.Type) *SchemaObject {
	name := t.Name()
	if name == "" {
		// Anonymous struct — inline.
		return g.buildStructSchema(t)
	}

	// Check if already generated.
	if _, exists := g.components[name]; exists {
		return &SchemaObject{Ref: "#/components/schemas/" + name}
	}

	// Register a placeholder to break cycles.
	g.components[name] = nil
	schema := g.buildStructSchema(t)
	g.components[name] = schema

	return &SchemaObject{Ref: "#/components/schemas/" + name}
}

func (g *SchemaGenerator) buildStructSchema(t reflect.Type) *SchemaObject {
	schema := &SchemaObject{
		Type:       "object",
		Properties: make(map[string]*SchemaObject),
	}

	var required []string

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		// Handle embedded structs.
		if field.Anonymous {
			embedded := g.generateType(field.Type)
			// Inline properties from the embedded struct if it was resolved.
			if embedded.Ref != "" {
				// Look up the component to inline.
				refName := strings.TrimPrefix(embedded.Ref, "#/components/schemas/")
				if comp, ok := g.components[refName]; ok && comp != nil {
					for k, v := range comp.Properties {
						schema.Properties[k] = v
					}
					required = append(required, comp.Required...)
				}
			} else if embedded.Properties != nil {
				for k, v := range embedded.Properties {
					schema.Properties[k] = v
				}
				required = append(required, embedded.Required...)
			}
			continue
		}

		// Determine JSON field name.
		jsonTag := field.Tag.Get("json")
		fieldName := field.Name
		omitempty := false
		if jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] == "-" {
				continue // Excluded from JSON.
			}
			if parts[0] != "" {
				fieldName = parts[0]
			}
			for _, p := range parts[1:] {
				if p == "omitempty" {
					omitempty = true
				}
			}
		}

		propSchema := g.generateType(field.Type)

		// Apply description struct tag.
		if desc := field.Tag.Get("description"); desc != "" {
			propSchema.Description = desc
		}
		// Apply example struct tag.
		// OpenAPI 3.1 uses the JSON Schema 2020-12 'examples' array instead of
		// the deprecated scalar 'example' keyword.
		if ex := field.Tag.Get("example"); ex != "" {
			propSchema.Examples = []interface{}{ex}
		}
		// Apply format override struct tag.
		if fmtTag := field.Tag.Get("format"); fmtTag != "" {
			propSchema.Format = fmtTag
		}
		// Apply enum struct tag (comma-separated values, e.g. enum:"a,b,c").
		if enumTag := field.Tag.Get("enum"); enumTag != "" {
			vals := strings.Split(enumTag, ",")
			enums := make([]interface{}, 0, len(vals))
			for _, v := range vals {
				v = strings.TrimSpace(v)
				if v != "" {
					enums = append(enums, v)
				}
			}
			if len(enums) > 0 {
				propSchema.Enum = enums
			}
		}

		// Parse validate tag for constraints.
		validateTag := field.Tag.Get("validate")
		if validateTag != "" {
			applyValidateTag(propSchema, validateTag, &required, fieldName, omitempty)
		} else if !omitempty {
			// No validate tag, not omitempty — consider required by convention.
		}

		schema.Properties[fieldName] = propSchema
	}

	if len(required) > 0 {
		schema.Required = dedupStrings(required)
	}

	return schema
}

// applyValidateTag parses a go-playground validate tag and applies constraints
// to the schema. It also adds to the required list when appropriate.
func applyValidateTag(schema *SchemaObject, tag string, required *[]string, fieldName string, omitempty bool) {
	parts := strings.Split(tag, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch {
		case part == "required":
			*required = append(*required, fieldName)
		case strings.HasPrefix(part, "min="):
			if v, err := strconv.ParseFloat(part[4:], 64); err == nil {
				if schema.Type == "string" {
					iv := int(v)
					schema.MinLength = &iv
				} else {
					schema.Minimum = &v
				}
			}
		case strings.HasPrefix(part, "max="):
			if v, err := strconv.ParseFloat(part[4:], 64); err == nil {
				if schema.Type == "string" {
					iv := int(v)
					schema.MaxLength = &iv
				} else {
					schema.Maximum = &v
				}
			}
		case strings.HasPrefix(part, "oneof="):
			vals := strings.Fields(part[6:])
			enums := make([]interface{}, len(vals))
			for i, v := range vals {
				enums[i] = v
			}
			schema.Enum = enums
		case strings.HasPrefix(part, "len="):
			if v, err := strconv.Atoi(part[4:]); err == nil {
				schema.MinLength = &v
				schema.MaxLength = &v
			}
		}
	}
}

// GenerateBodySchema generates a SchemaObject from a BodySchemaAccessor,
// handling Scalar, OneOf, AnyOf, and AllOf kinds.
func (g *SchemaGenerator) GenerateBodySchema(bs BodySchemaAccessor) *SchemaObject {
	types := bs.Types()
	if len(types) == 0 {
		return &SchemaObject{Type: "object"}
	}

	switch SchemaKind(bs.Kind()) {
	case SchemaScalar:
		return g.Generate(types[0])
	case SchemaOneOf:
		schemas := make([]*SchemaObject, len(types))
		for i, t := range types {
			schemas[i] = g.Generate(t)
		}
		return &SchemaObject{OneOf: schemas}
	case SchemaAnyOf:
		schemas := make([]*SchemaObject, len(types))
		for i, t := range types {
			schemas[i] = g.Generate(t)
		}
		return &SchemaObject{AnyOf: schemas}
	case SchemaAllOf:
		schemas := make([]*SchemaObject, len(types))
		for i, t := range types {
			schemas[i] = g.Generate(t)
		}
		return &SchemaObject{AllOf: schemas}
	default:
		return g.Generate(types[0])
	}
}

// dedupStrings removes duplicates from a string slice preserving order.
func dedupStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
