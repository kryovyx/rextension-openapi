// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: © 2026 Kryovyx

// Package openapi provides a Rex extension that generates an OpenAPI 3.1
// specification document from routes implementing the OpenAPIRoute interface.
//
// This file re-exports the reflection schema generator from corex/schema.
//
// The generator moved there because OpenAPI 3.1 and AsyncAPI 3.0 both use JSON
// Schema 2020-12, so it is genuinely shared rather than coincidentally similar
// (W23). W18's alternative — copy it into the AsyncAPI extension and record
// the duplication as debt — was dropped because the expensive half of this
// work is refactoring *this* module onto a shared package, and deferring it
// means doing that half twice, by which point the two copies have started
// disagreeing about oneOf.
//
// Both names are kept, so nothing else in this module changed: SchemaObject
// and SchemaGenerator still mean what they did, and still produce the same
// documents.
package openapi

import "github.com/kryovyx/corex/schema"

// SchemaGenerator builds JSON Schema objects from Go types and manages the
// shared component schemas for $ref deduplication. Aliased from corex/schema.
type SchemaGenerator = schema.Generator

// NewSchemaGenerator creates a new schema generator.
//
// A wrapper rather than `var NewSchemaGenerator = schema.NewGenerator`, which
// would make the constructor reassignable by anything in the process.
func NewSchemaGenerator() *SchemaGenerator { return schema.NewGenerator() }
