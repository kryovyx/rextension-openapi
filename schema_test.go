package openapi_test

import (
	"testing"

	openapi "github.com/kryovyx/rextension-openapi"
)

// ---- helpers ----

func assertType(t *testing.T, s *openapi.SchemaObject, expected string) {
	t.Helper()
	if s.Type != expected {
		t.Errorf("expected type %q, got %q", expected, s.Type)
	}
}

func assertFormat(t *testing.T, s *openapi.SchemaObject, expected string) {
	t.Helper()
	if s.Format != expected {
		t.Errorf("expected format %q, got %q", expected, s.Format)
	}
}

// ---- primitive types ----

func TestGenerate_String(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	s := g.Generate("")
	assertType(t, s, "string")
}

func TestGenerate_Bool(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	s := g.Generate(true)
	assertType(t, s, "boolean")
}

func TestGenerate_Int(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	s := g.Generate(int(0))
	assertType(t, s, "integer")
}

func TestGenerate_Int8(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	s := g.Generate(int8(0))
	assertType(t, s, "integer")
}

func TestGenerate_Int16(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	s := g.Generate(int16(0))
	assertType(t, s, "integer")
}

func TestGenerate_Int32(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	s := g.Generate(int32(0))
	assertType(t, s, "integer")
}

func TestGenerate_Int64(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	s := g.Generate(int64(0))
	assertType(t, s, "integer")
}

func TestGenerate_Uint(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	s := g.Generate(uint(0))
	assertType(t, s, "integer")
}

func TestGenerate_Uint8(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	s := g.Generate(uint8(0))
	assertType(t, s, "integer")
}

func TestGenerate_Uint16(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	s := g.Generate(uint16(0))
	assertType(t, s, "integer")
}

func TestGenerate_Uint32(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	s := g.Generate(uint32(0))
	assertType(t, s, "integer")
}

func TestGenerate_Uint64(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	s := g.Generate(uint64(0))
	assertType(t, s, "integer")
}

func TestGenerate_Float32(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	s := g.Generate(float32(0))
	assertType(t, s, "number")
	assertFormat(t, s, "float")
}

func TestGenerate_Float64(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	s := g.Generate(float64(0))
	assertType(t, s, "number")
	assertFormat(t, s, "double")
}

// ---- slice / array -> type:array ----

func TestGenerate_Slice(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	s := g.Generate([]string{})
	assertType(t, s, "array")
	if s.Items == nil {
		t.Fatal("expected items")
	}
	assertType(t, s.Items, "string")
}

func TestGenerate_SliceOfInt(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	s := g.Generate([]int{})
	assertType(t, s, "array")
	if s.Items == nil {
		t.Fatal("expected items")
	}
	assertType(t, s.Items, "integer")
}

// ---- map -> type:object ----

func TestGenerate_Map(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	s := g.Generate(map[string]string{})
	assertType(t, s, "object")
}

// ---- interface -> type:object ----

func TestGenerate_Interface(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	var v interface{}
	s := g.Generate(&v)
	// pointer to interface: nullable object
	assertType(t, s, "object")
}

// ---- pointer -> nullable:true ----

func TestGenerate_Pointer(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	var v *string
	s := g.Generate(v)
	assertType(t, s, "string")
	if !s.Nullable {
		t.Error("expected nullable true for pointer")
	}
}

func TestGenerate_PointerToInt(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	var v *int
	s := g.Generate(v)
	assertType(t, s, "integer")
	if !s.Nullable {
		t.Error("expected nullable true for pointer to int")
	}
}

// ---- nil -> type:object ----

func TestGenerate_Nil(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	s := g.Generate(nil)
	assertType(t, s, "object")
}

// ---- struct -> $ref + component registration ----

type SimpleStruct struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestGenerate_Struct(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	s := g.Generate(SimpleStruct{})

	if s.Ref != "#/components/schemas/SimpleStruct" {
		t.Errorf("expected $ref, got %q", s.Ref)
	}

	comps := g.Components()
	schema, ok := comps["SimpleStruct"]
	if !ok {
		t.Fatal("expected SimpleStruct in components")
	}
	if schema.Type != "object" {
		t.Errorf("expected type object, got %q", schema.Type)
	}
	if _, ok := schema.Properties["name"]; !ok {
		t.Error("expected 'name' property")
	}
	if _, ok := schema.Properties["age"]; !ok {
		t.Error("expected 'age' property")
	}
}

// ---- already-generated struct returns $ref without re-registering ----

func TestGenerate_StructDuplicate(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	s1 := g.Generate(SimpleStruct{})
	s2 := g.Generate(SimpleStruct{})

	if s1.Ref != s2.Ref {
		t.Errorf("expected same $ref, got %q and %q", s1.Ref, s2.Ref)
	}
	// Components should have exactly one entry.
	if len(g.Components()) != 1 {
		t.Errorf("expected 1 component, got %d", len(g.Components()))
	}
}

