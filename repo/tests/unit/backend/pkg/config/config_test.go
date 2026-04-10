package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"wlpr-portal/pkg/config"
)

func TestParse_ValidConfigJS(t *testing.T) {
	input := []byte(`
// Runtime configuration
window.__WLPR_CONFIG__ = {
  "API_BASE_URL": "/api",
  "APP_VERSION": "1.0.0",
  "DATABASE_URL": "postgres://wlpr:wlpr_secret@db:5432/wlpr_portal?sslmode=disable",
  "JWT_SECRET": "test-secret-at-least-32-characters-long",
  "AES_ENCRYPTION_KEY": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
  "PORT": "8080",
  "POSTGRES_DB": "wlpr_portal",
  "POSTGRES_USER": "wlpr",
  "POSTGRES_PASSWORD": "wlpr_secret"
};
`)

	cfg, err := config.Parse(input)
	if err != nil {
		t.Fatalf("config.Parse failed: %v", err)
	}

	if cfg.DatabaseURL != "postgres://wlpr:wlpr_secret@db:5432/wlpr_portal?sslmode=disable" {
		t.Errorf("DATABASE_URL: got %q", cfg.DatabaseURL)
	}
	if cfg.JWTSecret != "test-secret-at-least-32-characters-long" {
		t.Errorf("JWT_SECRET: got %q", cfg.JWTSecret)
	}
	if cfg.AESEncryptionKey != "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" {
		t.Errorf("AES_ENCRYPTION_KEY: got %q", cfg.AESEncryptionKey)
	}
	if cfg.Port != "8080" {
		t.Errorf("PORT: got %q", cfg.Port)
	}
	if cfg.PostgresDB != "wlpr_portal" {
		t.Errorf("POSTGRES_DB: got %q", cfg.PostgresDB)
	}
	if cfg.PostgresUser != "wlpr" {
		t.Errorf("POSTGRES_USER: got %q", cfg.PostgresUser)
	}
	if cfg.PostgresPassword != "wlpr_secret" {
		t.Errorf("POSTGRES_PASSWORD: got %q", cfg.PostgresPassword)
	}
	if cfg.APIBaseURL != "/api" {
		t.Errorf("API_BASE_URL: got %q", cfg.APIBaseURL)
	}
}

func TestParse_WithComments(t *testing.T) {
	input := []byte(`
// This is a comment
window.__WLPR_CONFIG__ = {
  // Backend settings
  "DATABASE_URL": "postgres://localhost/test",
  "JWT_SECRET": "my-secret-key-at-least-32-characters",
  "AES_ENCRYPTION_KEY": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "PORT": "9090"
};
`)

	cfg, err := config.Parse(input)
	if err != nil {
		t.Fatalf("parse with comments failed: %v", err)
	}

	if cfg.DatabaseURL != "postgres://localhost/test" {
		t.Errorf("DATABASE_URL: got %q", cfg.DatabaseURL)
	}
	if cfg.Port != "9090" {
		t.Errorf("PORT: got %q", cfg.Port)
	}
}

func TestParse_WithTrailingCommas(t *testing.T) {
	input := []byte(`
window.__WLPR_CONFIG__ = {
  "DATABASE_URL": "postgres://localhost/test",
  "JWT_SECRET": "my-secret-key-at-least-32-characters",
  "AES_ENCRYPTION_KEY": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "PORT": "8080",
};
`)

	cfg, err := config.Parse(input)
	if err != nil {
		t.Fatalf("parse with trailing commas failed: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("PORT: got %q", cfg.Port)
	}
}

func TestParse_InvalidContent(t *testing.T) {
	_, err := config.Parse([]byte(`this is not a config file`))
	if err == nil {
		t.Fatal("expected error for invalid content")
	}
}

func TestParse_EmptyObject(t *testing.T) {
	input := []byte(`window.__WLPR_CONFIG__ = {};`)
	cfg, err := config.Parse(input)
	if err != nil {
		t.Fatalf("parse empty object failed: %v", err)
	}
	if cfg.DatabaseURL != "" {
		t.Error("expected empty DATABASE_URL")
	}
}

func TestLoadFrom_FileExists(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.js")
	os.WriteFile(path, []byte(`
window.__WLPR_CONFIG__ = {
  "DATABASE_URL": "postgres://test",
  "JWT_SECRET": "test-secret-at-least-32-characters-long",
  "AES_ENCRYPTION_KEY": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
  "PORT": "3000"
};
`), 0644)

	cfg, err := config.LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}
	if cfg.Port != "3000" {
		t.Errorf("PORT: got %q", cfg.Port)
	}
}

func TestLoadFrom_FileNotFound(t *testing.T) {
	_, err := config.LoadFrom("/nonexistent/path/config.js")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestParse_NestedObjectsIgnored(t *testing.T) {
	input := []byte(`
window.__WLPR_CONFIG__ = {
  "DATABASE_URL": "postgres://localhost/test",
  "JWT_SECRET": "my-secret-key-at-least-32-characters",
  "AES_ENCRYPTION_KEY": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "PORT": "8080",
  "FEATURE_DEFAULTS": {
    "pinyin_search": true,
    "synonym_search": true
  }
};
`)

	cfg, err := config.Parse(input)
	if err != nil {
		t.Fatalf("parse with nested objects failed: %v", err)
	}

	if cfg.DatabaseURL != "postgres://localhost/test" {
		t.Errorf("DATABASE_URL: got %q", cfg.DatabaseURL)
	}
}
