import React, { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import client from '../api/client';

const ROLE_OPTIONS = [
  { value: '', label: 'Select your role...' },
  { value: 'learner', label: 'Learner', desc: 'Access learning paths, browse catalog, track progress' },
  { value: 'content_moderator', label: 'Content Moderator', desc: 'Manage learning content, taxonomy, and review appeals' },
  { value: 'procurement_specialist', label: 'Procurement Specialist', desc: 'Manage vendor orders, reviews, and disputes' },
  { value: 'approver', label: 'Approver', desc: 'Approve procurement requests and finance write-offs' },
  { value: 'finance_analyst', label: 'Finance Analyst', desc: 'Reconciliation, settlements, cost allocation' },
];

export default function Register() {
  const navigate = useNavigate();
  const [form, setForm] = useState({
    username: '', email: '', password: '', display_name: '',
    role: '', job_family: '', department: '', cost_center: '',
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  const handleChange = (e) => {
    setForm((f) => ({ ...f, [e.target.name]: e.target.value }));
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError('');
    setSuccess('');

    if (!form.username.trim() || !form.email.trim() || !form.password.trim() || !form.display_name.trim()) {
      setError('Username, email, password, and display name are required.');
      return;
    }
    if (form.password.length < 8) {
      setError('Password must be at least 8 characters.');
      return;
    }
    if (!form.role) {
      setError('Please select a role.');
      return;
    }

    setLoading(true);
    try {
      const payload = {
        username: form.username.trim(),
        email: form.email.trim(),
        password: form.password,
        display_name: form.display_name.trim(),
        role: form.role,
      };
      if (form.job_family.trim()) payload.job_family = form.job_family.trim();
      if (form.department.trim()) payload.department = form.department.trim();
      if (form.cost_center.trim()) payload.cost_center = form.cost_center.trim();

      const res = await client.post('/auth/register', payload);
      setSuccess(res.data.message || 'Registration successful.');
      setTimeout(() => navigate('/login'), 2000);
    } catch (err) {
      setError(err.response?.data?.message || 'Registration failed.');
    } finally {
      setLoading(false);
    }
  };

  const selectedRoleInfo = ROLE_OPTIONS.find((r) => r.value === form.role);

  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-primary-50 to-blue-100 py-8">
      <div className="max-w-md w-full">
        <div className="text-center mb-8">
          <h1 className="text-3xl font-bold text-primary-800">WLPR Portal</h1>
          <p className="text-gray-500 mt-2">Create your account</p>
        </div>

        <div className="bg-white rounded-xl shadow-lg p-8">
          <h2 className="text-xl font-semibold text-gray-800 mb-6">Register</h2>

          {error && (
            <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-md text-sm text-red-700">
              {error}
            </div>
          )}
          {success && (
            <div className="mb-4 p-3 bg-green-50 border border-green-200 rounded-md text-sm text-green-700">
              {success}
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label htmlFor="username" className="block text-sm font-medium text-gray-700 mb-1">Username</label>
              <input id="username" name="username" type="text" value={form.username} onChange={handleChange}
                className="w-full px-4 py-2.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500 outline-none"
                placeholder="Choose a username" disabled={loading} required />
            </div>
            <div>
              <label htmlFor="email" className="block text-sm font-medium text-gray-700 mb-1">Email</label>
              <input id="email" name="email" type="email" value={form.email} onChange={handleChange}
                className="w-full px-4 py-2.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500 outline-none"
                placeholder="your@email.com" disabled={loading} required />
            </div>
            <div>
              <label htmlFor="display_name" className="block text-sm font-medium text-gray-700 mb-1">Display Name</label>
              <input id="display_name" name="display_name" type="text" value={form.display_name} onChange={handleChange}
                className="w-full px-4 py-2.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500 outline-none"
                placeholder="Your full name" disabled={loading} required />
            </div>
            <div>
              <label htmlFor="password" className="block text-sm font-medium text-gray-700 mb-1">Password</label>
              <input id="password" name="password" type="password" value={form.password} onChange={handleChange}
                className="w-full px-4 py-2.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500 outline-none"
                placeholder="Min. 8 characters" disabled={loading} required minLength={8} />
            </div>

            <div>
              <label htmlFor="role" className="block text-sm font-medium text-gray-700 mb-1">Role</label>
              <select id="role" name="role" value={form.role} onChange={handleChange}
                className="w-full px-4 py-2.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500 outline-none bg-white"
                disabled={loading} required>
                {ROLE_OPTIONS.map((opt) => (
                  <option key={opt.value} value={opt.value} disabled={opt.value === ''}>
                    {opt.label}
                  </option>
                ))}
              </select>
              {selectedRoleInfo && selectedRoleInfo.desc && (
                <p className="mt-1 text-xs text-gray-500">{selectedRoleInfo.desc}</p>
              )}
            </div>

            <div className="grid grid-cols-3 gap-3">
              <div>
                <label htmlFor="job_family" className="block text-xs font-medium text-gray-500 mb-1">Job Family</label>
                <input id="job_family" name="job_family" type="text" value={form.job_family} onChange={handleChange}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-primary-500 outline-none"
                  placeholder="Optional" disabled={loading} />
              </div>
              <div>
                <label htmlFor="department" className="block text-xs font-medium text-gray-500 mb-1">Department</label>
                <input id="department" name="department" type="text" value={form.department} onChange={handleChange}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-primary-500 outline-none"
                  placeholder="Optional" disabled={loading} />
              </div>
              <div>
                <label htmlFor="cost_center" className="block text-xs font-medium text-gray-500 mb-1">Cost Center</label>
                <input id="cost_center" name="cost_center" type="text" value={form.cost_center} onChange={handleChange}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-primary-500 outline-none"
                  placeholder="Optional" disabled={loading} />
              </div>
            </div>

            <button type="submit" disabled={loading}
              className="w-full bg-primary-600 text-white py-2.5 rounded-lg font-medium hover:bg-primary-700 focus:ring-4 focus:ring-primary-200 disabled:opacity-50 disabled:cursor-not-allowed transition-all">
              {loading ? (
                <span className="flex items-center justify-center">
                  <span className="h-4 w-4 border-2 border-white border-t-transparent rounded-full animate-spin mr-2" />
                  Registering...
                </span>
              ) : 'Register'}
            </button>
          </form>

          <div className="mt-6 pt-4 border-t border-gray-100 text-center">
            <p className="text-sm text-gray-500">
              Already have an account?{' '}
              <Link to="/login" className="text-primary-600 font-medium hover:text-primary-800">
                Sign in
              </Link>
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
