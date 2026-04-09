// Default runtime configuration for the WLPR Portal frontend.
// In Docker, this file is overridden by mounting config/config.js as a volume.
// For local development, edit config/config.js in the project root.
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
  }
};
