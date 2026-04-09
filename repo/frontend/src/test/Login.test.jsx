import React from 'react';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi } from 'vitest';

vi.mock('../store/authStore', () => ({
  useAuthStore: vi.fn(),
}));

import Login from '../pages/Login';
import { useAuthStore } from '../store/authStore';

describe('Login', () => {
  beforeEach(() => {
    useAuthStore.mockReturnValue({
      login: vi.fn(),
      verifyMFA: vi.fn(),
      loading: false,
      error: null,
      clearError: vi.fn(),
      mfaPending: false,
    });
  });

  it('renders login form with username and password fields', () => {
    render(
      <MemoryRouter>
        <Login />
      </MemoryRouter>
    );

    expect(screen.getByLabelText('Username')).toBeInTheDocument();
    expect(screen.getByLabelText('Password')).toBeInTheDocument();
  });

  it('shows register link', () => {
    render(
      <MemoryRouter>
        <Login />
      </MemoryRouter>
    );

    expect(screen.getByText('Register')).toBeInTheDocument();
    expect(screen.getByText('Register').closest('a')).toHaveAttribute('href', '/register');
  });

  it('submit button exists and is not disabled initially', () => {
    render(
      <MemoryRouter>
        <Login />
      </MemoryRouter>
    );

    const submitButton = screen.getByRole('button', { name: 'Sign In' });
    expect(submitButton).toBeInTheDocument();
    expect(submitButton).not.toBeDisabled();
  });
});
