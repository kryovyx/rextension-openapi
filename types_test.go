package openapi_test

import (
	"encoding/json"
	"testing"

	openapi "github.com/kryovyx/rextension-openapi"
)

func TestPathItem_SetOperation_GET(t *testing.T) {
	pi := &openapi.PathItem{}
	op := &openapi.Operation{OperationID: "getOp"}

	// setOperation is unexported, so we test via JSON round-trip through the
	// Document/Generator or directly via the exported struct fields.
	// Since setOperation is unexported, we verify behavior through Generator tests.
	// Here we test the struct serialization directly.

	pi.Get = op
	data, err := json.Marshal(pi)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if _, ok := result["get"]; !ok {
		t.Error("expected 'get' key in JSON")
	}
}

func TestPathItem_AllMethods(t *testing.T) {
	methods := []struct {
		field string
		apply func(*openapi.PathItem, *openapi.Operation)
	}{
		{"get", func(pi *openapi.PathItem, op *openapi.Operation) { pi.Get = op }},
		{"post", func(pi *openapi.PathItem, op *openapi.Operation) { pi.Post = op }},
		{"put", func(pi *openapi.PathItem, op *openapi.Operation) { pi.Put = op }},
		{"patch", func(pi *openapi.PathItem, op *openapi.Operation) { pi.Patch = op }},
		{"delete", func(pi *openapi.PathItem, op *openapi.Operation) { pi.Delete = op }},
		{"head", func(pi *openapi.PathItem, op *openapi.Operation) { pi.Head = op }},
		{"options", func(pi *openapi.PathItem, op *openapi.Operation) { pi.Options = op }},
	}

	for _, m := range methods {
		t.Run(m.field, func(t *testing.T) {
			pi := &openapi.PathItem{}
			op := &openapi.Operation{OperationID: m.field + "Op"}
			m.apply(pi, op)

			data, err := json.Marshal(pi)
			if err != nil {
				t.Fatalf("marshal error: %v", err)
			}
			var result map[string]interface{}
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if _, ok := result[m.field]; !ok {
				t.Errorf("expected %q key in JSON", m.field)
			}
		})
	}
}

func TestDocument_JSON(t *testing.T) {
	doc := openapi.Document{
		OpenAPI: "3.1.0",
		Info: openapi.Info{
			Title:       "Test API",
			Version:     "1.0.0",
			Description: "Desc",
		},
		Paths: map[string]*openapi.PathItem{
			"/test": {
				Get: &openapi.Operation{
					OperationID: "getTest",
					Summary:     "Get test",
				},
			},
		},
	}

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if out["openapi"] != "3.1.0" {
		t.Errorf("expected openapi 3.1.0, got %v", out["openapi"])
	}
	info := out["info"].(map[string]interface{})
	if info["title"] != "Test API" {
		t.Errorf("expected title 'Test API', got %v", info["title"])
	}
}

func TestSchemaObject_Ref(t *testing.T) {
	s := openapi.SchemaObject{
		Ref: "#/components/schemas/Foo",
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if out["$ref"] != "#/components/schemas/Foo" {
		t.Errorf("expected $ref, got %v", out["$ref"])
	}
}

func TestSchemaObject_Properties(t *testing.T) {
	minLen := 3
	s := openapi.SchemaObject{
		Type: "object",
		Properties: map[string]*openapi.SchemaObject{
			"name": {Type: "string", MinLength: &minLen},
		},
		Required: []string{"name"},
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if out["type"] != "object" {
		t.Errorf("expected type 'object', got %v", out["type"])
	}
	props := out["properties"].(map[string]interface{})
	if _, ok := props["name"]; !ok {
		t.Error("expected 'name' property")
	}
}

func TestSchemaObject_OneOf_AnyOf_AllOf(t *testing.T) {
	s := openapi.SchemaObject{
		OneOf: []*openapi.SchemaObject{
			{Type: "string"},
			{Type: "integer"},
		},
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if _, ok := out["oneOf"]; !ok {
		t.Error("expected oneOf in JSON")
	}

	s2 := openapi.SchemaObject{
		AnyOf: []*openapi.SchemaObject{{Type: "string"}},
	}
	data2, _ := json.Marshal(s2)
	var out2 map[string]interface{}
	json.Unmarshal(data2, &out2)
	if _, ok := out2["anyOf"]; !ok {
		t.Error("expected anyOf in JSON")
	}

	s3 := openapi.SchemaObject{
		AllOf: []*openapi.SchemaObject{{Type: "string"}},
	}
	data3, _ := json.Marshal(s3)
	var out3 map[string]interface{}
	json.Unmarshal(data3, &out3)
	if _, ok := out3["allOf"]; !ok {
		t.Error("expected allOf in JSON")
	}
}

func TestSchemaObject_Nullable(t *testing.T) {
	s := openapi.SchemaObject{
		Type:     "string",
		Nullable: true,
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if out["nullable"] != true {
		t.Errorf("expected nullable true, got %v", out["nullable"])
	}
}

func TestSchemaObject_MinMax(t *testing.T) {
	min := 1.0
	max := 100.0
	s := openapi.SchemaObject{
		Type:    "integer",
		Minimum: &min,
		Maximum: &max,
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if out["minimum"].(float64) != 1.0 {
		t.Errorf("expected minimum 1, got %v", out["minimum"])
	}
	if out["maximum"].(float64) != 100.0 {
		t.Errorf("expected maximum 100, got %v", out["maximum"])
	}
}

func TestSecurityRequirement_JSON(t *testing.T) {
	sr := openapi.SecurityRequirement{
		"bearer": {},
	}
	data, err := json.Marshal(sr)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if _, ok := out["bearer"]; !ok {
		t.Error("expected 'bearer' key in JSON")
	}
}

func TestComponents_JSON(t *testing.T) {
	c := openapi.Components{
		Schemas: map[string]*openapi.SchemaObject{
			"User": {Type: "object"},
		},
		SecuritySchemes: map[string]*openapi.SecuritySchemeObject{
			"bearer": {Type: "http", Scheme: "bearer"},
		},
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if _, ok := out["schemas"]; !ok {
		t.Error("expected 'schemas' key")
	}
	if _, ok := out["securitySchemes"]; !ok {
		t.Error("expected 'securitySchemes' key")
	}
}

func TestExampleObject_JSON(t *testing.T) {
	ex := openapi.ExampleObject{
		Summary:     "A test example",
		Description: "Description",
		Value:       map[string]string{"key": "val"},
	}
	data, err := json.Marshal(ex)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if out["summary"] != "A test example" {
		t.Errorf("expected summary, got %v", out["summary"])
	}
}

func TestOmitemptyFields(t *testing.T) {
	// Empty fields with omitempty should not appear in JSON.
	doc := openapi.Document{
		OpenAPI: "3.1.0",
		Info: openapi.Info{
			Title:   "T",
			Version: "1",
		},
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	// paths should be omitted when nil
	if _, ok := out["paths"]; ok {
		t.Error("expected paths to be omitted when nil")
	}
	// components should be omitted when nil
	if _, ok := out["components"]; ok {
		t.Error("expected components to be omitted when nil")
	}
}
