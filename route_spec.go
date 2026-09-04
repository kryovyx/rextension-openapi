// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: © 2026 Kryovyx

// Package openapi provides a Rex extension that generates an OpenAPI 3.1
// specification document from routes implementing the OpenAPIRoute interface.
//
// This file defines the served route that delivers the pre-built OpenAPI
// document as a JSON response.
package openapi

import (
	"net/http"
	"sync/atomic"

	rx "github.com/kryovyx/rextension"
	rxroute "github.com/kryovyx/rextension/route"
)

// specRoute serves a document built at startup.
//
// The bytes are published by atomic swap, so the request path reads them
// without a lock. Nothing rebuilds them: the document is generated once, when
// the route table is frozen (P4.14).
//
// # What this replaces
//
// Generation used to be lazy — built on the first request, cached after — and
// the whole arrangement is gone rather than fixed:
//
//   - The cache check was `lock; if generatedOnce { return }; unlock; …build…;
//     lock; store; unlock`. Two concurrent first requests both saw
//     generatedOnce false and both built the document, and the second
//     overwrote the first.
//   - Building read e.securitySchemes and wrote it, unsynchronised, from
//     whichever request arrived first.
//   - A generation failure produced a 500 on every request to the spec path,
//     with the cause discoverable only by making one.
//
// Laziness existed because "all routes are registered" was not a moment the
// framework offered. Freezing the route table makes it one (D40), so the
// document can be built at startup and a failure to build it can be a startup
// error.
type specRoute struct {
	path string
	doc  atomic.Pointer[[]byte]
}

// newSpecRoute creates a route that serves the document once it is set.
func newSpecRoute(path string) *specRoute {
	if path == "" {
		path = "/openapi.json"
	}
	return &specRoute{path: path}
}

// setDocument publishes the generated document.
func (s *specRoute) setDocument(doc []byte) {
	s.doc.Store(&doc)
}

func (s *specRoute) Method() string { return http.MethodGet }
func (s *specRoute) Path() string   { return s.path }

func (s *specRoute) Handler() rxroute.HandlerFunc {
	return func(ctx rxroute.Context) {
		doc := s.doc.Load()
		if doc == nil {
			// Only reachable if the route is served before the document is
			// generated, which the startup ordering prevents. A problem
			// document rather than a half-built spec: a truncated OpenAPI
			// document is worse than none, because tooling will act on it.
			rx.WriteProblem(ctx.ResponseWriter(), ctx.Request(),
				http.StatusServiceUnavailable, rx.ProblemInternal,
				"the OpenAPI document is not available")
			return
		}
		_ = ctx.Respond(http.StatusOK, "application/json", *doc)
	}
}

var _ rxroute.Route = (*specRoute)(nil)
