import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import client from '../api/client';
import LoadingSpinner from '../components/LoadingSpinner';

const TYPE_COLORS = {
  course: 'bg-blue-100 text-blue-700',
  article: 'bg-green-100 text-green-700',
  video: 'bg-purple-100 text-purple-700',
  document: 'bg-yellow-100 text-yellow-700',
  assessment: 'bg-red-100 text-red-700',
  link: 'bg-gray-100 text-gray-700',
};

export default function ResourceDetail() {
  const { id } = useParams();
  const navigate = useNavigate();
  const [resource, setResource] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    const fetchResource = async () => {
      setLoading(true);
      setError(null);
      try {
        const res = await client.get(`/resources/${id}`);
        setResource(res.data);
      } catch (err) {
        if (err.response?.status === 404) {
          setError('Resource not found');
        } else {
          setError(err.response?.data?.message || 'Failed to load resource');
        }
      } finally {
        setLoading(false);
      }
    };
    fetchResource();
  }, [id]);

  if (loading) return <LoadingSpinner text="Loading resource..." />;

  if (error) {
    return (
      <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="bg-red-50 border border-red-200 rounded-lg p-8 text-center">
          <p className="text-red-600 font-medium text-lg">{error}</p>
          <button
            onClick={() => navigate('/catalog')}
            className="mt-4 px-4 py-2 text-sm text-primary-600 border border-primary-300 rounded-md hover:bg-primary-50"
          >
            Back to Catalog
          </button>
        </div>
      </div>
    );
  }

  if (!resource) return null;

  const typeColor = TYPE_COLORS[resource.resource_type] || TYPE_COLORS.link;

  return (
    <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <button
        onClick={() => navigate('/catalog')}
        className="mb-4 text-sm text-gray-500 hover:text-primary-600 flex items-center"
      >
        <svg className="h-4 w-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
        </svg>
        Back to Catalog
      </button>

      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
        <div className="flex items-center space-x-3 mb-4">
          <span className={`inline-flex px-2.5 py-1 rounded text-xs font-medium ${typeColor}`}>
            {resource.resource_type}
          </span>
          {resource.difficulty && (
            <span className="text-sm text-gray-400 capitalize">{resource.difficulty}</span>
          )}
          {resource.category_name && (
            <span className="text-sm text-gray-400">{resource.category_name}</span>
          )}
        </div>

        <h1 className="text-2xl font-bold text-gray-900 mb-3">{resource.title}</h1>

        {resource.description && (
          <p className="text-gray-600 mb-6 leading-relaxed">{resource.description}</p>
        )}

        {resource.tags && resource.tags.length > 0 && (
          <div className="flex flex-wrap gap-2 mb-6">
            {resource.tags.map((tag) => (
              <span
                key={tag.id}
                className="px-2.5 py-1 bg-gray-100 text-gray-600 rounded text-sm"
              >
                {tag.name}
              </span>
            ))}
          </div>
        )}

        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 pt-4 border-t border-gray-200">
          {resource.duration_mins > 0 && (
            <div>
              <p className="text-xs text-gray-400 uppercase">Duration</p>
              <p className="text-sm font-medium text-gray-700">{resource.duration_mins} min</p>
            </div>
          )}
          <div>
            <p className="text-xs text-gray-400 uppercase">Views</p>
            <p className="text-sm font-medium text-gray-700">{resource.view_count || 0}</p>
          </div>
          <div>
            <p className="text-xs text-gray-400 uppercase">Completions</p>
            <p className="text-sm font-medium text-gray-700">{resource.completion_count || 0}</p>
          </div>
          {resource.published_at && (
            <div>
              <p className="text-xs text-gray-400 uppercase">Published</p>
              <p className="text-sm font-medium text-gray-700">
                {new Date(resource.published_at).toLocaleDateString()}
              </p>
            </div>
          )}
        </div>

        {resource.url && (
          <div className="mt-6">
            <a
              href={resource.url}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center px-4 py-2 bg-primary-600 text-white rounded-md text-sm hover:bg-primary-700 transition-colors"
            >
              Open Resource
              <svg className="h-4 w-4 ml-1.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
              </svg>
            </a>
          </div>
        )}
      </div>
    </div>
  );
}
