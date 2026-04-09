import React, { useEffect, useState } from 'react';
import { useConfigStore } from '../store/configStore';
import LoadingSpinner from '../components/LoadingSpinner';

export default function AdminConfig() {
  const {
    configs, flags, loading, error,
    fetchConfigs, fetchFlags, updateConfig, updateFlag,
  } = useConfigStore();

  const [editingConfig, setEditingConfig] = useState(null);
  const [editValue, setEditValue] = useState('');
  const [saveError, setSaveError] = useState('');

  useEffect(() => {
    fetchConfigs();
    fetchFlags();
  }, [fetchConfigs, fetchFlags]);

  const handleSaveConfig = async (key) => {
    setSaveError('');
    try {
      await updateConfig(key, editValue);
      setEditingConfig(null);
    } catch (err) {
      setSaveError(err.message);
    }
  };

  const handleToggleFlag = async (key, currentEnabled) => {
    try {
      await updateFlag(key, { enabled: !currentEnabled });
    } catch (err) {
      setSaveError(err.message);
    }
  };

  if (loading && configs.length === 0) {
    return <LoadingSpinner text="Loading configuration..." />;
  }

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <h1 className="text-2xl font-bold text-gray-900 mb-8">
        Configuration Center
      </h1>

      {error && (
        <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-md text-sm text-red-700">
          {error}
        </div>
      )}

      {saveError && (
        <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-md text-sm text-red-700">
          {saveError}
        </div>
      )}

      {/* System Configs */}
      <section className="mb-10">
        <h2 className="text-lg font-semibold text-gray-800 mb-4">
          System Parameters
        </h2>
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 overflow-hidden">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Key</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Value</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Module</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {configs.map((cfg) => (
                <tr key={cfg.key} className="hover:bg-gray-50">
                  <td className="px-6 py-4 text-sm font-mono text-gray-800">
                    {cfg.key}
                    <div className="text-xs text-gray-400">{cfg.description}</div>
                  </td>
                  <td className="px-6 py-4 text-sm">
                    {editingConfig === cfg.key ? (
                      <input
                        type="text"
                        value={editValue}
                        onChange={(e) => setEditValue(e.target.value)}
                        className="border border-gray-300 rounded px-2 py-1 text-sm w-40 focus:ring-2 focus:ring-primary-500 outline-none"
                        autoFocus
                      />
                    ) : (
                      <span className="font-medium text-gray-900">{cfg.value}</span>
                    )}
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500">{cfg.module}</td>
                  <td className="px-6 py-4 text-sm">
                    {editingConfig === cfg.key ? (
                      <div className="flex space-x-2">
                        <button
                          onClick={() => handleSaveConfig(cfg.key)}
                          className="text-green-600 hover:text-green-800 font-medium"
                        >
                          Save
                        </button>
                        <button
                          onClick={() => setEditingConfig(null)}
                          className="text-gray-500 hover:text-gray-700"
                        >
                          Cancel
                        </button>
                      </div>
                    ) : (
                      <button
                        onClick={() => {
                          setEditingConfig(cfg.key);
                          setEditValue(cfg.value);
                        }}
                        className="text-primary-600 hover:text-primary-800 font-medium"
                      >
                        Edit
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {configs.length === 0 && (
            <div className="py-8 text-center text-gray-400">
              No configurations found.
            </div>
          )}
        </div>
      </section>

      {/* Feature Flags */}
      <section>
        <h2 className="text-lg font-semibold text-gray-800 mb-4">
          Feature Flags
        </h2>
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 overflow-hidden">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Flag</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Strategy</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Toggle</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {flags.map((flag) => (
                <tr key={flag.key} className="hover:bg-gray-50">
                  <td className="px-6 py-4 text-sm font-mono text-gray-800">
                    {flag.key}
                    <div className="text-xs text-gray-400">{flag.description}</div>
                  </td>
                  <td className="px-6 py-4">
                    <span
                      className={`inline-flex px-2 py-1 text-xs font-medium rounded-full ${
                        flag.enabled
                          ? 'bg-green-100 text-green-800'
                          : 'bg-gray-100 text-gray-600'
                      }`}
                    >
                      {flag.enabled ? 'Enabled' : 'Disabled'}
                    </span>
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500">
                    {flag.rollout_strategy}
                    {flag.rollout_strategy === 'percentage' &&
                      ` (${flag.rollout_percentage}%)`}
                  </td>
                  <td className="px-6 py-4">
                    <button
                      onClick={() => handleToggleFlag(flag.key, flag.enabled)}
                      className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                        flag.enabled ? 'bg-primary-600' : 'bg-gray-300'
                      }`}
                    >
                      <span
                        className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                          flag.enabled ? 'translate-x-6' : 'translate-x-1'
                        }`}
                      />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {flags.length === 0 && (
            <div className="py-8 text-center text-gray-400">
              No feature flags configured.
            </div>
          )}
        </div>
      </section>
    </div>
  );
}
