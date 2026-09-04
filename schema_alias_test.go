// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: © 2026 Kryovyx

// The reflection generator and the schema type it produces are corex/schema's
// now (W23). Its 720-line suite moved with it; what has to be checked here is
// only that these two names are still the same types, so a document this
// module builds and a document the AsyncAPI extension builds describe one wire
// format rather than two that happen to look alike.
package openapi_test

import (
	"testing"

	"github.com/kryovyx/corex"
	"github.com/kryovyx/corex/schema"
	openapi "github.com/kryovyx/rextension-openapi"
)

// Assigning across the boundary with no conversion: these stop compiling if
// either name is ever re-declared instead of aliased.
var (
	_ *schema.Schema        = (*openapi.SchemaObject)(nil)
	_ *openapi.SchemaObject = (*schema.Schema)(nil)
	_ *schema.Generator     = (*openapi.SchemaGenerator)(nil)
)

func TestNewSchemaGenerator_delegates(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	if g == nil {
		t.Fatal("NewSchemaGenerator returned nil")
	}
	if got := g.Components(); got == nil {
		t.Error("a fresh generator must have an initialised component map")
	}

	// A struct is hoisted into components and referenced, which is the one
	// behaviour of this generator the document depends on most: without it a
	// recursive type generates until the stack runs out.
	type user struct {
		Name string `json:"name"`
	}
	s := g.Generate(user{})
	if s.Ref != "#/components/schemas/user" {
		t.Fatalf("Ref = %q", s.Ref)
	}
	if _, ok := g.Components()["user"]; !ok {
		t.Error("the struct was not registered in components")
	}
}

// The document generator's extractBodySchema is a one-line delegation now, so
// the nil case is the part worth pinning: "this route declares no body" must
// not become `{"type":"object"}`, which says a body is expected.
func TestGenerateBodySchema_through_the_alias(t *testing.T) {
	g := openapi.NewSchemaGenerator()

	if got := g.GenerateBodySchema(nil); got != nil {
		t.Errorf("nil body schema gave %+v, want nil", got)
	}

	type payload struct {
		ID int `json:"id"`
	}
	s := g.GenerateBodySchema(corex.Scalar(payload{}))
	if s == nil || s.Ref != "#/components/schemas/payload" {
		t.Fatalf("Scalar gave %+v", s)
	}

	u := g.GenerateBodySchema(corex.OneOf(payload{}, struct{}{}))
	if u == nil || len(u.OneOf) != 2 {
		t.Fatalf("OneOf gave %+v", u)
	}
}
