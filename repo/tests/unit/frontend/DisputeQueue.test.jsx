import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock the API client before importing components that use it
vi.mock('../../../frontend/src/api/client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
  },
}));

vi.mock('../../../frontend/src/store/authStore', () => ({
  useAuthStore: vi.fn(),
}));

import DisputeQueue from '../../../frontend/src/pages/DisputeQueue';
import { useAuthStore } from '../../../frontend/src/store/authStore';
import client from '../../../frontend/src/api/client';

// A dispute in 'created' state — triggers upload_evidence action panel
const DISPUTE_CREATED = {
  id: 1,
  status: 'created',
  vendor_name: 'Acme Corp',
  reason: 'Disputed invoice amount',
  review_id: 10,
  evidence_urls: [],
};

// A dispute in 'evidence_uploaded' state — triggers respond + start_review panel
const DISPUTE_EVIDENCE_UPLOADED = {
  id: 2,
  status: 'evidence_uploaded',
  vendor_name: 'Beta LLC',
  reason: 'Wrong item delivered',
  review_id: 11,
  evidence_urls: ['http://fileserver/evidence/doc.pdf'],
};

// A dispute in 'under_review' state — triggers escalate panel (canArbitrate only)
const DISPUTE_UNDER_REVIEW = {
  id: 3,
  status: 'under_review',
  vendor_name: 'Gamma Inc',
  reason: 'Missing delivery',
  review_id: 12,
  evidence_urls: [],
};

function makeAuthStore(roleName) {
  const roles = roleName ? [roleName] : [];
  return {
    hasAnyRole: (...requiredRoles) => roles.some((r) => requiredRoles.includes(r)),
  };
}

function renderDisputeQueue() {
  return render(
    <MemoryRouter>
      <DisputeQueue />
    </MemoryRouter>
  );
}

