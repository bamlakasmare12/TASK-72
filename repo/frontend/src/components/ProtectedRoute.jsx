import React from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { useAuthStore } from '../store/authStore';

export default function ProtectedRoute({ children, requiredRoles, requiredPermissions }) {
  const { isAuthenticated, user } = useAuthStore();
  const location = useLocation();

  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  if (requiredRoles && requiredRoles.length > 0) {
    const userRoles = user?.roles?.map((r) => r.name) || [];
    const hasRole = requiredRoles.some((r) => userRoles.includes(r));
    if (!hasRole) {
      // Feature invisibility: user should not know this page exists
      return <NotFoundPage />;
    }
  }

  if (requiredPermissions && requiredPermissions.length > 0) {
    const userPerms = user?.permissions || [];
    const hasPerm = requiredPermissions.some((p) => userPerms.includes(p));
    if (!hasPerm) {
      return <NotFoundPage />;
    }
  }

  return children;
}

function NotFoundPage() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="max-w-md w-full bg-white rounded-lg shadow-md p-8 text-center">
        <div className="text-gray-400 text-5xl mb-4">404</div>
        <h2 className="text-xl font-semibold text-gray-800 mb-2">Page Not Found</h2>
        <p className="text-gray-600 mb-6">
          The page you are looking for does not exist.
        </p>
        <a
          href="/"
          className="inline-block bg-primary-600 text-white px-6 py-2 rounded-md hover:bg-primary-700 transition-colors"
        >
          Go to Dashboard
        </a>
      </div>
    </div>
  );
}
