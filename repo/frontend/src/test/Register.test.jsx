import React from 'react';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi } from 'vitest';

vi.mock('../api/client', () => ({
  default: {
    post: vi.fn(),
  },
}));

import Register from '../pages/Register';

describe('Register', () => {
  it('renders registration form with all required fields', () => {
    render(
      <MemoryRouter>
        <Register />
      </MemoryRouter>
    );

    expect(screen.getByLabelText('Username')).toBeInTheDocument();
    expect(screen.getByLabelText('Email')).toBeInTheDocument();
    expect(screen.getByLabelText('Display Name')).toBeInTheDocument();
    expect(screen.getByLabelText('Password')).toBeInTheDocument();
    expect(screen.getByLabelText('Role')).toBeInTheDocument();
  });

  it('has role dropdown with non-privileged roles only (no system_admin)', () => {
    render(
      <MemoryRouter>
        <Register />
      </MemoryRouter>
    );

    const roleSelect = screen.getByLabelText('Role');
    const options = roleSelect.querySelectorAll('option');

    const optionValues = Array.from(options).map((opt) => opt.value);
    // system_admin must NOT be available for self-registration (security fix)
    expect(optionValues).not.toContain('system_admin');
    // All non-privileged roles must be present
    expect(optionValues).toContain('learner');
    expect(optionValues).toContain('content_moderator');
    expect(optionValues).toContain('procurement_specialist');
    expect(optionValues).toContain('approver');
    expect(optionValues).toContain('finance_analyst');
  });

  it('shows login link', () => {
    render(
      <MemoryRouter>
        <Register />
      </MemoryRouter>
    );

    expect(screen.getByText('Sign in')).toBeInTheDocument();
    expect(screen.getByText('Sign in').closest('a')).toHaveAttribute('href', '/login');
  });

  it('submit button exists', () => {
    render(
      <MemoryRouter>
        <Register />
      </MemoryRouter>
    );

    const submitButton = screen.getByRole('button', { name: 'Register' });
    expect(submitButton).toBeInTheDocument();
    expect(submitButton).not.toBeDisabled();
  });
});
