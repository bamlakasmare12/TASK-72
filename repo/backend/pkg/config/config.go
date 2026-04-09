package config

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// AppConfig holds all configuration values parsed from config.js.
type AppConfig struct {
	// Backend
	DatabaseURL      string `json:"DATABASE_URL"`
	JWTSecret        string `json:"JWT_SECRET"`
	AESEncryptionKey string `json:"AES_ENCRYPTION_KEY"`
	Port             string `json:"PORT"`

	// Database (for reference; consumed by PostgreSQL container directly)
	PostgresDB       string `json:"POSTGRES_DB"`
	PostgresUser     string `json:"POSTGRES_USER"`
	PostgresPassword string `json:"POSTGRES_PASSWORD"`

	// Frontend (for reference; consumed by browser via <script> tag)
	APIBaseURL string `json:"API_BASE_URL"`
	AppVersion string `json:"APP_VERSION"`
}

// configPaths lists the locations to search for config.js, in priority order.
var configPaths = []string{
	"../config/config.js",     // running from backend/ directory
	"./config/config.js",      // running from project root
	"/config/config.js",       // Docker mount path
	"config/config.js",        // relative to CWD
}

// Load reads and parses config.js from the first path that exists.
// The file format is: window.__WLPR_CONFIG__ = { ... };
// We extract the JSON object and unmarshal it.
func Load() (*AppConfig, error) {
	return LoadFrom("")
}

// LoadFrom reads config.js from a specific path, or searches default paths if empty.
func LoadFrom(path string) (*AppConfig, error) {
	var data []byte
	var err error

	if path != "" {
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
		}
	} else {
		for _, p := range configPaths {
			data, err = os.ReadFile(p)
			if err == nil {
				break
			}
		}
		if data == nil {
			return nil, fmt.Errorf("config.js not found in any of: %s", strings.Join(configPaths, ", "))
		}
	}

	return parse(data)
}

// parse extracts the JSON object from the config.js content.
func parse(data []byte) (*AppConfig, error) {
	content := string(data)

	// Extract the object between the first { and the last }
	re := regexp.MustCompile(`(?s)window\.__WLPR_CONFIG__\s*=\s*(\{.*\})`)
	matches := re.FindStringSubmatch(content)
	if len(matches) < 2 {
		return nil, fmt.Errorf("config.js does not contain a valid window.__WLPR_CONFIG__ = {...} assignment")
	}

	jsonStr := matches[1]

	// Remove trailing semicolon if present
	jsonStr = strings.TrimRight(strings.TrimSpace(jsonStr), ";")

	// Remove single-line JS comments (// ...)
	commentRe := regexp.MustCompile(`//[^\n]*`)
	jsonStr = commentRe.ReplaceAllString(jsonStr, "")

	// Remove trailing commas before } or ] (invalid JSON but valid JS)
	trailingCommaRe := regexp.MustCompile(`,\s*([}\]])`)
	jsonStr = trailingCommaRe.ReplaceAllString(jsonStr, "$1")

	cfg := &AppConfig{}
	if err := json.Unmarshal([]byte(jsonStr), cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config.js JSON: %w", err)
	}

	return cfg, nil
}
