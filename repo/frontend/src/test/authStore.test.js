import { describe, it, expect, beforeEach, vi } from 'vitest';

vi.mock('../api/client', () => ({
  default: {
    post: vi.fn(),
    get: vi.fn(),
    interceptors: {
      request: { use: vi.fn() },
      response: { use: vi.fn() },
    },
  },
}));

import { useAuthStore } from '../store/authStore';

describe('authStore', () => {
  beforeEach(() => {
    // Reset store to a clean state before each test
    useAuthStore.setState({
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
  });

  it('hasRole returns true when user has the role', () => {
    useAuthStore.setState({
      user: {
        roles: [{ name: 'learner' }, { name: 'approver' }],
        permissions: [],
      },
    });

    expect(useAuthStore.getState().hasRole('learner')).toBe(true);
  });

  it('hasRole returns false when user lacks the role', () => {
    useAuthStore.setState({
      user: {
        roles: [{ name: 'learner' }],
        permissions: [],
      },
    });

    expect(useAuthStore.getState().hasRole('system_admin')).toBe(false);
  });

  it('hasAnyRole returns true when user has at least one matching role', () => {
    useAuthStore.setState({
      user: {
        roles: [{ name: 'learner' }],
        permissions: [],
      },
    });

    expect(useAuthStore.getState().hasAnyRole('system_admin', 'learner')).toBe(true);
  });

  it('hasPermission returns true for existing permission', () => {
    useAuthStore.setState({
      user: {
        roles: [],
        permissions: ['read:courses', 'write:courses'],
      },
    });

    expect(useAuthStore.getState().hasPermission('read:courses')).toBe(true);
  });

  it('hasPermission returns false for missing permission', () => {
    useAuthStore.setState({
      user: {
        roles: [],
        permissions: ['read:courses'],
      },
    });

    expect(useAuthStore.getState().hasPermission('delete:courses')).toBe(false);
  });

  it('logout clears all state', async () => {
    useAuthStore.setState({
      token: 'some-token',
      user: { roles: [{ name: 'learner' }], permissions: ['read:courses'] },
      isAuthenticated: true,
      mfaPending: true,
      mfaSessionId: 'session-123',
      loading: true,
      error: 'some error',
      accessDenied: true,
      versionBlocked: true,
      deprecationWarning: '2.0.0',
    });

    await useAuthStore.getState().logout();

    const state = useAuthStore.getState();
    expect(state.token).toBeNull();
    expect(state.user).toBeNull();
    expect(state.isAuthenticated).toBe(false);
    expect(state.mfaPending).toBe(false);
    expect(state.mfaSessionId).toBeNull();
    expect(state.loading).toBe(false);
    expect(state.error).toBeNull();
    expect(state.accessDenied).toBe(false);
    expect(state.versionBlocked).toBe(false);
    expect(state.deprecationWarning).toBeNull();
  });
});
