import React from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { useAuthStore } from './store/authStore';
import ErrorBoundary from './components/ErrorBoundary';
import ProtectedRoute from './components/ProtectedRoute';
import Navbar from './components/Navbar';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import AdminConfig from './pages/AdminConfig';
import Catalog from './pages/Catalog';
import LearningDashboard from './pages/LearningDashboard';
import DisputeQueue from './pages/DisputeQueue';
import Reconciliation from './pages/Reconciliation';
import Register from './pages/Register';

function AuthenticatedLayout({ children }) {
  return (
    <div className="min-h-screen bg-gray-50">
      <Navbar />
      <main>{children}</main>
    </div>
  );
}

function PlaceholderPage({ title }) {
  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <h1 className="text-2xl font-bold text-gray-900 mb-4">{title}</h1>
      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-12 text-center text-gray-400">
        <p className="text-lg">Module coming soon</p>
        <p className="text-sm mt-2">This feature is under development.</p>
      </div>
    </div>
  );
}

export default function App() {
  const { isAuthenticated, versionBlocked } = useAuthStore();

  if (versionBlocked) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <div className="max-w-md w-full bg-white rounded-lg shadow-md p-8 text-center">
          <div className="text-orange-500 text-5xl mb-4">!</div>
          <h2 className="text-xl font-semibold text-gray-800 mb-2">
            Update Required
          </h2>
          <p className="text-gray-600">
            Your application version is no longer supported. Please upgrade to
            continue.
          </p>
        </div>
      </div>
    );
  }

  return (
    <ErrorBoundary>
      <Routes>
        <Route
          path="/login"
          element={
            isAuthenticated ? <Navigate to="/" replace /> : <Login />
          }
        />

        <Route
          path="/register"
          element={
            isAuthenticated ? <Navigate to="/" replace /> : <Register />
          }
        />

        <Route
          path="/"
          element={
            <ProtectedRoute>
              <AuthenticatedLayout>
                <Dashboard />
              </AuthenticatedLayout>
            </ProtectedRoute>
          }
        />

        <Route
          path="/learning"
          element={
            <ProtectedRoute
              requiredRoles={['learner', 'content_moderator', 'system_admin']}
            >
              <AuthenticatedLayout>
                <LearningDashboard />
              </AuthenticatedLayout>
            </ProtectedRoute>
          }
        />

        <Route
          path="/catalog"
          element={
            <ProtectedRoute
              requiredRoles={['learner', 'content_moderator', 'system_admin']}
            >
              <AuthenticatedLayout>
                <Catalog />
              </AuthenticatedLayout>
            </ProtectedRoute>
          }
        />

        <Route
          path="/procurement"
          element={
            <ProtectedRoute
              requiredRoles={[
                'procurement_specialist',
                'content_moderator',
                'approver',
                'system_admin',
              ]}
            >
              <AuthenticatedLayout>
                <DisputeQueue />
              </AuthenticatedLayout>
            </ProtectedRoute>
          }
        />

        <Route
          path="/finance"
          element={
            <ProtectedRoute
              requiredRoles={['finance_analyst', 'approver', 'system_admin']}
            >
              <AuthenticatedLayout>
                <Reconciliation />
              </AuthenticatedLayout>
            </ProtectedRoute>
          }
        />

        <Route
          path="/admin"
          element={
            <ProtectedRoute requiredRoles={['system_admin']}>
              <AuthenticatedLayout>
                <AdminConfig />
              </AuthenticatedLayout>
            </ProtectedRoute>
          }
        />

        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </ErrorBoundary>
  );
}
