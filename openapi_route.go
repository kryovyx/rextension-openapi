// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: © 2026 Kryovyx

package openapi

import rx "github.com/kryovyx/rextension"

// OpenAPIRoute is an optional interface that a route.Route may implement
// to be included in the generated OpenAPI document. Routes that do not
// implement this interface are excluded from the spec.
type OpenAPIRoute interface {
	// OperationID returns a unique identifier for the operation.
	OperationID() string
	// Summary returns a short summary for the operation.
	Summary() string
	// Description returns a longer description for the operation.
	Description() string
	// Tags returns the tags to categorize the operation.
	Tags() []string
}

// --- Soft-dependency interfaces ---
// These are aliases of the contracts declared in rextension, so the openapi
// module documents routes and schemes without importing
// rextension-validation or rextension-security. Routes that implement
// OpenAPIRoute alongside these get richer documentation automatically.

// SchemaKind describes the composition strategy for a body schema.
// Aliased from rextension.
type SchemaKind = rx.SchemaKind

// Schema composition strategies. Aliased from rextension.
const (
	// SchemaScalar indicates a single concrete type.
	SchemaScalar = rx.SchemaScalar
	// SchemaOneOf indicates exactly one of the listed types must match.
	SchemaOneOf = rx.SchemaOneOf
	// SchemaAnyOf indicates one or more of the listed types may match.
	SchemaAnyOf = rx.SchemaAnyOf
	// SchemaAllOf indicates all of the listed types must match (merged).
	SchemaAllOf = rx.SchemaAllOf
)

// BodySchemaAccessor describes the shape of a request or response body.
// Aliased from rextension.
type BodySchemaAccessor = rx.BodySchema

// ValidatableRouteAccessor is implemented by a route that declares its request
// and response body schemas.
//
// This used to be deliberately *not* an interface: the generator called
// RequestBody() and Responses() through reflect.MethodByName, on the reasoning
// that "any route implementing validation.ValidatableRoute works
// automatically". Reflection was really a workaround for the two extensions
// having no shared type — and it came with an unchecked
// `.([]interface{})` assertion that panicked inside document generation on any
// unexpected slice type.
//
// The contract now lives in rextension, so both extensions assert the same
// named interface and the reflection is gone (D20).
type ValidatableRouteAccessor = rx.BodySchemaProvider

// SecuredRouteAccessor mirrors security.SecuredRoute.
// If a route implements this alongside OpenAPIRoute, the generator
// auto-populates security requirements.
type SecuredRouteAccessor = rx.SecuredRouteAccessor

// ScopedSecuredRouteAccessor is an optional extension of SecuredRouteAccessor.
// Routes implementing this declare OAuth2/OIDC scopes required per scheme.
// The generator uses the returned map to populate the scopes list in each
// OpenAPI security requirement entry (instead of emitting an empty slice).
//
// The map key must match a scheme name returned by RequiredSchemes.
type ScopedSecuredRouteAccessor interface {
	RequiredScopes() map[string][]string
}

// RoleGuardedRouteAccessor is an optional extension of SecuredRouteAccessor.
// Routes implementing this declare roles required per scheme.
// The generator uses the returned map to populate the x-required-roles extension
// on the OpenAPI operation (one entry per scheme → list of required roles).
//
// The map key must match a scheme name returned by RequiredSchemes.
type RoleGuardedRouteAccessor interface {
	RequiredRoles() map[string][]string
}

// RoleClaimProvider is an optional interface a SecurityScheme may implement to
// expose which token claim carries the role/permission values. When implemented,
// the generator emits x-roles-claim on the security scheme object in the
// components section, enabling UI tooling (e.g. the Swagger UI plugin) to
// surface the claim name alongside the required-roles badges.
//
// Aliased from rextension.
type RoleClaimProvider = rx.RoleClaimProvider

// ParameterizedScheme is implemented by a scheme carried in a named request
// parameter — an API key in a header, query string or cookie.
//
// Aliased from rextension. Reading the location through this interface is what
// replaced reflect.MethodByName("Location") in the generator (D20).
type ParameterizedScheme = rx.ParameterizedScheme

// BearerFormatProvider is implemented by a bearer scheme that documents its
// token format, e.g. "JWT". Aliased from rextension.
type BearerFormatProvider = rx.BearerFormatProvider

// SecuritySchemeAccessor mirrors security.SecurityScheme for doc generation.
type SecuritySchemeAccessor = rx.SecuritySchemeAccessor

// ResponseExamplesProvider is an optional interface a route may implement to
// supply named examples for specific HTTP status code responses.
// The outer map key is the status code; the inner map key is the example name.
//
// Example:
//
//	func (r *myRoute) ResponseExamples() map[int]map[string]ExampleObject {
//		return map[int]map[string]ExampleObject{
//			200: {
//				"success": {Summary: "Successful payment", Value: PaymentResponse{Status: "success"}},
//			},
//		}
//	}
type ResponseExamplesProvider interface {
	ResponseExamples() map[int]map[string]ExampleObject
}

// RequestBodyExamplesProvider is an optional interface a route may implement
// to supply named examples for the request body.
// The map key is the example name.
//
// Example:
//
//	func (r *myRoute) RequestBodyExamples() map[string]ExampleObject {
//		return map[string]ExampleObject{
//			"usd-payment": {Summary: "USD payment", Value: PaymentRequest{Amount: 1000, Currency: "USD"}},
//		}
//	}
type RequestBodyExamplesProvider interface {
	RequestBodyExamples() map[string]ExampleObject
}

// ResponseDescriptionProvider is an optional interface a route may implement
// to supply human-readable descriptions for each HTTP status code response entry.
// When implemented, the returned description replaces the default "Response for status N".
//
// Example:
//
//	func (r *myRoute) ResponseDescriptions() map[int]string {
//		return map[int]string{
//			200: "Payment processed successfully",
//			400: "Invalid request payload",
//			401: "Missing or invalid authentication",
//		}
//	}
type ResponseDescriptionProvider interface {
	ResponseDescriptions() map[int]string
}
