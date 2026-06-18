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
// These are re-declared locally so the openapi module does not import
// rextension-validation or rextension-security directly.
// Routes that implement both OpenAPIRoute and these interfaces get richer
// documentation automatically.

// SchemaKind mirrors validation.SchemaKind for local use.
type SchemaKind int

const (
	SchemaScalar SchemaKind = iota
	SchemaOneOf
	SchemaAnyOf
	SchemaAllOf
)

// BodySchemaAccessor mirrors validation.BodySchema.
type BodySchemaAccessor interface {
	Kind() SchemaKind
	Types() []interface{}
}

// ValidatableRouteAccessor is intentionally not defined as a Go interface.
// Instead, the generator uses reflection to call RequestBody()/Responses()
// on routes and extract Kind()/Types() from the returned body schemas.
// This avoids requiring routes to implement a second interface just for OpenAPI;
// any route implementing validation.ValidatableRoute works automatically.

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
type RoleClaimProvider interface {
	RolesClaim() string
}

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
