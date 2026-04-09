import React from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAuthStore } from '../store/authStore';

// NAV_ITEMS: each item only visible to roles with access per the RBAC matrix.
// Users without access do not see the item — feature invisibility.
const NAV_ITEMS = [
  {
    label: 'Dashboard',
    path: '/',
    roles: null, // all authenticated users
  },
  {
    label: 'Catalog',
    path: '/catalog',
    roles: ['learner', 'content_moderator', 'system_admin'],
  },
  {
    label: 'Learning',
    path: '/learning',
    roles: ['learner', 'content_moderator', 'system_admin'],
  },
  {
    label: 'Procurement',
    path: '/procurement',
    roles: ['procurement_specialist', 'content_moderator', 'approver', 'system_admin'],
  },
  {
    label: 'Finance',
    path: '/finance',
    roles: ['finance_analyst', 'approver', 'system_admin'],
  },
  {
    label: 'Admin',
    path: '/admin',
    roles: ['system_admin'],
  },
];

export default function Navbar() {
  const { user, logout, deprecationWarning } = useAuthStore();
  const navigate = useNavigate();

  const userRoles = user?.roles?.map((r) => r.name) || [];

  const visibleItems = NAV_ITEMS.filter(
    (item) =>
      !item.roles || item.roles.some((role) => userRoles.includes(role))
  );

  const handleLogout = async () => {
    await logout();
    navigate('/login');
  };

  return (
    <nav className="bg-white border-b border-gray-200 shadow-sm">
      {deprecationWarning && (
        <div className="bg-yellow-50 border-b border-yellow-200 px-4 py-2 text-center text-sm text-yellow-800">
          Your client version is outdated. Please upgrade to v{deprecationWarning} or later.
          Write operations may be restricted.
        </div>
      )}
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex justify-between h-16">
          <div className="flex items-center space-x-8">
            <Link to="/" className="text-xl font-bold text-primary-700">
              WLPR Portal
            </Link>
            <div className="hidden md:flex space-x-1">
              {visibleItems.map((item) => (
                <Link
                  key={item.path}
                  to={item.path}
                  className="px-3 py-2 rounded-md text-sm font-medium text-gray-600 hover:text-primary-700 hover:bg-primary-50 transition-colors"
                >
                  {item.label}
                </Link>
              ))}
            </div>
          </div>
          <div className="flex items-center space-x-4">
            <div className="text-sm text-gray-600">
              <span className="font-medium">{user?.display_name}</span>
              <span className="ml-2 text-xs text-gray-400">
                {userRoles.join(', ')}
              </span>
            </div>
            <button
              onClick={handleLogout}
              className="px-3 py-1.5 text-sm text-red-600 border border-red-300 rounded-md hover:bg-red-50 transition-colors"
            >
              Logout
            </button>
          </div>
        </div>
      </div>
    </nav>
  );
}
