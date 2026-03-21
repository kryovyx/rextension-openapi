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

	"github.com/kryovyx/rex/route"
)

// newSpecRoute creates a route that serves the OpenAPI JSON document.
func newSpecRoute(path string, docBytes []byte) route.Route {
	return route.New("GET", path, func(ctx route.Context) {
		ctx.Respond(http.StatusOK, "application/json", docBytes)
	})
}

// newSpecRouteWithGenerator creates a route that lazily generates and serves the OpenAPI JSON document.
// The document is generated on first request to ensure all routes are collected.
func newSpecRouteWithGenerator(path string, ext *OpenAPIExtension) route.Route {
	return route.New("GET", path, func(ctx route.Context) {
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
