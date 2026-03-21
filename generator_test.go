package openapi_test

import (
	"testing"

	"github.com/kryovyx/rex/route"
	openapi "github.com/kryovyx/rextension-openapi"
)

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

func (r *mockRoute) Method() string             { return r.method }
func (r *mockRoute) Path() string               { return r.path }
func (r *mockRoute) Handler() route.HandlerFunc { return func(ctx route.Context) {} }
func (r *mockRoute) OperationID() string        { return r.operationID }
func (r *mockRoute) Summary() string            { return r.summary }
func (r *mockRoute) Description() string        { return r.description }
func (r *mockRoute) Tags() []string             { return r.tags }

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

func (r *plainRoute) Method() string             { return r.method }
func (r *plainRoute) Path() string               { return r.path }
func (r *plainRoute) Handler() route.HandlerFunc { return func(ctx route.Context) {} }

// BodySchema mocks for validation enrichment
type testBodySchema struct {
	kind  int
	types []interface{}
}

func (b *testBodySchema) Kind() int            { return b.kind }
func (b *testBodySchema) Types() []interface{} { return b.types }

// mockValidatedRoute implements route.Route + OpenAPIRoute + RequestBody/Responses
type mockValidatedRoute struct {
	mockRoute
	reqBody   *testBodySchema
	responses map[int]*testBodySchema
}

func (r *mockValidatedRoute) RequestBody() *testBodySchema { return r.reqBody }
func (r *mockValidatedRoute) Responses() map[int]*testBodySchema {
	return r.responses
}

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

	routes := []route.Route{
		&mockRoute{
			method:      "GET",
			path:        "/users",
			operationID: "listUsers",
			summary:     "List users",
			description: "Returns all users",
			tags:        []string{"users"},
		},
	}

	doc, err := gen.Generate(routes)
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

	routes := []route.Route{
		&plainRoute{method: "GET", path: "/health"},
	}

	doc, err := gen.Generate(routes)
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

	routes := []route.Route{
		&mockRoute{method: "GET", path: "/test", operationID: "test"},
	}

	doc, _ := gen.Generate(routes)

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
	routes := make([]route.Route, len(methods))
	for i, m := range methods {
		routes[i] = &mockRoute{method: m, path: "/test", operationID: m + "Op"}
	}

	doc, _ := gen.Generate(routes)

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

	routes := []route.Route{
		&mockRoute{method: "GET", path: "/a", operationID: "getA"},
		&mockRoute{method: "POST", path: "/b", operationID: "postB"},
	}

	doc, _ := gen.Generate(routes)

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
			mockSecurityScheme: mockSecurityScheme{
				name:        "bearer",
				typ:         "http",
				description: "Bearer token auth",
				challenge:   "Bearer",
			},
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
			mockSecurityScheme: mockSecurityScheme{
				name: "apiKey",
				typ:  "apiKey",
			},
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

func TestGenerator_Generate_RouteWithSecurity(t *testing.T) {
	cfg := openapi.Config{Title: "T", Version: "1"}
	schemes := []openapi.SecuritySchemeAccessor{
		&mockSecurityScheme{name: "bearer", typ: "http", challenge: "Bearer"},
	}
	gen := openapi.NewGenerator(cfg, schemes)

	routes := []route.Route{
		&mockSecuredRoute{
			mockRoute: mockRoute{method: "GET", path: "/secure", operationID: "secureOp"},
			schemes:   []string{"bearer"},
		},
	}

	doc, _ := gen.Generate(routes)

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

	routes := []route.Route{
		&mockSecuredRoute{
			mockRoute: mockRoute{method: "GET", path: "/public", operationID: "publicOp"},
			schemes:   nil,
		},
	}

	doc, _ := gen.Generate(routes)

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

	routes := []route.Route{
		&mockValidatedRoute{
			mockRoute: mockRoute{method: "POST", path: "/users", operationID: "createUser"},
			reqBody:   &testBodySchema{kind: 0, types: []interface{}{ReqBody{}}},
			responses: map[int]*testBodySchema{
				201: {kind: 0, types: []interface{}{RespBody{}}},
			},
		},
	}

	doc, err := gen.Generate(routes)
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

	routes := []route.Route{
		&mockValidatedRoute{
			mockRoute: mockRoute{method: "GET", path: "/items", operationID: "listItems"},
			reqBody:   nil,
			responses: nil,
		},
	}

	doc, _ := gen.Generate(routes)

	op := doc.Paths["/items"].Get
	if op.RequestBody != nil {
		t.Error("expected no request body for nil")
	}
}

// ---- Examples providers ----

func TestGenerator_Generate_WithExamples(t *testing.T) {
	cfg := openapi.Config{Title: "T", Version: "1"}
	gen := openapi.NewGenerator(cfg, nil)

	routes := []route.Route{
		&mockRouteWithExamples{
			mockValidatedRoute: mockValidatedRoute{
				mockRoute: mockRoute{method: "POST", path: "/pay", operationID: "pay"},
				reqBody:   &testBodySchema{kind: 0, types: []interface{}{ReqBody{}}},
				responses: map[int]*testBodySchema{
					200: {kind: 0, types: []interface{}{RespBody{}}},
				},
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

	doc, err := gen.Generate(routes)
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
	routes := []route.Route{
		&mockRoute{method: "GET", path: "/test", operationID: "test"},
	}

	doc, _ := gen.Generate(routes)

	if doc.Components != nil {
		t.Error("expected nil components when no schemas or security schemes")
	}
}

func TestGenerator_Generate_OnlySchemasNoSecurity(t *testing.T) {
	cfg := openapi.Config{Title: "T", Version: "1"}
	gen := openapi.NewGenerator(cfg, nil)

	routes := []route.Route{
		&mockValidatedRoute{
			mockRoute: mockRoute{method: "POST", path: "/users", operationID: "createUser"},
			reqBody:   &testBodySchema{kind: 0, types: []interface{}{ReqBody{}}},
		},
	}

	doc, _ := gen.Generate(routes)

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

	routes := []route.Route{
		&mockRoute{method: "GET", path: "/test", operationID: "test"},
	}

	doc, _ := gen.Generate(routes)

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
	routes := []route.Route{
		&mockRoute{method: "get", path: "/test", operationID: "test"},
	}

	doc, _ := gen.Generate(routes)

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

	routes := []route.Route{
		&mockSecuredRoute{
			mockRoute: mockRoute{method: "GET", path: "/secure", operationID: "secureOp"},
			schemes:   []string{"bearer", "basic"},
		},
	}

	doc, _ := gen.Generate(routes)

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

	routes := []route.Route{
		&mockValidatedRoute{
			mockRoute: mockRoute{method: "POST", path: "/items", operationID: "createItem"},
			reqBody:   &testBodySchema{kind: 0, types: []interface{}{}}, // empty types
		},
	}

	doc, _ := gen.Generate(routes)

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

	routes := []route.Route{
		&mockRoute{method: "GET", path: "/users/{id}", operationID: "getUser"},
	}

	doc, err := gen.Generate(routes)
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

	routes := []route.Route{
		&mockRoute{method: "GET", path: "/users/{userID}/posts/{postID}", operationID: "getPost"},
	}

	doc, err := gen.Generate(routes)
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

	routes := []route.Route{
		&mockRoute{method: "GET", path: "/health", operationID: "health"},
	}

	doc, err := gen.Generate(routes)
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
