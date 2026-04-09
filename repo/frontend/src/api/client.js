import axios from 'axios';
import { useAuthStore } from '../store/authStore';

// Runtime config from config.js (injected via Docker volume or served statically)
const runtimeConfig = window.__WLPR_CONFIG__ || {};
const API_BASE_URL = runtimeConfig.API_BASE_URL || '/api';
const APP_VERSION = runtimeConfig.APP_VERSION || '1.0.0';

const client = axios.create({
  baseURL: API_BASE_URL,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
    'X-App-Version': APP_VERSION,
  },
});

// Request interceptor: attach JWT token
client.interceptors.request.use(
  (config) => {
    const token = useAuthStore.getState().token;
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// Response interceptor: handle auth errors, version warnings, retries
client.interceptors.response.use(
  (response) => {
    // Check for app deprecation warning
    if (response.headers['x-app-deprecated'] === 'true') {
      const minVersion = response.headers['x-min-version'] || 'unknown';
      console.warn(
        `[WLPR] Client version deprecated. Minimum: ${minVersion}. Write operations may be blocked.`
      );
      useAuthStore.getState().setDeprecationWarning(minVersion);
    }
    return response;
  },
  async (error) => {
    const originalRequest = error.config;

    // 401 Unauthorized -> force logout
    if (error.response?.status === 401) {
      useAuthStore.getState().logout();
      window.location.href = '/login';
      return Promise.reject(error);
    }

    // 403 Forbidden -> set access denied state
    if (error.response?.status === 403) {
      useAuthStore.getState().setAccessDenied(true);
      return Promise.reject(error);
    }

    // 426 Upgrade Required -> version blocked
    if (error.response?.status === 426) {
      useAuthStore.getState().setVersionBlocked(true);
      return Promise.reject(error);
    }

    // 5xx -> retry once
    if (
      error.response?.status >= 500 &&
      !originalRequest._retried
    ) {
      originalRequest._retried = true;
      return client(originalRequest);
    }

    return Promise.reject(error);
  }
);

export default client;
