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

	rxroute "github.com/kryovyx/rextension/route"
)

// newSpecRoute creates a route that serves the OpenAPI JSON document.
func newSpecRoute(path string, docBytes []byte) rxroute.Route {
	return rxroute.New("GET", path, func(ctx rxroute.Context) {
		ctx.Respond(http.StatusOK, "application/json", docBytes)
	})
}

// newSpecRouteWithGenerator creates a route that lazily generates and serves the OpenAPI JSON document.
// The document is generated on first request to ensure all routes are collected.
func newSpecRouteWithGenerator(path string, ext *OpenAPIExtension) rxroute.Route {
	return rxroute.New("GET", path, func(ctx rxroute.Context) {
		ext.logger.Debug("OpenAPI: Spec route handler called, generating document on demand")
		docBytes, err := ext.ensureGenerated()
		if err != nil {
			ext.logger.Error("Failed to generate OpenAPI document on request: %v", err)
			ctx.Respond(http.StatusInternalServerError, "application/json",
				map[string]string{"error": "Failed to generate OpenAPI document"})
			return
		}
		ext.logger.Debug("OpenAPI: Responding with %d bytes", len(docBytes))
		ctx.Respond(http.StatusOK, "application/json", docBytes)
	})
}