// ---- anonymous struct -> inlined (no $ref) ----

func TestGenerate_AnonymousStruct(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	v := struct {
		X string `json:"x"`
		Y int    `json:"y"`
	}{}
	s := g.Generate(v)

	if s.Ref != "" {
		t.Errorf("expected no $ref for anonymous struct, got %q", s.Ref)
	}
	if s.Type != "object" {
		t.Errorf("expected type object, got %q", s.Type)
	}
	if _, ok := s.Properties["x"]; !ok {
		t.Error("expected 'x' property")
	}
	if _, ok := s.Properties["y"]; !ok {
		t.Error("expected 'y' property")
	}
}

// ---- struct with JSON tags ----

type JSONTagStruct struct {
	FieldA string `json:"field_a"`
	FieldB string `json:"field_b,omitempty"`
	Hidden string `json:"-"`
	NoTag  string
}

func TestGenerate_JSONTags(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	g.Generate(JSONTagStruct{})

	schema := g.Components()["JSONTagStruct"]
	if schema == nil {
		t.Fatal("expected JSONTagStruct in components")
	}
	if _, ok := schema.Properties["field_a"]; !ok {
		t.Error("expected 'field_a' from json tag")
	}
	if _, ok := schema.Properties["field_b"]; !ok {
		t.Error("expected 'field_b' from json tag")
	}
	if _, ok := schema.Properties["Hidden"]; ok {
		t.Error("expected hidden field to be excluded (json:\"-\")")
	}
	if _, ok := schema.Properties["NoTag"]; !ok {
		t.Error("expected NoTag field with original name when no json tag")
	}
}

// ---- struct with validate tags ----

type ValidatedStruct struct {
	Name  string `json:"name" validate:"required,min=3,max=50"`
	Email string `json:"email" validate:"required"`
	Age   int    `json:"age" validate:"min=0,max=150"`
	Role  string `json:"role" validate:"oneof=admin user guest"`
	Code  string `json:"code" validate:"len=6"`
}

func TestGenerate_ValidateTags_Required(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	g.Generate(ValidatedStruct{})

	schema := g.Components()["ValidatedStruct"]
	if schema == nil {
		t.Fatal("expected ValidatedStruct in components")
	}

	found := map[string]bool{}
	for _, r := range schema.Required {
		found[r] = true
	}
	if !found["name"] {
		t.Error("expected 'name' in required")
	}
	if !found["email"] {
		t.Error("expected 'email' in required")
	}
	// age has no 'required' validate tag
	if found["age"] {
		t.Error("expected 'age' NOT in required")
	}
}

func TestGenerate_ValidateTags_MinMaxString(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	g.Generate(ValidatedStruct{})

	schema := g.Components()["ValidatedStruct"]
	nameProp := schema.Properties["name"]
	if nameProp.MinLength == nil || *nameProp.MinLength != 3 {
		t.Errorf("expected minLength 3 for name, got %v", nameProp.MinLength)
	}
	if nameProp.MaxLength == nil || *nameProp.MaxLength != 50 {
		t.Errorf("expected maxLength 50 for name, got %v", nameProp.MaxLength)
	}
}

func TestGenerate_ValidateTags_MinMaxNumber(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	g.Generate(ValidatedStruct{})

	schema := g.Components()["ValidatedStruct"]
	ageProp := schema.Properties["age"]
	if ageProp.Minimum == nil || *ageProp.Minimum != 0 {
		t.Errorf("expected minimum 0 for age, got %v", ageProp.Minimum)
	}
	if ageProp.Maximum == nil || *ageProp.Maximum != 150 {
		t.Errorf("expected maximum 150 for age, got %v", ageProp.Maximum)
	}
}

func TestGenerate_ValidateTags_Oneof(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	g.Generate(ValidatedStruct{})

	schema := g.Components()["ValidatedStruct"]
	roleProp := schema.Properties["role"]
	if len(roleProp.Enum) != 3 {
		t.Fatalf("expected 3 enum values for role, got %d", len(roleProp.Enum))
	}
	expected := map[string]bool{"admin": true, "user": true, "guest": true}
	for _, e := range roleProp.Enum {
		if !expected[e.(string)] {
			t.Errorf("unexpected enum value %v", e)
		}
	}
}

func TestGenerate_ValidateTags_Len(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	g.Generate(ValidatedStruct{})

	schema := g.Components()["ValidatedStruct"]
	codeProp := schema.Properties["code"]
	if codeProp.MinLength == nil || *codeProp.MinLength != 6 {
		t.Errorf("expected minLength 6, got %v", codeProp.MinLength)
	}
	if codeProp.MaxLength == nil || *codeProp.MaxLength != 6 {
		t.Errorf("expected maxLength 6, got %v", codeProp.MaxLength)
	}
}

