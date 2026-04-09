// Runtime configuration for the WLPR Portal.
// This file is the SINGLE source of truth for all configuration.
// Both the frontend (via <script> tag) and backend (parsed at startup) read from this file.
// Override at deploy time by mounting a replacement via Docker volume.
//
// IMPORTANT: The object below must use double-quoted keys (valid JSON) so the
// backend Go parser can read it. Values must also be double-quoted strings or
// valid JSON literals (true/false/numbers/objects).
window.__WLPR_CONFIG__ = {
  "API_BASE_URL": "/api",
  "APP_VERSION": "1.0.0",
  "SESSION_IDLE_WARNING_SECONDS": 840,
  "SESSION_MAX_HOURS": 8,
  "FEATURE_DEFAULTS": {
    "pinyin_search": true,
    "synonym_search": true,
    "learning_recommendations": true,
    "procurement_disputes": true
  },

  "DATABASE_URL": "postgres://wlpr:wlpr_secret@db:5432/wlpr_portal?sslmode=disable",
  "JWT_SECRET": "change-me-to-a-secure-random-string-at-least-32-chars",
  "AES_ENCRYPTION_KEY": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
  "PORT": "8080",

  "POSTGRES_DB": "wlpr_portal",
  "POSTGRES_USER": "wlpr",
  "POSTGRES_PASSWORD": "wlpr_secret"
};
