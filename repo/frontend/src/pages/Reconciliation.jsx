import React, { useState, useEffect, useCallback } from 'react';
import client from '../api/client';
import LoadingSpinner from '../components/LoadingSpinner';
import { useAuthStore } from '../store/authStore';

const STATUS_COLORS = {
  open: 'bg-gray-100 text-gray-700',
  matched: 'bg-green-100 text-green-700',
  variance_pending: 'bg-yellow-100 text-yellow-700',
  writeoff_suggested: 'bg-orange-100 text-orange-700',
  writeoff_approved: 'bg-blue-100 text-blue-700',
  settled: 'bg-green-200 text-green-800',
  disputed: 'bg-red-100 text-red-700',
  pending: 'bg-gray-100 text-gray-600',
  pending_approval: 'bg-yellow-100 text-yellow-700',
  manual_investigation: 'bg-red-100 text-red-700',
  paid: 'bg-green-200 text-green-800',
};

const INVOICE_STATUS_COLORS = {
  pending: 'bg-gray-100 text-gray-600',
  matched: 'bg-green-100 text-green-700',
  variance_detected: 'bg-yellow-100 text-yellow-700',
  pending_approval: 'bg-orange-100 text-orange-700',
  manual_investigation: 'bg-red-100 text-red-700',
  approved: 'bg-blue-100 text-blue-700',
  rejected: 'bg-red-200 text-red-800',
  paid: 'bg-green-200 text-green-800',
};