// ---- struct with description, example, format, enum tags ----

type AnnotatedStruct struct {
	ID     string `json:"id" description:"The unique identifier" example:"abc-123" format:"uuid"`
	Status string `json:"status" enum:"active,inactive,pending"`
}

func TestGenerate_DescriptionTag(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	g.Generate(AnnotatedStruct{})

	schema := g.Components()["AnnotatedStruct"]
	idProp := schema.Properties["id"]
	if idProp.Description != "The unique identifier" {
		t.Errorf("expected description, got %q", idProp.Description)
	}
}

func TestGenerate_ExampleTag(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	g.Generate(AnnotatedStruct{})

	schema := g.Components()["AnnotatedStruct"]
	idProp := schema.Properties["id"]
	if len(idProp.Examples) == 0 || idProp.Examples[0] != "abc-123" {
		t.Errorf("expected examples[0] 'abc-123', got %v", idProp.Examples)
	}
}

func TestGenerate_FormatTag(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	g.Generate(AnnotatedStruct{})

	schema := g.Components()["AnnotatedStruct"]
	idProp := schema.Properties["id"]
	if idProp.Format != "uuid" {
		t.Errorf("expected format 'uuid', got %q", idProp.Format)
	}
}

func TestGenerate_EnumTag(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	g.Generate(AnnotatedStruct{})

	schema := g.Components()["AnnotatedStruct"]
	statusProp := schema.Properties["status"]
	if len(statusProp.Enum) != 3 {
		t.Fatalf("expected 3 enum values, got %d", len(statusProp.Enum))
	}
	expected := map[string]bool{"active": true, "inactive": true, "pending": true}
	for _, e := range statusProp.Enum {
		if !expected[e.(string)] {
			t.Errorf("unexpected enum value %v", e)
		}
	}
}

// ---- embedded/anonymous structs ----

type Base struct {
	ID        string `json:"id" validate:"required"`
	CreatedAt string `json:"created_at"`
}

type Extended struct {
	Base
	Name string `json:"name" validate:"required"`
}

func TestGenerate_EmbeddedStruct(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	g.Generate(Extended{})

	schema := g.Components()["Extended"]
	if schema == nil {
		t.Fatal("expected Extended in components")
	}

	// Properties from Base should be merged.
	if _, ok := schema.Properties["id"]; !ok {
		t.Error("expected 'id' from embedded Base")
	}
	if _, ok := schema.Properties["created_at"]; !ok {
		t.Error("expected 'created_at' from embedded Base")
	}
	if _, ok := schema.Properties["name"]; !ok {
		t.Error("expected 'name' from Extended")
	}

	// Required should be merged.
	found := map[string]bool{}
	for _, r := range schema.Required {
		found[r] = true
	}
	if !found["id"] {
		t.Error("expected 'id' in required (from Base)")
	}
	if !found["name"] {
		t.Error("expected 'name' in required (from Extended)")
	}
}

// ---- slice of structs ----

func TestGenerate_SliceOfStruct(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	s := g.Generate([]SimpleStruct{})
	assertType(t, s, "array")
	if s.Items == nil {
		t.Fatal("expected items")
	}
	if s.Items.Ref != "#/components/schemas/SimpleStruct" {
		t.Errorf("expected $ref in items, got %q", s.Items.Ref)
	}
}

// ---- pointer to struct ----

func TestGenerate_PointerToStruct(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	var v *SimpleStruct
	s := g.Generate(v)
	if s.Ref != "#/components/schemas/SimpleStruct" {
		t.Errorf("expected $ref, got %q", s.Ref)
	}
	if !s.Nullable {
		t.Error("expected nullable true for pointer to struct")
	}
}

// ---- unexported fields skipped ----

type WithUnexported struct {
	Public  string `json:"public"`
	private string //nolint:unused
}

func TestGenerate_UnexportedFieldsSkipped(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	g.Generate(WithUnexported{})

	schema := g.Components()["WithUnexported"]
	if _, ok := schema.Properties["private"]; ok {
		t.Error("expected unexported field to be skipped")
	}
	if _, ok := schema.Properties["public"]; !ok {
		t.Error("expected 'public' property")
	}
}

// ---- Components() returns accumulated schemas ----

func TestSchemaGenerator_Components(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	comps := g.Components()
	if len(comps) != 0 {
		t.Errorf("expected empty components, got %d", len(comps))
	}

	g.Generate(SimpleStruct{})
	comps = g.Components()
	if len(comps) != 1 {
		t.Errorf("expected 1 component, got %d", len(comps))
	}
}

// ---- GenerateBodySchema ----

