import React, { useState, useEffect, useCallback } from 'react';
import client from '../api/client';
import { useAuthStore } from '../store/authStore';
import RecommendationCarousel from '../components/RecommendationCarousel';
import ProgressBar from '../components/ProgressBar';
import LoadingSpinner from '../components/LoadingSpinner';

export default function LearningDashboard() {
  const { user } = useAuthStore();
  const [enrollments, setEnrollments] = useState([]);
  const [allPaths, setAllPaths] = useState([]);
  const [selectedEnrollment, setSelectedEnrollment] = useState(null);
  const [enrollmentDetail, setEnrollmentDetail] = useState(null);
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [error, setError] = useState(null);
  const [actionLoading, setActionLoading] = useState(false);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [enrollRes, pathsRes] = await Promise.all([
        client.get('/learning/enrollments'),
        client.get('/learning/paths'),
      ]);
      setEnrollments(enrollRes.data || []);
      setAllPaths(pathsRes.data || []);
    } catch (err) {
      setError(err.response?.data?.message || 'Failed to load learning data');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const loadEnrollmentDetail = useCallback(async (pathId) => {
    setDetailLoading(true);
    try {
      const res = await client.get(`/learning/enrollments/${pathId}`);
      setEnrollmentDetail(res.data);
      setSelectedEnrollment(pathId);
    } catch (err) {
      setError(err.response?.data?.message || 'Failed to load enrollment details');
    } finally {
      setDetailLoading(false);
    }
  }, []);

  const handleEnroll = async (pathId) => {
    setActionLoading(true);
    try {
      await client.post('/learning/enroll', { path_id: pathId });
      await fetchData();
    } catch (err) {
      setError(err.response?.data?.message || 'Enrollment failed');
    } finally {
      setActionLoading(false);
    }
  };

  const handleDrop = async (pathId) => {
    setActionLoading(true);
    try {
      await client.delete(`/learning/enroll/${pathId}`);
      setSelectedEnrollment(null);
      setEnrollmentDetail(null);
      await fetchData();
    } catch (err) {
      setError(err.response?.data?.message || 'Failed to drop enrollment');
    } finally {
      setActionLoading(false);
    }
  };

  const handleUpdateProgress = async (resourceId, pathId, status, progressPct) => {
    try {
      await client.put('/learning/progress', {
        resource_id: resourceId,
        path_id: pathId,
        status,
        progress_pct: progressPct,
        time_spent_mins: 5,
      });
      if (selectedEnrollment) {
        await loadEnrollmentDetail(selectedEnrollment);
      }
    } catch (err) {
      setError(err.response?.data?.message || 'Failed to update progress');
    }
  };

  const handleExportCSV = async () => {
    try {
      const res = await client.get('/learning/export', { responseType: 'blob' });
      const url = window.URL.createObjectURL(new Blob([res.data]));
      const link = document.createElement('a');
      link.href = url;
      link.setAttribute('download', 'learning_records.csv');
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.URL.revokeObjectURL(url);
    } catch (err) {
      setError('Failed to export CSV');
    }
  };

  const enrolledPathIds = new Set(
    enrollments.filter((e) => e.status === 'active').map((e) => e.path_id)
  );

  if (loading) return <LoadingSpinner text="Loading your learning dashboard..." />;

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      {error && (
        <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-md text-sm text-red-700">
          {error}
          <button onClick={() => setError(null)} className="ml-2 text-red-500 hover:text-red-800 font-medium">
            Dismiss
          </button>
        </div>
      )}

      {/* Recommendations */}
      <section className="mb-10">
        <RecommendationCarousel />
      </section>

      {/* My Enrollments */}
      <section className="mb-10">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-gray-900">My Learning Paths</h2>
          <button
            onClick={handleExportCSV}
            className="text-sm text-primary-600 hover:text-primary-800 font-medium border border-primary-300 px-3 py-1.5 rounded-md hover:bg-primary-50 transition-colors"
          >
            Export CSV
          </button>
        </div>

        {enrollments.length === 0 ? (
          <div className="bg-gray-50 rounded-lg p-8 text-center text-gray-400">
            <p className="text-lg">No active enrollments</p>
            <p className="text-sm mt-1">Browse available paths below to get started</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {enrollments.map((enrollment) => (
              <EnrollmentCard
                key={enrollment.id}
                enrollment={enrollment}
                allPaths={allPaths}
                isSelected={selectedEnrollment === enrollment.path_id}
                onSelect={() => loadEnrollmentDetail(enrollment.path_id)}
                onDrop={() => handleDrop(enrollment.path_id)}
              />
            ))}
          </div>
        )}
      </section>

      {/* Enrollment Detail */}
      {selectedEnrollment && (
        <section className="mb-10">
          {detailLoading ? (
            <LoadingSpinner text="Loading path details..." />
          ) : enrollmentDetail ? (
            <EnrollmentDetailView
              detail={enrollmentDetail}
              onUpdateProgress={handleUpdateProgress}
            />
          ) : null}
        </section>
      )}

      {/* Available Paths */}
      <section>
        <h2 className="text-lg font-semibold text-gray-900 mb-4">Available Learning Paths</h2>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {allPaths
            .filter((p) => !enrolledPathIds.has(p.id))
            .map((path) => (
              <PathCard
                key={path.id}
                path={path}
                onEnroll={() => handleEnroll(path.id)}
                loading={actionLoading}
              />
            ))}
          {allPaths.filter((p) => !enrolledPathIds.has(p.id)).length === 0 && (
            <div className="col-span-full bg-gray-50 rounded-lg p-6 text-center text-gray-400 text-sm">
              You are enrolled in all available paths.
            </div>
          )}
        </div>
      </section>
    </div>
  );
}

