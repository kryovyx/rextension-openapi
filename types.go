// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: © 2026 Kryovyx

// Package openapi provides a Rex extension that generates an OpenAPI 3.1
// specification document from routes implementing the OpenAPIRoute interface.
//
// This file defines the OpenAPI 3.1 document struct types as plain Go structs.
// No external OpenAPI library is required — the structs serialize to JSON
// that conforms to the OpenAPI 3.1 specification.
package openapi

// Tag adds metadata to a single tag used by operations.
type Tag struct {
	Name         string        `json:"name"`
	Description  string        `json:"description,omitempty"`
	ExternalDocs *ExternalDocs `json:"externalDocs,omitempty"`
}

// ExternalDocs points to additional external documentation.
type ExternalDocs struct {
	Description string `json:"description,omitempty"`
	URL         string `json:"url"`
}

// Document represents a complete OpenAPI 3.1 document.
type Document struct {
	OpenAPI    string               `json:"openapi"`
	Info       Info                 `json:"info"`
	Tags       []Tag                `json:"tags,omitempty"`
	Paths      map[string]*PathItem `json:"paths,omitempty"`
	Components *Components          `json:"components,omitempty"`
}

// Info provides metadata about the API.
type Info struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

// PathItem describes the operations available on a single path.
type PathItem struct {
	Get     *Operation `json:"get,omitempty"`
	Post    *Operation `json:"post,omitempty"`
	Put     *Operation `json:"put,omitempty"`
	Patch   *Operation `json:"patch,omitempty"`
	Delete  *Operation `json:"delete,omitempty"`
	Head    *Operation `json:"head,omitempty"`
	Options *Operation `json:"options,omitempty"`
}

// setOperation sets the operation on the PathItem for the given method.
func (pi *PathItem) setOperation(method string, op *Operation) {
	switch method {
	case "GET":
		pi.Get = op
	case "POST":
		pi.Post = op
	case "PUT":
		pi.Put = op
	case "PATCH":
		pi.Patch = op
	case "DELETE":
		pi.Delete = op
	case "HEAD":
		pi.Head = op
	case "OPTIONS":
		pi.Options = op
	}
}

// Operation describes a single API operation on a path.
type Operation struct {
	OperationID string                `json:"operationId,omitempty"`
	Summary     string                `json:"summary,omitempty"`
	Description string                `json:"description,omitempty"`
	Tags        []string              `json:"tags,omitempty"`
	RequestBody *RequestBody          `json:"requestBody,omitempty"`
	Responses   map[string]*Response  `json:"responses,omitempty"`
	Security    []SecurityRequirement `json:"security,omitempty"`
}

// RequestBody describes the request body for an operation.
type RequestBody struct {
	Description string               `json:"description,omitempty"`
	Required    bool                 `json:"required"`
	Content     map[string]MediaType `json:"content"`
}

// Response describes a single response from an API operation.
type Response struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
}

// ExampleObject represents a named example in OpenAPI 3.1.
type ExampleObject struct {
	Summary     string      `json:"summary,omitempty"`
	Description string      `json:"description,omitempty"`
	Value       interface{} `json:"value,omitempty"`
}

// MediaType describes a media type with a schema and optional examples.
type MediaType struct {
	Schema   *SchemaObject             `json:"schema"`
	Example  interface{}               `json:"example,omitempty"`
	Examples map[string]*ExampleObject `json:"examples,omitempty"`
}

// SecurityRequirement maps scheme name → scopes.
type SecurityRequirement map[string][]string

// Components holds reusable objects.
type Components struct {
	Schemas         map[string]*SchemaObject         `json:"schemas,omitempty"`
	SecuritySchemes map[string]*SecuritySchemeObject `json:"securitySchemes,omitempty"`
}

// SecuritySchemeObject describes a security scheme in components.
type SecuritySchemeObject struct {
	Type         string `json:"type"`
	Description  string `json:"description,omitempty"`
	Scheme       string `json:"scheme,omitempty"`
	BearerFormat string `json:"bearerFormat,omitempty"`
	Name         string `json:"name,omitempty"`
	In           string `json:"in,omitempty"`
}

// SchemaObject represents a JSON Schema 2020-12 object (the dialect used by OpenAPI 3.1).
type SchemaObject struct {
	Type        string                   `json:"type,omitempty"`
	Format      string                   `json:"format,omitempty"`
	Description string                   `json:"description,omitempty"`
	Properties  map[string]*SchemaObject `json:"properties,omitempty"`
	Required    []string                 `json:"required,omitempty"`
	Items       *SchemaObject            `json:"items,omitempty"`
	Enum        []interface{}            `json:"enum,omitempty"`
	Minimum     *float64                 `json:"minimum,omitempty"`
	Maximum     *float64                 `json:"maximum,omitempty"`
	MinLength   *int                     `json:"minLength,omitempty"`
	MaxLength   *int                     `json:"maxLength,omitempty"`
	Ref         string                   `json:"$ref,omitempty"`
	OneOf       []*SchemaObject          `json:"oneOf,omitempty"`
	AnyOf       []*SchemaObject          `json:"anyOf,omitempty"`
	AllOf       []*SchemaObject          `json:"allOf,omitempty"`
	Nullable    bool                     `json:"nullable,omitempty"`
	Example     interface{}              `json:"example,omitempty"`
}