type mockBodySchema struct {
	kind  openapi.SchemaKind
	types []interface{}
}

func (m *mockBodySchema) Kind() openapi.SchemaKind { return m.kind }
func (m *mockBodySchema) Types() []interface{}     { return m.types }

func TestGenerateBodySchema_Scalar(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	bs := &mockBodySchema{kind: openapi.SchemaScalar, types: []interface{}{SimpleStruct{}}}
	s := g.GenerateBodySchema(bs)

	if s.Ref != "#/components/schemas/SimpleStruct" {
		t.Errorf("expected $ref for scalar, got %q", s.Ref)
	}
}

func TestGenerateBodySchema_OneOf(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	bs := &mockBodySchema{
		kind:  openapi.SchemaOneOf,
		types: []interface{}{"", int(0)},
	}
	s := g.GenerateBodySchema(bs)

	if len(s.OneOf) != 2 {
		t.Fatalf("expected 2 oneOf schemas, got %d", len(s.OneOf))
	}
	assertType(t, s.OneOf[0], "string")
	assertType(t, s.OneOf[1], "integer")
}

func TestGenerateBodySchema_AnyOf(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	bs := &mockBodySchema{
		kind:  openapi.SchemaAnyOf,
		types: []interface{}{"", true},
	}
	s := g.GenerateBodySchema(bs)

	if len(s.AnyOf) != 2 {
		t.Fatalf("expected 2 anyOf schemas, got %d", len(s.AnyOf))
	}
	assertType(t, s.AnyOf[0], "string")
	assertType(t, s.AnyOf[1], "boolean")
}

func TestGenerateBodySchema_AllOf(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	bs := &mockBodySchema{
		kind:  openapi.SchemaAllOf,
		types: []interface{}{SimpleStruct{}, Base{}},
	}
	s := g.GenerateBodySchema(bs)

	if len(s.AllOf) != 2 {
		t.Fatalf("expected 2 allOf schemas, got %d", len(s.AllOf))
	}
}

func TestGenerateBodySchema_EmptyTypes(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	bs := &mockBodySchema{kind: openapi.SchemaScalar, types: []interface{}{}}
	s := g.GenerateBodySchema(bs)

	assertType(t, s, "object")
}

func TestGenerateBodySchema_UnknownKind(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	bs := &mockBodySchema{
		kind:  openapi.SchemaKind(999),
		types: []interface{}{""},
	}
	s := g.GenerateBodySchema(bs)

	// Unknown kind defaults to generating from the first type.
	assertType(t, s, "string")
}

// ---- dedupStrings tested indirectly via required merging with embedded ----

func TestGenerate_DedupRequired(t *testing.T) {
	// If both embedded and parent declare the same required field,
	// it should appear only once.
	type DupBase struct {
		ID string `json:"id" validate:"required"`
	}
	type DupChild struct {
		DupBase
		ID string `json:"id" validate:"required"` //nolint:govet
	}

	g := openapi.NewSchemaGenerator()
	g.Generate(DupChild{})

	schema := g.Components()["DupChild"]
	count := 0
	for _, r := range schema.Required {
		if r == "id" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 'id' in required exactly once, got %d", count)
	}
}

// ---- double pointer ----

func TestGenerate_DoublePointer(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	var v **string
	s := g.Generate(v)
	assertType(t, s, "string")
	if !s.Nullable {
		t.Error("expected nullable for double pointer")
	}
}

// ---- empty enum tag ----

type EmptyEnumStruct struct {
	V string `json:"v" enum:""`
}

func TestGenerate_EmptyEnumTag(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	g.Generate(EmptyEnumStruct{})

	schema := g.Components()["EmptyEnumStruct"]
	vProp := schema.Properties["v"]
	if len(vProp.Enum) != 0 {
		t.Errorf("expected no enum values for empty enum tag, got %d", len(vProp.Enum))
	}
}

// ---- nested struct ----

type Inner struct {
	Value string `json:"value"`
}

type Outer struct {
	Inner Inner `json:"inner"`
}

func TestGenerate_NestedStruct(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	g.Generate(Outer{})

	comps := g.Components()
	if _, ok := comps["Outer"]; !ok {
		t.Error("expected Outer in components")
	}
	if _, ok := comps["Inner"]; !ok {
		t.Error("expected Inner in components")
	}

	outerSchema := comps["Outer"]
	innerProp := outerSchema.Properties["inner"]
	if innerProp.Ref != "#/components/schemas/Inner" {
		t.Errorf("expected $ref for nested struct, got %q", innerProp.Ref)
	}
}

// ---- map of structs ----

func TestGenerate_MapOfString(t *testing.T) {
	g := openapi.NewSchemaGenerator()
	s := g.Generate(map[string]int{})
	assertType(t, s, "object")
}