export default function Reconciliation() {
  const { hasAnyRole } = useAuthStore();
  // Only finance_analyst and system_admin can access cost allocation, ledger, compare, and exports
  const isFinanceOrAdmin = hasAnyRole('finance_analyst', 'system_admin');
  // Only approver and system_admin can match invoices to orders (backend: procApprove group)
  const canMatchInvoice = hasAnyRole('approver', 'system_admin');

  const [activeTab, setActiveTab] = useState('invoices');
  const [invoices, setInvoices] = useState([]);
  const [settlements, setSettlements] = useState([]);
  const [costAllocation, setCostAllocation] = useState([]);
  const [orders, setOrders] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [actionMsg, setActionMsg] = useState('');

  // Statement comparison
  const [compareForm, setCompareForm] = useState({ vendor_id: '', statement_total: '', period_start: '', period_end: '' });
  const [compareResult, setCompareResult] = useState(null);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const fetches = [
        client.get('/procurement/invoices'),
        client.get('/procurement/settlements'),
        client.get('/procurement/orders'),
      ];
      // Cost allocation is restricted to finance_analyst + system_admin
      if (isFinanceOrAdmin) {
        fetches.push(client.get('/procurement/cost-allocation'));
      }
      const results = await Promise.all(fetches);
      setInvoices(results[0].data || []);
      setSettlements(results[1].data || []);
      setOrders(results[2].data || []);
      setCostAllocation(isFinanceOrAdmin ? (results[3].data || []) : []);
    } catch (err) {
      setError(err.response?.data?.message || 'Failed to load reconciliation data');
    } finally {
      setLoading(false);
    }
  }, [isFinanceOrAdmin]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const handleMatchInvoice = async (invoiceId, orderId) => {
    try {
      await client.post('/procurement/invoices/match', { invoice_id: invoiceId, order_id: orderId });
      setActionMsg('Invoice matched successfully');
      fetchData();
    } catch (err) {
      setError(err.response?.data?.message || 'Match failed');
    }
  };

  const handleSettlementTransition = async (settlementId, action) => {
    try {
      await client.post('/procurement/settlements/transition', { settlement_id: settlementId, action });
      setActionMsg(`Settlement ${action} successful`);
      fetchData();
    } catch (err) {
      setError(err.response?.data?.message || 'Transition failed');
    }
  };

  const handleCompare = async (e) => {
    e.preventDefault();
    setCompareResult(null);
    try {
      const res = await client.post('/procurement/reconciliation/compare', {
        vendor_id: parseInt(compareForm.vendor_id),
        statement_total: parseFloat(compareForm.statement_total),
        period_start: compareForm.period_start,
        period_end: compareForm.period_end,
      });
      setCompareResult(res.data);
    } catch (err) {
      setError(err.response?.data?.message || 'Comparison failed');
    }
  };

  const handleExport = async (exportType) => {
    try {
      const endpoint = exportType === 'ledger'
        ? '/procurement/export/ledger'
        : '/procurement/export/settlements';
      const res = await client.get(endpoint, { responseType: 'blob' });
      const filename = exportType === 'ledger' ? 'ledger_export.csv' : 'settlements_export.csv';
      const url = window.URL.createObjectURL(new Blob([res.data]));
      const link = document.createElement('a');
      link.href = url;
      link.setAttribute('download', filename);
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.URL.revokeObjectURL(url);
      setActionMsg(`${filename} downloaded`);
    } catch (err) {
      setError(err.response?.data?.message || 'Export failed');
    }
  };

  if (loading) return <LoadingSpinner text="Loading reconciliation data..." />;

  const tabs = [
    { key: 'invoices', label: 'Invoices' },
    { key: 'settlements', label: 'Settlements' },
    // Compare and cost allocation are finance_analyst + system_admin only
    ...(isFinanceOrAdmin ? [
      { key: 'compare', label: 'Statement Compare' },
      { key: 'cost', label: 'Cost Allocation' },
    ] : []),
  ];

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Finance & Reconciliation</h1>
        {isFinanceOrAdmin && (
          <div className="flex space-x-2">
            <button onClick={() => handleExport('ledger')} className="px-3 py-1.5 text-xs border border-gray-300 rounded-md hover:bg-gray-50">Export Ledger</button>
            <button onClick={() => handleExport('settlements')} className="px-3 py-1.5 text-xs border border-gray-300 rounded-md hover:bg-gray-50">Export Settlements</button>
          </div>
        )}
      </div>

      {error && (
        <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-md text-sm text-red-700">
          {error} <button onClick={() => setError(null)} className="ml-2 font-medium">Dismiss</button>
        </div>
      )}
      {actionMsg && (
        <div className="mb-4 p-3 bg-green-50 border border-green-200 rounded-md text-sm text-green-700">
          {actionMsg} <button onClick={() => setActionMsg('')} className="ml-2 font-medium">Dismiss</button>
        </div>
      )}

      {/* Tabs */}
      <div className="border-b border-gray-200 mb-6">
        <div className="flex space-x-6">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              onClick={() => setActiveTab(tab.key)}
              className={`pb-3 text-sm font-medium border-b-2 transition-colors ${
                activeTab === tab.key
                  ? 'border-primary-600 text-primary-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>
      </div>

      {/* Invoices Tab */}
      {activeTab === 'invoices' && (
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 overflow-hidden">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Invoice #</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Vendor</th>
                <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase">Invoice Amt</th>
                <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase">Order Amt</th>
                <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase">Variance</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {invoices.map((inv) => (
                <tr key={inv.id} className="hover:bg-gray-50">
                  <td className="px-4 py-3 text-sm font-mono">{inv.invoice_number}</td>
                  <td className="px-4 py-3 text-sm">{inv.vendor_name}</td>
                  <td className="px-4 py-3 text-sm text-right font-mono">${inv.invoice_amount?.toFixed(2)}</td>
                  <td className="px-4 py-3 text-sm text-right font-mono">{inv.order_amount != null ? `$${inv.order_amount.toFixed(2)}` : '-'}</td>
                  <td className={`px-4 py-3 text-sm text-right font-mono ${inv.variance_amount && Math.abs(inv.variance_amount) > 0 ? 'text-red-600 font-medium' : ''}`}>
                    {inv.variance_amount != null ? `$${inv.variance_amount.toFixed(2)}` : '-'}
                  </td>
                  <td className="px-4 py-3">
                    <span className={`px-2 py-0.5 rounded text-xs font-medium ${INVOICE_STATUS_COLORS[inv.status] || 'bg-gray-100'}`}>
                      {inv.status?.replace(/_/g, ' ')}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-sm">
                    {inv.status === 'pending' && canMatchInvoice && (
                      <select
                        onChange={(e) => { if (e.target.value) handleMatchInvoice(inv.id, parseInt(e.target.value)); }}
                        className="text-xs border rounded px-2 py-1"
                        defaultValue=""
                      >
                        <option value="">Match to order...</option>
                        {orders.map((o) => (
                          <option key={o.id} value={o.id}>{o.order_number} (${o.total_amount?.toFixed(2)})</option>
                        ))}
                      </select>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {invoices.length === 0 && <div className="py-8 text-center text-gray-400">No invoices</div>}
        </div>
      )}

      {/* Settlements Tab */}
      {activeTab === 'settlements' && (
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 overflow-hidden">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Vendor</th>
                <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase">AR Total</th>
                <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase">AP Total</th>
                <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase">Variance</th>
                <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase">Write-off</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {settlements.map((s) => (
                <tr key={s.id} className="hover:bg-gray-50">
                  <td className="px-4 py-3 text-sm font-medium">{s.vendor_name}</td>
                  <td className="px-4 py-3 text-sm text-right font-mono">${s.ar_total?.toFixed(2)}</td>
                  <td className="px-4 py-3 text-sm text-right font-mono">${s.ap_total?.toFixed(2)}</td>
                  <td className={`px-4 py-3 text-sm text-right font-mono ${Math.abs(s.variance_amount) > 0 ? 'text-red-600' : ''}`}>${s.variance_amount?.toFixed(2)}</td>
                  <td className="px-4 py-3 text-sm text-right font-mono">${s.writeoff_amount?.toFixed(2)}</td>
                  <td className="px-4 py-3">
                    <span className={`px-2 py-0.5 rounded text-xs font-medium ${STATUS_COLORS[s.status] || 'bg-gray-100'}`}>
                      {s.status?.replace(/_/g, ' ')}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex space-x-1">
                      {s.status === 'writeoff_suggested' && (
                        <button onClick={() => handleSettlementTransition(s.id, 'approve_writeoff')}
                          className="px-2 py-1 text-xs bg-green-600 text-white rounded hover:bg-green-700">Approve</button>
                      )}
                      {(s.status === 'matched' || s.status === 'writeoff_approved') && (
                        <button onClick={() => handleSettlementTransition(s.id, 'settle')}
                          className="px-2 py-1 text-xs bg-blue-600 text-white rounded hover:bg-blue-700">Settle</button>
                      )}
                      {s.status !== 'settled' && s.status !== 'disputed' && (
                        <button onClick={() => handleSettlementTransition(s.id, 'dispute')}
                          className="px-2 py-1 text-xs bg-red-500 text-white rounded hover:bg-red-600">Dispute</button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {settlements.length === 0 && <div className="py-8 text-center text-gray-400">No settlements</div>}
        </div>
      )}

      {/* Statement Compare Tab */}
      {activeTab === 'compare' && (
        <div className="max-w-2xl">
          <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
            <h2 className="text-lg font-semibold text-gray-800 mb-4">Compare Vendor Statement</h2>
            <form onSubmit={handleCompare} className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-medium text-gray-500 mb-1">Vendor ID</label>
                  <input type="number" value={compareForm.vendor_id}
                    onChange={(e) => setCompareForm(f => ({ ...f, vendor_id: e.target.value }))}
                    className="w-full border rounded-md px-3 py-2 text-sm" required />
                </div>
                <div>
                  <label className="block text-xs font-medium text-gray-500 mb-1">Statement Total ($)</label>
                  <input type="number" step="0.01" value={compareForm.statement_total}
                    onChange={(e) => setCompareForm(f => ({ ...f, statement_total: e.target.value }))}
                    className="w-full border rounded-md px-3 py-2 text-sm" required />
                </div>
                <div>
                  <label className="block text-xs font-medium text-gray-500 mb-1">Period Start</label>
                  <input type="date" value={compareForm.period_start}
                    onChange={(e) => setCompareForm(f => ({ ...f, period_start: e.target.value }))}
                    className="w-full border rounded-md px-3 py-2 text-sm" />
                </div>
                <div>
                  <label className="block text-xs font-medium text-gray-500 mb-1">Period End</label>
                  <input type="date" value={compareForm.period_end}
                    onChange={(e) => setCompareForm(f => ({ ...f, period_end: e.target.value }))}
                    className="w-full border rounded-md px-3 py-2 text-sm" />
                </div>
              </div>
              <button type="submit" className="px-4 py-2 bg-primary-600 text-white rounded-md text-sm hover:bg-primary-700">
                Compare
              </button>
            </form>
            {compareResult && (
              <div className="mt-6 p-4 bg-gray-50 rounded-lg">
                <h3 className="text-sm font-semibold text-gray-800 mb-3">Comparison Result</h3>
                <dl className="grid grid-cols-2 gap-3 text-sm">
                  <div><dt className="text-gray-500">Statement Total</dt><dd className="font-mono font-medium">${compareResult.statement_total?.toFixed(2)}</dd></div>
                  <div><dt className="text-gray-500">Ledger Total</dt><dd className="font-mono font-medium">${compareResult.ledger_total?.toFixed(2)}</dd></div>
                  <div><dt className="text-gray-500">Variance</dt>
                    <dd className={`font-mono font-medium ${Math.abs(compareResult.variance) > 0 ? 'text-red-600' : 'text-green-600'}`}>
                      ${compareResult.variance?.toFixed(2)} ({compareResult.variance_pct?.toFixed(2)}%)
                    </dd>
                  </div>
                  <div><dt className="text-gray-500">Suggested Action</dt>
                    <dd>
                      <span className={`px-2 py-0.5 rounded text-xs font-medium ${STATUS_COLORS[compareResult.suggested_state] || 'bg-gray-100'}`}>
                        {compareResult.suggested_state?.replace(/_/g, ' ')}
                      </span>
                      {compareResult.auto_writeoff && (
                        <span className="ml-2 text-xs text-orange-600 font-medium">Auto write-off suggested</span>
                      )}
                    </dd>
                  </div>
                </dl>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Cost Allocation Tab */}
      {activeTab === 'cost' && (
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 overflow-hidden">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Department</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Cost Center</th>
                <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase">AR Total</th>
                <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase">AP Total</th>
                <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase">Net</th>
                <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase">Entries</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {costAllocation.map((ca, i) => (
                <tr key={i} className="hover:bg-gray-50">
                  <td className="px-4 py-3 text-sm font-medium">{ca.department}</td>
                  <td className="px-4 py-3 text-sm">{ca.cost_center}</td>
                  <td className="px-4 py-3 text-sm text-right font-mono">${ca.ar_total?.toFixed(2)}</td>
                  <td className="px-4 py-3 text-sm text-right font-mono">${ca.ap_total?.toFixed(2)}</td>
                  <td className={`px-4 py-3 text-sm text-right font-mono font-medium ${ca.net_amount >= 0 ? 'text-green-600' : 'text-red-600'}`}>
                    ${ca.net_amount?.toFixed(2)}
                  </td>
                  <td className="px-4 py-3 text-sm text-right">{ca.entry_count}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {costAllocation.length === 0 && <div className="py-8 text-center text-gray-400">No cost allocation data</div>}
        </div>
      )}
    </div>
  );
}
