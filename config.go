// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: © 2026 Kryovyx

// Package openapi provides a Rex extension that generates an OpenAPI 3.1
// specification document from routes implementing the OpenAPIRoute interface.
//
// This file defines the extension configuration and functional options.
package openapi

// Config controls the OpenAPI extension behavior.
type Config struct {
	// Title is the API title in the info section.
	Title string
	// Version is the API version in the info section.
	Version string
	// Description is the API description in the info section.
	Description string
	// ServePath is the path to serve the OpenAPI JSON document. Default: "/openapi.json".
	ServePath string
	// Tags defines top-level tag descriptions for the document.
	Tags []Tag
}

// NewDefaultConfig returns the default configuration.
func NewDefaultConfig() *Config {
	return &Config{
		Title:     "API",
		Version:   "1.0.0",
		ServePath: "/openapi.json",
	}
}

// ConfigOption allows functional configuration.
type ConfigOption func(*Config)

// WithTitle sets the API title.
func WithTitle(title string) ConfigOption {
	return func(cfg *Config) {
		cfg.Title = title
	}
}

// WithVersion sets the API version.
func WithVersion(version string) ConfigOption {
	return func(cfg *Config) {
		cfg.Version = version
	}
}

// WithDescription sets the API description.
func WithDescription(desc string) ConfigOption {
	return func(cfg *Config) {
		cfg.Description = desc
	}
}

// WithServePath sets the path to serve the OpenAPI document.
func WithServePath(path string) ConfigOption {
	return func(cfg *Config) {
		cfg.ServePath = path
	}
}

// WithTags registers top-level tag definitions that describe the tags
// used by route operations. Tags are merged with any previously registered
// tags; duplicate names are overwritten by the last entry.
func WithTags(tags ...Tag) ConfigOption {
	return func(cfg *Config) {
		cfg.Tags = append(cfg.Tags, tags...)
	}
}

// NewConfig creates a config with the given options applied on top of defaults.
func NewConfig(opts ...ConfigOption) *Config {
	c := NewDefaultConfig()
	for _, opt := range opts {
		opt(c)
	}
	return c
}
