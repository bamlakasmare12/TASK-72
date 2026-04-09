import React, { useState, useEffect, useCallback } from 'react';
import client from '../api/client';
import LoadingSpinner from '../components/LoadingSpinner';

const DISPUTE_STATUS_COLORS = {
  created: 'bg-gray-100 text-gray-700',
  evidence_uploaded: 'bg-yellow-100 text-yellow-700',
  under_review: 'bg-blue-100 text-blue-700',
  arbitration: 'bg-purple-100 text-purple-700',
  resolved_hidden: 'bg-red-100 text-red-700',
  resolved_disclaimer: 'bg-orange-100 text-orange-700',
  resolved_restored: 'bg-green-100 text-green-700',
  rejected: 'bg-gray-200 text-gray-600',
};

const REVIEW_STATUS_ICONS = {
  visible: { label: 'Visible', color: 'text-green-600' },
  hidden: { label: 'Hidden', color: 'text-red-600' },
  disclaimer: { label: 'Disclaimer', color: 'text-orange-600' },
};

export default function DisputeQueue() {
  const [disputes, setDisputes] = useState([]);
  const [reviews, setReviews] = useState([]);
  const [selectedDispute, setSelectedDispute] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [actionMsg, setActionMsg] = useState('');

  // Transition form state
  const [transForm, setTransForm] = useState({
    evidence_urls: '',
    merchant_response: '',
    arbitration_notes: '',
    arbitration_outcome: 'restore',
  });

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [dispRes, revRes] = await Promise.all([
        client.get('/procurement/disputes'),
        client.get('/procurement/reviews'),
      ]);
      setDisputes(dispRes.data || []);
      setReviews(revRes.data || []);
    } catch (err) {
      setError(err.response?.data?.message || 'Failed to load dispute data');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchData(); }, [fetchData]);

  const handleTransition = async (disputeId, action) => {
    setError(null);
    try {
      const payload = { dispute_id: disputeId, action };

      if (action === 'upload_evidence') {
        const urls = transForm.evidence_urls.split('\n').map(s => s.trim()).filter(Boolean);
        if (urls.length === 0) {
          setError('At least one evidence URL is required');
          return;
        }
        payload.evidence_urls = urls;
      }
      if (action === 'respond') {
        if (!transForm.merchant_response.trim()) {
          setError('Merchant response text is required');
          return;
        }
        payload.merchant_response = transForm.merchant_response;
      }
      if (action === 'arbitrate') {
        payload.arbitration_notes = transForm.arbitration_notes;
        payload.arbitration_outcome = transForm.arbitration_outcome;
      }
      if (action === 'reject') {
        payload.arbitration_notes = transForm.arbitration_notes;
      }

      await client.post('/procurement/disputes/transition', payload);
      setActionMsg(`Dispute ${action.replace(/_/g, ' ')} successful`);
      setTransForm({ evidence_urls: '', merchant_response: '', arbitration_notes: '', arbitration_outcome: 'restore' });
      setSelectedDispute(null);
      fetchData();
    } catch (err) {
      setError(err.response?.data?.message || `Action ${action} failed`);
    }
  };

  if (loading) return <LoadingSpinner text="Loading dispute queue..." />;

  const activeDisputes = disputes.filter(d => !d.status.startsWith('resolved_') && d.status !== 'rejected');
  const resolvedDisputes = disputes.filter(d => d.status.startsWith('resolved_') || d.status === 'rejected');

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <h1 className="text-2xl font-bold text-gray-900 mb-6">Review Disputes</h1>

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

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Dispute list */}
        <div className="lg:col-span-1">
          <h2 className="text-sm font-semibold text-gray-700 mb-3 uppercase tracking-wider">
            Active ({activeDisputes.length})
          </h2>
          <div className="space-y-2 mb-6">
            {activeDisputes.map((d) => (
              <button
                key={d.id}
                onClick={() => setSelectedDispute(d)}
                className={`w-full text-left p-3 rounded-lg border transition-all ${
                  selectedDispute?.id === d.id ? 'border-primary-500 bg-primary-50 shadow-sm' : 'border-gray-200 bg-white hover:border-gray-300'
                }`}
              >
                <div className="flex items-center justify-between mb-1">
                  <span className="text-sm font-medium text-gray-800">#{d.id} - {d.vendor_name}</span>
                  <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${DISPUTE_STATUS_COLORS[d.status]}`}>
                    {d.status.replace(/_/g, ' ')}
                  </span>
                </div>
                <p className="text-xs text-gray-500 line-clamp-2">{d.reason}</p>
              </button>
            ))}
            {activeDisputes.length === 0 && (
              <p className="text-sm text-gray-400 text-center py-4">No active disputes</p>
            )}
          </div>

          <h2 className="text-sm font-semibold text-gray-700 mb-3 uppercase tracking-wider">
            Resolved ({resolvedDisputes.length})
          </h2>
          <div className="space-y-2">
            {resolvedDisputes.map((d) => (
              <button
                key={d.id}
                onClick={() => setSelectedDispute(d)}
                className={`w-full text-left p-3 rounded-lg border transition-all ${
                  selectedDispute?.id === d.id ? 'border-primary-500 bg-primary-50' : 'border-gray-200 bg-white hover:border-gray-300'
                }`}
              >
                <div className="flex items-center justify-between mb-1">
                  <span className="text-sm font-medium text-gray-600">#{d.id} - {d.vendor_name}</span>
                  <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${DISPUTE_STATUS_COLORS[d.status]}`}>
                    {d.status.replace(/_/g, ' ')}
                  </span>
                </div>
              </button>
            ))}
          </div>
        </div>

        {/* Dispute detail + actions */}
        <div className="lg:col-span-2">
          {selectedDispute ? (
            <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
              <div className="flex items-start justify-between mb-4">
                <div>
                  <h3 className="text-lg font-semibold text-gray-900">Dispute #{selectedDispute.id}</h3>
                  <p className="text-sm text-gray-500">Vendor: {selectedDispute.vendor_name} | Review #{selectedDispute.review_id}</p>
                </div>
                <span className={`px-2 py-1 rounded text-xs font-medium ${DISPUTE_STATUS_COLORS[selectedDispute.status]}`}>
                  {selectedDispute.status.replace(/_/g, ' ')}
                </span>
              </div>

              {/* State machine visualization */}
              <div className="mb-6 flex items-center space-x-1 text-xs overflow-x-auto pb-2">
                {['created', 'evidence_uploaded', 'under_review', 'arbitration', 'resolved'].map((step, i) => {
                  const isCurrent = selectedDispute.status === step ||
                    (step === 'resolved' && selectedDispute.status.startsWith('resolved_'));
                  const isPast = ['created', 'evidence_uploaded', 'under_review', 'arbitration', 'resolved']
                    .indexOf(step) < ['created', 'evidence_uploaded', 'under_review', 'arbitration']
                    .indexOf(selectedDispute.status);
                  return (
                    <React.Fragment key={step}>
                      {i > 0 && <span className="text-gray-300">-</span>}
                      <span className={`px-2 py-1 rounded whitespace-nowrap ${
                        isCurrent ? 'bg-primary-600 text-white' : isPast ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'
                      }`}>
                        {step.replace(/_/g, ' ')}
                      </span>
                    </React.Fragment>
                  );
                })}
              </div>

              <div className="space-y-4">
                <div>
                  <h4 className="text-xs font-semibold text-gray-500 uppercase mb-1">Reason</h4>
                  <p className="text-sm text-gray-800">{selectedDispute.reason}</p>
                </div>

                {selectedDispute.evidence_urls && selectedDispute.evidence_urls.length > 0 && (
                  <div>
                    <h4 className="text-xs font-semibold text-gray-500 uppercase mb-1">Evidence</h4>
                    <ul className="list-disc list-inside text-sm text-blue-600">
                      {selectedDispute.evidence_urls.map((url, i) => (
                        <li key={i}>{url}</li>
                      ))}
                    </ul>
                  </div>
                )}

                {selectedDispute.merchant_response && (
                  <div>
                    <h4 className="text-xs font-semibold text-gray-500 uppercase mb-1">Merchant Response</h4>
                    <p className="text-sm text-gray-800 bg-yellow-50 p-3 rounded">{selectedDispute.merchant_response}</p>
                  </div>
                )}

                {selectedDispute.arbitration_notes && (
                  <div>
                    <h4 className="text-xs font-semibold text-gray-500 uppercase mb-1">Arbitration Notes</h4>
                    <p className="text-sm text-gray-800">{selectedDispute.arbitration_notes}</p>
                  </div>
                )}

                {selectedDispute.arbitration_outcome && (
                  <div>
                    <h4 className="text-xs font-semibold text-gray-500 uppercase mb-1">Outcome</h4>
                    <span className="text-sm font-medium capitalize">{selectedDispute.arbitration_outcome}</span>
                  </div>
                )}
              </div>

              {/* Actions based on current status */}
              <div className="mt-6 pt-4 border-t border-gray-200">
                {selectedDispute.status === 'created' && (
                  <div>
                    <h4 className="text-sm font-semibold text-gray-700 mb-2">Upload Evidence</h4>
                    <textarea
                      value={transForm.evidence_urls}
                      onChange={(e) => setTransForm(f => ({ ...f, evidence_urls: e.target.value }))}
                      placeholder="Enter evidence URLs (one per line)"
                      className="w-full border rounded-md px-3 py-2 text-sm mb-2 h-20"
                    />
                    <button onClick={() => handleTransition(selectedDispute.id, 'upload_evidence')}
                      className="px-4 py-2 bg-yellow-500 text-white rounded-md text-sm hover:bg-yellow-600">
                      Upload Evidence
                    </button>
                  </div>
                )}

                {selectedDispute.status === 'evidence_uploaded' && (
                  <div className="space-y-3">
                    <div>
                      <h4 className="text-sm font-semibold text-gray-700 mb-2">Merchant Response</h4>
                      <textarea
                        value={transForm.merchant_response}
                        onChange={(e) => setTransForm(f => ({ ...f, merchant_response: e.target.value }))}
                        placeholder="Enter merchant's response..."
                        className="w-full border rounded-md px-3 py-2 text-sm mb-2 h-20"
                      />
                      <button onClick={() => handleTransition(selectedDispute.id, 'respond')}
                        className="px-4 py-2 bg-gray-600 text-white rounded-md text-sm hover:bg-gray-700 mr-2">
                        Submit Response
                      </button>
                    </div>
                    <button onClick={() => handleTransition(selectedDispute.id, 'start_review')}
                      className="px-4 py-2 bg-blue-600 text-white rounded-md text-sm hover:bg-blue-700">
                      Start Review
                    </button>
                  </div>
                )}

                {selectedDispute.status === 'under_review' && (
                  <div className="space-y-3">
                    <textarea
                      value={transForm.merchant_response}
                      onChange={(e) => setTransForm(f => ({ ...f, merchant_response: e.target.value }))}
                      placeholder="Enter merchant's response (optional)..."
                      className="w-full border rounded-md px-3 py-2 text-sm h-16"
                    />
                    <div className="flex space-x-2">
                      {transForm.merchant_response && (
                        <button onClick={() => handleTransition(selectedDispute.id, 'respond')}
                          className="px-3 py-2 bg-gray-500 text-white rounded-md text-sm hover:bg-gray-600">
                          Record Response
                        </button>
                      )}
                      <button onClick={() => handleTransition(selectedDispute.id, 'escalate_arbitration')}
                        className="px-4 py-2 bg-purple-600 text-white rounded-md text-sm hover:bg-purple-700">
                        Escalate to Arbitration
                      </button>
                    </div>
                  </div>
                )}

                {selectedDispute.status === 'arbitration' && (
                  <div className="space-y-3">
                    <h4 className="text-sm font-semibold text-gray-700">Arbitration Decision</h4>
                    <textarea
                      value={transForm.arbitration_notes}
                      onChange={(e) => setTransForm(f => ({ ...f, arbitration_notes: e.target.value }))}
                      placeholder="Arbitration notes..."
                      className="w-full border rounded-md px-3 py-2 text-sm h-20"
                    />
                    <div className="flex items-center space-x-3">
                      <label className="text-sm text-gray-600">Outcome:</label>
                      <select
                        value={transForm.arbitration_outcome}
                        onChange={(e) => setTransForm(f => ({ ...f, arbitration_outcome: e.target.value }))}
                        className="border rounded-md px-3 py-1.5 text-sm"
                      >
                        <option value="restore">Restore review (visible)</option>
                        <option value="disclaimer">Show with disclaimer</option>
                        <option value="hide">Hide review</option>
                      </select>
                    </div>
                    <div className="flex space-x-2 pt-2">
                      <button onClick={() => handleTransition(selectedDispute.id, 'arbitrate')}
                        className="px-4 py-2 bg-green-600 text-white rounded-md text-sm hover:bg-green-700">
                        Resolve
                      </button>
                      <button onClick={() => handleTransition(selectedDispute.id, 'reject')}
                        className="px-4 py-2 bg-red-500 text-white rounded-md text-sm hover:bg-red-600">
                        Reject Dispute
                      </button>
                    </div>
                  </div>
                )}
              </div>
            </div>
          ) : (
            <div className="bg-gray-50 rounded-lg p-12 text-center text-gray-400">
              <p className="text-lg">Select a dispute to view details</p>
              <p className="text-sm mt-1">Click on a dispute from the list on the left</p>
            </div>
          )}
        </div>
      </div>

      {/* Reviews with statuses */}
      <section className="mt-10">
        <h2 className="text-lg font-semibold text-gray-900 mb-4">Vendor Reviews</h2>
        <div className="space-y-3">
          {reviews.map((review) => {
            const statusInfo = REVIEW_STATUS_ICONS[review.review_status] || REVIEW_STATUS_ICONS.visible;
            return (
              <div key={review.id} className={`bg-white rounded-lg border border-gray-200 p-4 ${review.review_status === 'hidden' ? 'opacity-50' : ''}`}>
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <div className="flex items-center space-x-2 mb-1">
                      <span className="text-sm font-semibold text-gray-800">{review.vendor_name}</span>
                      <div className="flex">
                        {Array.from({ length: 5 }, (_, i) => (
                          <span key={i} className={`text-sm ${i < review.rating ? 'text-yellow-400' : 'text-gray-300'}`}>&#9733;</span>
                        ))}
                      </div>
                      <span className={`text-xs font-medium ${statusInfo.color}`}>[{statusInfo.label}]</span>
                    </div>
                    {review.title && <h4 className="text-sm font-medium text-gray-700">{review.title}</h4>}
                    <p className="text-sm text-gray-600 mt-1">{review.body}</p>
                    {review.disclaimer_text && (
                      <p className="mt-2 text-xs text-orange-700 bg-orange-50 px-3 py-1.5 rounded italic">
                        {review.disclaimer_text}
                      </p>
                    )}
                  </div>
                </div>
              </div>
            );
          })}
          {reviews.length === 0 && (
            <div className="text-center py-8 text-gray-400">No reviews</div>
          )}
        </div>
      </section>
    </div>
  );
}
