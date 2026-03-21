package openapi_test

import (
	"testing"

	openapi "github.com/kryovyx/rextension-openapi"
)

func TestNewDefaultConfig(t *testing.T) {
	cfg := openapi.NewDefaultConfig()

	if cfg.Title != "API" {
		t.Errorf("expected Title %q, got %q", "API", cfg.Title)
	}
	if cfg.Version != "1.0.0" {
		t.Errorf("expected Version %q, got %q", "1.0.0", cfg.Version)
	}
	if cfg.Description != "" {
		t.Errorf("expected empty Description, got %q", cfg.Description)
	}
	if cfg.ServePath != "/openapi.json" {
		t.Errorf("expected ServePath %q, got %q", "/openapi.json", cfg.ServePath)
	}
}

func TestWithTitle(t *testing.T) {
	cfg := openapi.NewConfig(openapi.WithTitle("My API"))

	if cfg.Title != "My API" {
		t.Errorf("expected Title %q, got %q", "My API", cfg.Title)
	}
	// Other defaults remain
	if cfg.Version != "1.0.0" {
		t.Errorf("expected default Version %q, got %q", "1.0.0", cfg.Version)
	}
}

func TestWithVersion(t *testing.T) {
	cfg := openapi.NewConfig(openapi.WithVersion("2.0.0"))

	if cfg.Version != "2.0.0" {
		t.Errorf("expected Version %q, got %q", "2.0.0", cfg.Version)
	}
	if cfg.Title != "API" {
		t.Errorf("expected default Title %q, got %q", "API", cfg.Title)
	}
}

func TestWithDescription(t *testing.T) {
	cfg := openapi.NewConfig(openapi.WithDescription("A test API"))

	if cfg.Description != "A test API" {
		t.Errorf("expected Description %q, got %q", "A test API", cfg.Description)
	}
}

func TestWithServePath(t *testing.T) {
	cfg := openapi.NewConfig(openapi.WithServePath("/docs/openapi.json"))

	if cfg.ServePath != "/docs/openapi.json" {
		t.Errorf("expected ServePath %q, got %q", "/docs/openapi.json", cfg.ServePath)
	}
}

func TestNewConfig_NoOptions(t *testing.T) {
	cfg := openapi.NewConfig()

	if cfg.Title != "API" {
		t.Errorf("expected default Title %q, got %q", "API", cfg.Title)
	}
	if cfg.Version != "1.0.0" {
		t.Errorf("expected default Version %q, got %q", "1.0.0", cfg.Version)
	}
	if cfg.ServePath != "/openapi.json" {
		t.Errorf("expected default ServePath %q, got %q", "/openapi.json", cfg.ServePath)
	}
}

func TestNewConfig_MultipleOptions(t *testing.T) {
	cfg := openapi.NewConfig(
		openapi.WithTitle("Multi"),
		openapi.WithVersion("3.0.0"),
		openapi.WithDescription("Multi test"),
		openapi.WithServePath("/api/spec"),
	)

	if cfg.Title != "Multi" {
		t.Errorf("expected Title %q, got %q", "Multi", cfg.Title)
	}
	if cfg.Version != "3.0.0" {
		t.Errorf("expected Version %q, got %q", "3.0.0", cfg.Version)
	}
	if cfg.Description != "Multi test" {
		t.Errorf("expected Description %q, got %q", "Multi test", cfg.Description)
	}
	if cfg.ServePath != "/api/spec" {
		t.Errorf("expected ServePath %q, got %q", "/api/spec", cfg.ServePath)
	}
}

func TestNewConfig_OptionOverridesOption(t *testing.T) {
	cfg := openapi.NewConfig(
		openapi.WithTitle("First"),
		openapi.WithTitle("Second"),
	)

	if cfg.Title != "Second" {
		t.Errorf("expected last option to win, got Title %q", cfg.Title)
	}
}
