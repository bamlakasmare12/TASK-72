import React from 'react';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('../../../frontend/src/store/authStore', () => ({
  useAuthStore: vi.fn(),
}));

import ProtectedRoute from '../../../frontend/src/components/ProtectedRoute';
import { useAuthStore } from '../../../frontend/src/store/authStore';

describe('ProtectedRoute', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders children when user has required role', () => {
    useAuthStore.mockReturnValue({
      isAuthenticated: true,
      user: { roles: [{ name: 'learner' }], permissions: [] },
    });

    render(
      <MemoryRouter initialEntries={['/test']}>
        <Routes>
          <Route
            path="/test"
            element={
              <ProtectedRoute requiredRoles={['learner']}>
                <div>Protected Content</div>
              </ProtectedRoute>
            }
          />
        </Routes>
      </MemoryRouter>
    );

    expect(screen.getByText('Protected Content')).toBeInTheDocument();
  });

  it('shows 404 page when user lacks required role', () => {
    useAuthStore.mockReturnValue({
      isAuthenticated: true,
      user: { roles: [{ name: 'learner' }], permissions: [] },
    });

    render(
      <MemoryRouter initialEntries={['/test']}>
        <Routes>
          <Route
            path="/test"
            element={
              <ProtectedRoute requiredRoles={['system_admin']}>
                <div>Admin Content</div>
              </ProtectedRoute>
            }
          />
        </Routes>
      </MemoryRouter>
    );

    expect(screen.queryByText('Admin Content')).not.toBeInTheDocument();
    expect(screen.getByText('Page Not Found')).toBeInTheDocument();
    expect(screen.getByText('404')).toBeInTheDocument();
  });

  it('redirects to /login when not authenticated', () => {
    useAuthStore.mockReturnValue({
      isAuthenticated: false,
      user: null,
    });

    render(
      <MemoryRouter initialEntries={['/protected']}>
        <Routes>
          <Route
            path="/protected"
            element={
              <ProtectedRoute requiredRoles={['learner']}>
                <div>Protected Content</div>
              </ProtectedRoute>
            }
          />
          <Route path="/login" element={<div>Login Page</div>} />
        </Routes>
      </MemoryRouter>
    );

    expect(screen.queryByText('Protected Content')).not.toBeInTheDocument();
    expect(screen.getByText('Login Page')).toBeInTheDocument();
  });
});
