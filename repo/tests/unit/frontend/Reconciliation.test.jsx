import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('../../../frontend/src/api/client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
  },
}));

vi.mock('../../../frontend/src/store/authStore', () => ({
  useAuthStore: vi.fn(),
}));

import Reconciliation from '../../../frontend/src/pages/Reconciliation';
import { useAuthStore } from '../../../frontend/src/store/authStore';
import client from '../../../frontend/src/api/client';

// A pending invoice — the match dropdown is only shown if canMatchInvoice
const PENDING_INVOICE = {
  id: 1,
  invoice_number: 'INV-001',
  vendor_name: 'Acme Corp',
  invoice_amount: 500.0,
  order_amount: null,
  variance_amount: null,
  status: 'pending',
};

const MATCHED_INVOICE = {
  id: 2,
  invoice_number: 'INV-002',
  vendor_name: 'Beta LLC',
  invoice_amount: 200.0,
  order_amount: 200.0,
  variance_amount: 0.0,
  status: 'matched',
};

const OPEN_ORDER = {
  id: 10,
  order_number: 'ORD-010',
  total_amount: 500.0,
};

const SETTLEMENT = {
  id: 1,
  vendor_name: 'Acme Corp',
  ar_total: 500.0,
  ap_total: 500.0,
  variance_amount: 0.0,
  writeoff_amount: 0.0,
  status: 'open',
};

function makeAuthStore(roleName) {
  const roles = roleName ? [roleName] : [];
  return {
    hasAnyRole: (...requiredRoles) => roles.some((r) => requiredRoles.includes(r)),
  };
}

function renderReconciliation() {
  return render(
    <MemoryRouter>
      <Reconciliation />
    </MemoryRouter>
  );
}

describe('Reconciliation — invoice match action role gating', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    client.get.mockImplementation((url) => {
      if (url === '/procurement/invoices') return Promise.resolve({ data: [PENDING_INVOICE, MATCHED_INVOICE] });
      if (url === '/procurement/settlements') return Promise.resolve({ data: [SETTLEMENT] });
      if (url === '/procurement/orders') return Promise.resolve({ data: [OPEN_ORDER] });
      if (url === '/procurement/cost-allocation') return Promise.resolve({ data: [] });
      return Promise.resolve({ data: [] });
    });
  });

  describe('match-to-order dropdown for pending invoices', () => {
    it('shows match dropdown to approver', async () => {
      useAuthStore.mockReturnValue(makeAuthStore('approver'));
      renderReconciliation();
      // Wait for invoices tab to render (default tab)
      await waitFor(() => screen.getByText('INV-001'));
      expect(screen.getByRole('combobox')).toBeInTheDocument();
      expect(screen.getByText('Match to order...')).toBeInTheDocument();
    });

    it('shows match dropdown to system_admin', async () => {
      useAuthStore.mockReturnValue(makeAuthStore('system_admin'));
      renderReconciliation();
      await waitFor(() => screen.getByText('INV-001'));
      expect(screen.getByRole('combobox')).toBeInTheDocument();
      expect(screen.getByText('Match to order...')).toBeInTheDocument();
    });

    it('does NOT show match dropdown to finance_analyst', async () => {
      useAuthStore.mockReturnValue(makeAuthStore('finance_analyst'));
      renderReconciliation();
      await waitFor(() => screen.getByText('INV-001'));
      expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
      expect(screen.queryByText('Match to order...')).not.toBeInTheDocument();
    });

    it('does NOT show match dropdown to procurement_specialist', async () => {
      useAuthStore.mockReturnValue(makeAuthStore('procurement_specialist'));
      renderReconciliation();
      await waitFor(() => screen.getByText('INV-001'));
      expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
    });

    it('does NOT show match dropdown to learner', async () => {
      useAuthStore.mockReturnValue(makeAuthStore('learner'));
      renderReconciliation();
      await waitFor(() => screen.getByText('INV-001'));
      expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
    });

    it('does not show match dropdown for already-matched invoices even for approver', async () => {
      useAuthStore.mockReturnValue(makeAuthStore('approver'));
      renderReconciliation();
      // There's only one pending invoice (INV-001), so only one combobox
      await waitFor(() => screen.getByText('INV-002'));
      const combos = screen.queryAllByRole('combobox');
      // Only INV-001 is pending — should be exactly one combobox
      expect(combos).toHaveLength(1);
    });
  });

  describe('export buttons — finance_analyst + system_admin only', () => {
    it('shows export buttons to finance_analyst', async () => {
      useAuthStore.mockReturnValue(makeAuthStore('finance_analyst'));
      renderReconciliation();
      await waitFor(() => screen.getByText('INV-001'));
      expect(screen.getByRole('button', { name: 'Export Ledger' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Export Settlements' })).toBeInTheDocument();
    });

    it('does NOT show export buttons to approver', async () => {
      useAuthStore.mockReturnValue(makeAuthStore('approver'));
      renderReconciliation();
      await waitFor(() => screen.getByText('INV-001'));
      expect(screen.queryByRole('button', { name: 'Export Ledger' })).not.toBeInTheDocument();
    });
  });
});
