import { create } from 'zustand';
import client from '../api/client';

const STORAGE_KEY = 'wlpr_auth';

function loadPersistedAuth() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    const data = JSON.parse(raw);
    // Check if token is still present (basic check; server validates fully)
    if (data.token) return data;
    return null;
  } catch {
    return null;
  }
}

function persistAuth(state) {
  if (state.token) {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        token: state.token,
        user: state.user,
      })
    );
  } else {
    localStorage.removeItem(STORAGE_KEY);
  }
}

const persisted = loadPersistedAuth();

export const useAuthStore = create((set, get) => ({
  // Auth state
  token: persisted?.token || null,
  user: persisted?.user || null,
  isAuthenticated: !!persisted?.token,
  mfaPending: false,
  mfaSessionId: null,

  // UI state
  loading: false,
  error: null,
  accessDenied: false,
  versionBlocked: false,
  deprecationWarning: null,

  // Login step 1: credentials
  login: async (username, password) => {
    set({ loading: true, error: null, accessDenied: false });
    try {
      const res = await client.post('/auth/login', { username, password });
      const data = res.data;

      if (data.requires_mfa) {
        set({
          loading: false,
          mfaPending: true,
          mfaSessionId: data.session_id,
        });
        return { requiresMFA: true };
      }

      const newState = {
        token: data.token,
        user: data.user,
        isAuthenticated: true,
        loading: false,
        mfaPending: false,
        mfaSessionId: null,
      };
      set(newState);
      persistAuth(newState);
      return { requiresMFA: false };
    } catch (err) {
      const msg =
        err.response?.data?.message || err.message || 'Login failed';
      set({ loading: false, error: msg });
      throw new Error(msg);
    }
  },

  // Login step 2: MFA verification
  verifyMFA: async (code) => {
    const { mfaSessionId } = get();
    if (!mfaSessionId) {
      set({ error: 'No MFA session active' });
      throw new Error('No MFA session active');
    }

    set({ loading: true, error: null });
    try {
      const res = await client.post('/auth/mfa/verify', {
        code,
        session_id: mfaSessionId,
      });
      const data = res.data;

      const newState = {
        token: data.token,
        user: data.user,
        isAuthenticated: true,
        loading: false,
        mfaPending: false,
        mfaSessionId: null,
      };
      set(newState);
      persistAuth(newState);
    } catch (err) {
      const msg =
        err.response?.data?.message || err.message || 'MFA verification failed';
      set({ loading: false, error: msg });
      throw new Error(msg);
    }
  },

  // Logout: clear all state
  logout: async () => {
    const { token } = get();
    if (token) {
      try {
        await client.post('/auth/logout');
      } catch {
        // Ignore logout API errors
      }
    }
    set({
      token: null,
      user: null,
      isAuthenticated: false,
      mfaPending: false,
      mfaSessionId: null,
      loading: false,
      error: null,
      accessDenied: false,
      versionBlocked: false,
      deprecationWarning: null,
    });
    localStorage.removeItem(STORAGE_KEY);
  },

  // Fetch current user info
  fetchMe: async () => {
    try {
      const res = await client.get('/auth/me');
      set({ user: { ...get().user, ...res.data } });
    } catch {
      // Token might be invalid
      get().logout();
    }
  },

  // MFA setup
  setupMFA: async () => {
    const res = await client.post('/auth/mfa/setup');
    return res.data; // { secret, qr_code }
  },

  confirmMFA: async (code) => {
    const res = await client.post('/auth/mfa/confirm', { code });
    return res.data; // { message, recovery_codes }
  },

  disableMFA: async () => {
    await client.post('/auth/mfa/disable');
    set((state) => ({
      user: state.user ? { ...state.user, mfa_enabled: false } : null,
    }));
  },

  // UI state setters
  clearError: () => set({ error: null }),
  setAccessDenied: (val) => set({ accessDenied: val }),
  setVersionBlocked: (val) => set({ versionBlocked: val }),
  setDeprecationWarning: (minVersion) =>
    set({ deprecationWarning: minVersion }),

  // Permission helpers
  hasRole: (role) => {
    const { user } = get();
    return user?.roles?.some((r) => r.name === role) || false;
  },

  hasPermission: (perm) => {
    const { user } = get();
    return user?.permissions?.includes(perm) || false;
  },

  hasAnyRole: (...roles) => {
    const { user } = get();
    return user?.roles?.some((r) => roles.includes(r.name)) || false;
  },
}));