function EnrollmentCard({ enrollment, allPaths, isSelected, onSelect, onDrop }) {
  const path = allPaths.find((p) => p.id === enrollment.path_id);
  const statusColors = {
    active: 'bg-blue-100 text-blue-700',
    completed: 'bg-green-100 text-green-700',
    dropped: 'bg-gray-100 text-gray-500',
  };

  return (
    <div
      className={`bg-white rounded-lg border-2 p-4 cursor-pointer transition-all ${
        isSelected ? 'border-primary-500 shadow-md' : 'border-gray-200 hover:border-gray-300'
      }`}
      onClick={onSelect}
    >
      <div className="flex items-start justify-between mb-2">
        <h3 className="text-sm font-semibold text-gray-900">
          {path?.title || `Path #${enrollment.path_id}`}
        </h3>
        <span className={`px-2 py-0.5 rounded text-xs font-medium ${statusColors[enrollment.status]}`}>
          {enrollment.status}
        </span>
      </div>
      {path && (
        <>
          <p className="text-xs text-gray-500 mb-3">
            {path.required_count} required + {path.elective_min} electives
          </p>
          {path.estimated_hours && (
            <p className="text-xs text-gray-400 mb-2">~{path.estimated_hours}h estimated</p>
          )}
        </>
      )}
      <p className="text-[10px] text-gray-400">
        Enrolled: {new Date(enrollment.enrolled_at).toLocaleDateString()}
      </p>
      {enrollment.status === 'active' && (
        <button
          onClick={(e) => {
            e.stopPropagation();
            onDrop();
          }}
          className="mt-2 text-xs text-red-500 hover:text-red-700"
        >
          Drop
        </button>
      )}
    </div>
  );
}

function EnrollmentDetailView({ detail, onUpdateProgress }) {
  const { path, required_items, elective_items, required_complete, elective_complete, is_path_complete } = detail;

  return (
    <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
      <div className="flex items-center justify-between mb-4">
        <div>
          <h3 className="text-lg font-semibold text-gray-900">{path.title}</h3>
          {path.description && (
            <p className="text-sm text-gray-500 mt-1">{path.description}</p>
          )}
        </div>
        {is_path_complete && (
          <span className="px-4 py-1.5 bg-green-100 text-green-700 rounded-full text-sm font-medium">
            Path Completed
          </span>
        )}
      </div>

      {/* Required items */}
      <div className="mb-6">
        <div className="mb-3">
          <ProgressBar
            current={required_complete}
            total={path.required_count}
            label={`Required Items (${path.required_count} needed)`}
            color="primary"
          />
        </div>
        <div className="space-y-2">
          {(required_items || []).map((pip) => (
            <PathItemRow
              key={pip.item.id}
              pip={pip}
              pathId={path.id}
              onUpdateProgress={onUpdateProgress}
            />
          ))}
        </div>
      </div>

      {/* Elective items */}
      <div>
        <div className="mb-3">
          <ProgressBar
            current={elective_complete}
            total={path.elective_min}
            label={`Electives (${path.elective_min} needed)`}
            color="green"
          />
        </div>
        <div className="space-y-2">
          {(elective_items || []).map((pip) => (
            <PathItemRow
              key={pip.item.id}
              pip={pip}
              pathId={path.id}
              onUpdateProgress={onUpdateProgress}
            />
          ))}
        </div>
      </div>
    </div>
  );
}