describe('DisputeQueue — action-level role gating', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Default: return empty disputes so no 401/error
    client.get.mockResolvedValue({ data: [] });
  });

  // -----------------------------------------------------------------------
  // 'created' status: upload_evidence panel requires canArbitrate
  // -----------------------------------------------------------------------
  describe("upload evidence panel (dispute status = 'created')", () => {
    beforeEach(() => {
      client.get.mockImplementation((url) => {
        if (url === '/procurement/disputes') return Promise.resolve({ data: [DISPUTE_CREATED] });
        if (url === '/procurement/reviews') return Promise.resolve({ data: [] });
        return Promise.resolve({ data: [] });
      });
    });

    it('shows Submit Evidence URLs button to content_moderator', async () => {
      useAuthStore.mockReturnValue(makeAuthStore('content_moderator'));
      renderDisputeQueue();
      await waitFor(() => screen.getByText('#1 - Acme Corp'));
      fireEvent.click(screen.getByText('#1 - Acme Corp'));
      expect(await screen.findByText('Submit Evidence URLs')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Upload Evidence' })).toBeInTheDocument();
    });

    it('shows Submit Evidence URLs button to system_admin', async () => {
      useAuthStore.mockReturnValue(makeAuthStore('system_admin'));
      renderDisputeQueue();
      await waitFor(() => screen.getByText('#1 - Acme Corp'));
      fireEvent.click(screen.getByText('#1 - Acme Corp'));
      expect(await screen.findByText('Submit Evidence URLs')).toBeInTheDocument();
    });

    it('does NOT show Upload Evidence button to procurement_specialist', async () => {
      useAuthStore.mockReturnValue(makeAuthStore('procurement_specialist'));
      renderDisputeQueue();
      await waitFor(() => screen.getByText('#1 - Acme Corp'));
      fireEvent.click(screen.getByText('#1 - Acme Corp'));
      // Give time for any conditional render
      await waitFor(() => expect(screen.queryByText('Submit Evidence URLs')).not.toBeInTheDocument());
      expect(screen.queryByRole('button', { name: 'Upload Evidence' })).not.toBeInTheDocument();
    });

    it('does NOT show Upload Evidence button to approver', async () => {
      useAuthStore.mockReturnValue(makeAuthStore('approver'));
      renderDisputeQueue();
      await waitFor(() => screen.getByText('#1 - Acme Corp'));
      fireEvent.click(screen.getByText('#1 - Acme Corp'));
      await waitFor(() => expect(screen.queryByRole('button', { name: 'Upload Evidence' })).not.toBeInTheDocument());
    });

    it('does NOT show Upload Evidence button to finance_analyst', async () => {
      useAuthStore.mockReturnValue(makeAuthStore('finance_analyst'));
      // finance_analyst not allowed to see disputes at all (but we test the UI gate)
      renderDisputeQueue();
      await waitFor(() => screen.getByText('#1 - Acme Corp'));
      fireEvent.click(screen.getByText('#1 - Acme Corp'));
      await waitFor(() => expect(screen.queryByRole('button', { name: 'Upload Evidence' })).not.toBeInTheDocument());
    });
  });

  // -----------------------------------------------------------------------
  // 'evidence_uploaded' status: respond + start_review requires canArbitrate
  // -----------------------------------------------------------------------
  describe("respond/start_review panel (dispute status = 'evidence_uploaded')", () => {
    beforeEach(() => {
      client.get.mockImplementation((url) => {
        if (url === '/procurement/disputes') return Promise.resolve({ data: [DISPUTE_EVIDENCE_UPLOADED] });
        if (url === '/procurement/reviews') return Promise.resolve({ data: [] });
        return Promise.resolve({ data: [] });
      });
    });

    it('shows Start Review button to content_moderator', async () => {
      useAuthStore.mockReturnValue(makeAuthStore('content_moderator'));
      renderDisputeQueue();
      await waitFor(() => screen.getByText('#2 - Beta LLC'));
      fireEvent.click(screen.getByText('#2 - Beta LLC'));
      expect(await screen.findByRole('button', { name: 'Start Review' })).toBeInTheDocument();
    });

    it('shows Start Review button to system_admin', async () => {
      useAuthStore.mockReturnValue(makeAuthStore('system_admin'));
      renderDisputeQueue();
      await waitFor(() => screen.getByText('#2 - Beta LLC'));
      fireEvent.click(screen.getByText('#2 - Beta LLC'));
      expect(await screen.findByRole('button', { name: 'Start Review' })).toBeInTheDocument();
    });

    it('does NOT show Start Review to procurement_specialist', async () => {
      useAuthStore.mockReturnValue(makeAuthStore('procurement_specialist'));
      renderDisputeQueue();
      await waitFor(() => screen.getByText('#2 - Beta LLC'));
      fireEvent.click(screen.getByText('#2 - Beta LLC'));
      await waitFor(() => expect(screen.queryByRole('button', { name: 'Start Review' })).not.toBeInTheDocument());
    });

    it('does NOT show Submit Response to procurement_specialist', async () => {
      useAuthStore.mockReturnValue(makeAuthStore('procurement_specialist'));
      renderDisputeQueue();
      await waitFor(() => screen.getByText('#2 - Beta LLC'));
      fireEvent.click(screen.getByText('#2 - Beta LLC'));
      await waitFor(() => expect(screen.queryByRole('button', { name: 'Submit Response' })).not.toBeInTheDocument());
    });
  });

  // -----------------------------------------------------------------------
  // 'under_review' status: escalate_arbitration requires canArbitrate
  // -----------------------------------------------------------------------
  describe("escalate panel (dispute status = 'under_review')", () => {
    beforeEach(() => {
      client.get.mockImplementation((url) => {
        if (url === '/procurement/disputes') return Promise.resolve({ data: [DISPUTE_UNDER_REVIEW] });
        if (url === '/procurement/reviews') return Promise.resolve({ data: [] });
        return Promise.resolve({ data: [] });
      });
    });

    it('shows Escalate to Arbitration to content_moderator', async () => {
      useAuthStore.mockReturnValue(makeAuthStore('content_moderator'));
      renderDisputeQueue();
      await waitFor(() => screen.getByText('#3 - Gamma Inc'));
      fireEvent.click(screen.getByText('#3 - Gamma Inc'));
      expect(await screen.findByRole('button', { name: 'Escalate to Arbitration' })).toBeInTheDocument();
    });

    it('does NOT show Escalate to Arbitration to procurement_specialist', async () => {
      useAuthStore.mockReturnValue(makeAuthStore('procurement_specialist'));
      renderDisputeQueue();
      await waitFor(() => screen.getByText('#3 - Gamma Inc'));
      fireEvent.click(screen.getByText('#3 - Gamma Inc'));
      await waitFor(() => expect(screen.queryByRole('button', { name: 'Escalate to Arbitration' })).not.toBeInTheDocument());
    });

    it('does NOT show Escalate to Arbitration to approver', async () => {
      useAuthStore.mockReturnValue(makeAuthStore('approver'));
      renderDisputeQueue();
      await waitFor(() => screen.getByText('#3 - Gamma Inc'));
      fireEvent.click(screen.getByText('#3 - Gamma Inc'));
      await waitFor(() => expect(screen.queryByRole('button', { name: 'Escalate to Arbitration' })).not.toBeInTheDocument());
    });
  });
});
