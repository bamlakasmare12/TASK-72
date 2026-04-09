import React from 'react';
import { useAuthStore } from '../store/authStore';

export default function Dashboard() {
  const { user } = useAuthStore();
  const roles = user?.roles?.map((r) => r.name) || [];

  // Module cards: only shown to roles with access per the RBAC matrix.
  const modules = [
    {
      title: 'Learning Library',
      description: 'Browse courses, enroll in learning paths, and track your progress.',
      path: '/learning',
      roles: ['learner', 'content_moderator', 'system_admin'],
      color: 'bg-blue-50 border-blue-200 text-blue-800',
    },
    {
      title: 'Catalog & Taxonomy',
      description: 'Search the resource library and manage job/skill tags.',
      path: '/catalog',
      roles: ['learner', 'content_moderator', 'system_admin'],
      color: 'bg-teal-50 border-teal-200 text-teal-800',
    },
    {
      title: 'Procurement & Disputes',
      description: 'Manage vendor orders, reviews, and dispute arbitration.',
      path: '/procurement',
      roles: ['procurement_specialist', 'content_moderator', 'approver', 'system_admin'],
      color: 'bg-green-50 border-green-200 text-green-800',
    },
    {
      title: 'Finance & Reconciliation',
      description: 'Review settlements, cost allocation, and AR/AP entries.',
      path: '/finance',
      roles: ['finance_analyst', 'approver', 'system_admin'],
      color: 'bg-purple-50 border-purple-200 text-purple-800',
    },
    {
      title: 'Administration',
      description: 'Manage users, roles, feature flags, and system configuration.',
      path: '/admin',
      roles: ['system_admin'],
      color: 'bg-orange-50 border-orange-200 text-orange-800',
    },
  ];

  const visibleModules = modules.filter(
    (m) => m.roles.some((r) => roles.includes(r))
  );

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-gray-900">
          Welcome, {user?.display_name}
        </h1>
        <p className="text-gray-500 mt-1">
          {user?.department && `${user.department}`}
          {user?.job_family && ` - ${user.job_family}`}
        </p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {visibleModules.map((mod) => (
          <a
            key={mod.path}
            href={mod.path}
            className={`block p-6 rounded-lg border-2 ${mod.color} hover:shadow-md transition-shadow`}
          >
            <h3 className="text-lg font-semibold mb-2">{mod.title}</h3>
            <p className="text-sm opacity-80">{mod.description}</p>
          </a>
        ))}
      </div>

      {visibleModules.length === 0 && (
        <div className="text-center py-16 text-gray-400">
          <p className="text-lg">No modules available yet.</p>
          <p className="text-sm mt-2">
            {roles.length === 0
              ? 'Your account has not been assigned a role yet. Please contact an administrator.'
              : 'Contact your administrator for additional access.'}
          </p>
        </div>
      )}

      <div className="mt-8 bg-white rounded-lg shadow-sm border border-gray-200 p-6">
        <h2 className="text-lg font-semibold text-gray-800 mb-4">Your Profile</h2>
        <dl className="grid grid-cols-2 gap-4 text-sm">
          <div>
            <dt className="text-gray-500">Username</dt>
            <dd className="font-medium text-gray-900">{user?.username}</dd>
          </div>
          <div>
            <dt className="text-gray-500">Email</dt>
            <dd className="font-medium text-gray-900">{user?.email}</dd>
          </div>
          <div>
            <dt className="text-gray-500">Roles</dt>
            <dd className="font-medium text-gray-900">
              {roles.map((r) => r.replace('_', ' ')).join(', ')}
            </dd>
          </div>
          <div>
            <dt className="text-gray-500">MFA</dt>
            <dd className="font-medium">
              {user?.mfa_enabled ? (
                <span className="text-green-600">Enabled</span>
              ) : (
                <span className="text-yellow-600">Not configured</span>
              )}
            </dd>
          </div>
        </dl>
      </div>
    </div>
  );
}