function PathItemRow({ pip, pathId, onUpdateProgress }) {
  const { item, progress } = pip;
  const resource = item.resource;
  const status = progress?.status || 'not_started';
  const pct = progress?.progress_pct || 0;

  const statusIcons = {
    not_started: 'bg-gray-200',
    in_progress: 'bg-yellow-400',
    completed: 'bg-green-500',
  };

  return (
    <div className="flex items-center space-x-3 p-3 bg-gray-50 rounded-lg">
      <div className={`w-3 h-3 rounded-full flex-shrink-0 ${statusIcons[status]}`} />
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium text-gray-800 truncate">
          {resource?.title || `Resource #${item.resource_id}`}
        </p>
        <div className="flex items-center space-x-3 mt-1">
          <span className="text-xs text-gray-400 capitalize">{item.item_type}</span>
          {resource?.duration_mins && (
            <span className="text-xs text-gray-400">{resource.duration_mins} min</span>
          )}
          <span className="text-xs text-gray-400">{pct}% complete</span>
        </div>
        {status === 'in_progress' && (
          <div className="mt-1.5 w-32">
            <div className="w-full bg-gray-200 rounded-full h-1.5">
              <div
                className="bg-yellow-400 h-1.5 rounded-full transition-all"
                style={{ width: `${pct}%` }}
              />
            </div>
          </div>
        )}
      </div>
      <div className="flex-shrink-0 flex space-x-2">
        {status === 'not_started' && (
          <button
            onClick={() => onUpdateProgress(item.resource_id, pathId, 'in_progress', 10)}
            className="px-3 py-1 text-xs bg-primary-600 text-white rounded-md hover:bg-primary-700 transition-colors"
          >
            Start
          </button>
        )}
        {status === 'in_progress' && (
          <>
            <button
              onClick={() => onUpdateProgress(item.resource_id, pathId, 'in_progress', Math.min(pct + 25, 99))}
              className="px-3 py-1 text-xs bg-yellow-500 text-white rounded-md hover:bg-yellow-600 transition-colors"
            >
              Resume
            </button>
            <button
              onClick={() => onUpdateProgress(item.resource_id, pathId, 'completed', 100)}
              className="px-3 py-1 text-xs bg-green-600 text-white rounded-md hover:bg-green-700 transition-colors"
            >
              Complete
            </button>
          </>
        )}
        {status === 'completed' && (
          <span className="px-3 py-1 text-xs bg-green-100 text-green-700 rounded-md font-medium">
            Done
          </span>
        )}
      </div>
    </div>
  );
}

function PathCard({ path, onEnroll, loading }) {
  return (
    <div className="bg-white rounded-lg border border-gray-200 shadow-sm p-5 hover:shadow-md transition-shadow">
      <h3 className="text-sm font-semibold text-gray-900 mb-2">{path.title}</h3>
      {path.description && (
        <p className="text-xs text-gray-500 mb-3 line-clamp-2">{path.description}</p>
      )}
      <div className="flex items-center space-x-3 text-xs text-gray-400 mb-4">
        <span>{path.required_count} required</span>
        <span>{path.elective_min} electives min</span>
        {path.estimated_hours && <span>~{path.estimated_hours}h</span>}
        {path.difficulty && <span className="capitalize">{path.difficulty}</span>}
      </div>
      {path.category_name && (
        <span className="inline-block px-2 py-0.5 bg-gray-100 text-gray-600 rounded text-xs mb-3">
          {path.category_name}
        </span>
      )}
      <button
        onClick={onEnroll}
        disabled={loading}
        className="w-full mt-2 px-4 py-2 text-sm bg-primary-600 text-white rounded-md hover:bg-primary-700 disabled:opacity-50 transition-colors"
      >
        {loading ? 'Enrolling...' : 'Enroll'}
      </button>
    </div>
  );
}
