# Rex OpenAPI Extension (rextension-openapi)

A comprehensive OpenAPI 3.1 specification generator extension for the Rex framework.

[![Go Version](https://img.shields.io/badge/go-1.26+-blue.svg)](https://golang.org/dl/)
[![Coverage](https://img.shields.io/badge/coverage-67.9%25-yellowgreen.svg)](#)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## Overview

`rextension-openapi` is a Rex extension that provides:

- **Automatic OpenAPI 3.1 document generation** from registered routes
- **Rich JSON Schema generation** via reflection on Go types
- **Struct tag support**: `json`, `validate` (required, min, max, etc.)
- **Union type support**: OneOf/AnyOf/AllOf via `validation.BodySchema`
- **Per-status-code response documentation** via `ValidatableRoute.Responses()`
- **Security scheme documentation** (soft-dependency on `rextension-security`)
- **Request/response examples** via `ResponseExamplesProvider` and `RequestBodyExamplesProvider`
- **Response descriptions** via `ResponseDescriptionProvider`
- **Lazy generation**: Document generated on first request and cached in memory
- **Configurable serve path** (default: `/openapi.json`)

## Installation

```bash
go get github.com/kryovyx/rextension-openapi
```

## Quick Start

```go
package main

import (
    "github.com/kryovyx/rex"
    "github.com/kryovyx/rex/route"
    openapi "github.com/kryovyx/rextension-openapi"
)

func main() {
    app := rex.New()

    // Add OpenAPI extension with default config
    app.WithOptions(
        openapi.WithOpenAPI(nil),
    )

    // Register your routes (implementing OpenAPIRoute)
    app.RegisterRoute(&HelloRoute{})

    // Run the application
    // OpenAPI document available at /openapi.json
    if err := app.Run(); err != nil {
        panic(err)
    }
}
```

## OpenAPIRoute Interface

Routes must implement the `OpenAPIRoute` interface to be included in the generated OpenAPI document. Routes that do not implement this interface are excluded from the spec.

```go
type OpenAPIRoute interface {
    OperationID() string
    Summary() string
    Description() string
    Tags() []string
}
```

A route must also implement `route.Route` (`Method()`, `Path()`, `Handler()`). Combining both interfaces provides the metadata needed for the OpenAPI spec:

```go
type HelloRoute struct{}

func (r *HelloRoute) Method() string      { return "GET" }
func (r *HelloRoute) Path() string        { return "/hello" }
func (r *HelloRoute) Handler() route.HandlerFunc {
    return func(ctx route.Context) {
        ctx.JSON(200, map[string]string{"message": "Hello, World!"})
    }
}

// OpenAPIRoute implementation
func (r *HelloRoute) OperationID() string  { return "getHello" }
func (r *HelloRoute) Summary() string      { return "Say hello" }
func (r *HelloRoute) Description() string  { return "Returns a greeting message" }
func (r *HelloRoute) Tags() []string       { return []string{"greetings"} }
```

## Request/Response Schemas

If a route also implements `ValidatableRoute` (from `rextension-validation`), request and response schemas are automatically generated from the Go types via reflection.

### Request Body Schema

Struct tags drive schema generation:

```go
type CreateUserRequest struct {
    Name  string `json:"name"  validate:"required,min=1,max=100"`
    Email string `json:"email" validate:"required"`
    Age   int    `json:"age"   validate:"min=0,max=150"`
}
```

This produces an OpenAPI schema with `required` fields and `minLength`/`maxLength`/`minimum`/`maximum` constraints.

### Union Types

Use `validation.BodySchema` to declare union types:

- **OneOf**: Exactly one schema matches
- **AnyOf**: One or more schemas match
- **AllOf**: All schemas must match

### Per-Status-Code Responses

Routes implementing `ValidatableRoute` can return `Responses()` to document each status code with its own schema type:

```go
func (r *MyRoute) Responses() map[int]interface{} {
    return map[int]interface{}{
        200: SuccessResponse{},
        400: ErrorResponse{},
        401: AuthErrorResponse{},
    }
}
```

## Security Integration

Routes that implement both `OpenAPIRoute` and the `SecuredRoute` interface (from `rextension-security`) automatically have security requirements added to their operations in the spec.

```go
func (r *ProtectedRoute) RequiredSchemes() []string {
    return []string{"bearer"}
}
```

Security schemes registered via `rextension-security` are discovered through the DI container and the `rextension` global registry, and appear in the `components/securitySchemes` section of the document.

## Examples and Descriptions

### Response Examples

Implement `ResponseExamplesProvider` to supply named examples per status code:

```go
func (r *MyRoute) ResponseExamples() map[int]map[string]openapi.ExampleObject {
    return map[int]map[string]openapi.ExampleObject{
        200: {
            "success": {Summary: "Successful payment", Value: PaymentResponse{Status: "success"}},
        },
    }
}
```

### Request Body Examples

Implement `RequestBodyExamplesProvider` to supply named examples for the request body:

```go
func (r *MyRoute) RequestBodyExamples() map[string]openapi.ExampleObject {
    return map[string]openapi.ExampleObject{
        "usd-payment": {Summary: "USD payment", Value: PaymentRequest{Amount: 1000, Currency: "USD"}},
    }
}
```

### Response Descriptions

Implement `ResponseDescriptionProvider` to supply human-readable descriptions per status code:

```go
func (r *MyRoute) ResponseDescriptions() map[int]string {
    return map[int]string{
        200: "Payment processed successfully",
        400: "Invalid request payload",
        401: "Missing or invalid authentication",
    }
}
```

## Configuration Reference

| Field         | Type     | Default           | Description                                  |
|---------------|----------|-------------------|----------------------------------------------|
| `Title`       | `string` | `"API"`           | API title in the info section                |
| `Version`     | `string` | `"1.0.0"`         | API version in the info section              |
| `Description` | `string` | `""`              | API description in the info section          |
| `ServePath`   | `string` | `"/openapi.json"` | Path to serve the OpenAPI JSON document      |

### Config Options

```go
openapi.WithOpenAPI(openapi.NewConfig(
    openapi.WithTitle("My API"),
    openapi.WithVersion("2.0.0"),
    openapi.WithDescription("My awesome API"),
    openapi.WithServePath("/docs/openapi.json"),
))
```

| Option              | Description                              |
|---------------------|------------------------------------------|
| `WithTitle(s)`      | Sets the API title                       |
| `WithVersion(s)`    | Sets the API version                     |
| `WithDescription(s)`| Sets the API description                 |
| `WithServePath(s)`  | Sets the path to serve the OpenAPI JSON  |

## Best Practices

1. **Implement OpenAPIRoute on all public routes**: This ensures complete API documentation is generated automatically
2. **Use meaningful operation IDs**: Follow a consistent naming convention like `getUser`, `createOrder` for clear client SDK generation
3. **Leverage struct tags**: Use `json` and `validate` tags on request/response types for accurate schema generation
4. **Provide response examples**: Implement `ResponseExamplesProvider` to give consumers concrete examples of your API responses
5. **Add response descriptions**: Use `ResponseDescriptionProvider` to document what each status code means in your domain
6. **Declare per-status-code responses**: Return all possible response types from `Responses()` for complete documentation
7. **Use tags to organize operations**: Group related endpoints under the same tags for better navigation
8. **Customize the serve path**: In production, consider serving the spec at a well-known path like `/openapi.json` or `/docs/openapi.json`

## Contributing

**At this time, this project is in active development and is not open for external contributions.** The framework is still being refined and major interfaces may change.

Once the framework reaches a stable architecture and API, contributions from the community will be welcome. Please check back later or open an issue if you have feature requests or feedback.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Copyright

© 2026 Kryovyx
