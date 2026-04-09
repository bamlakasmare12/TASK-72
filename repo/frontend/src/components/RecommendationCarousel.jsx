import React, { useState, useEffect } from 'react';
import client from '../api/client';
import LoadingSpinner from './LoadingSpinner';

const REASON_LABELS = {
  similar_users: 'Users like you enjoyed this',
  tag_match: 'Matches your learning interests',
  job_family: 'Recommended for your role',
  popular: 'Trending in your organization',
  cold_start: 'Popular with your job family',
};

const REASON_COLORS = {
  similar_users: 'bg-blue-50 text-blue-700 border-blue-200',
  tag_match: 'bg-green-50 text-green-700 border-green-200',
  job_family: 'bg-purple-50 text-purple-700 border-purple-200',
  popular: 'bg-orange-50 text-orange-700 border-orange-200',
  cold_start: 'bg-gray-50 text-gray-700 border-gray-200',
};

export default function RecommendationCarousel() {
  const [recommendations, setRecommendations] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [scrollIdx, setScrollIdx] = useState(0);

  useEffect(() => {
    const fetchRecs = async () => {
      setLoading(true);
      try {
        const res = await client.get('/learning/recommendations?limit=20');
        setRecommendations(res.data || []);
      } catch (err) {
        setError(err.response?.data?.message || 'Failed to load recommendations');
      } finally {
        setLoading(false);
      }
    };
    fetchRecs();
  }, []);

  if (loading) return <LoadingSpinner size="sm" text="Loading recommendations..." />;
  if (error) {
    return (
      <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-sm text-red-600">
        {error}
      </div>
    );
  }
  if (recommendations.length === 0) {
    return (
      <div className="bg-gray-50 rounded-lg p-6 text-center text-gray-400 text-sm">
        No recommendations available yet. Start exploring resources to get personalized suggestions.
      </div>
    );
  }

  const visibleCount = 4;
  const maxScroll = Math.max(0, recommendations.length - visibleCount);

  const scrollLeft = () => setScrollIdx((i) => Math.max(0, i - 1));
  const scrollRight = () => setScrollIdx((i) => Math.min(maxScroll, i + 1));

  const visible = recommendations.slice(scrollIdx, scrollIdx + visibleCount);

  return (
    <div className="relative">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold text-gray-900">
          Recommended for You
        </h2>
        <div className="flex space-x-2">
          <button
            onClick={scrollLeft}
            disabled={scrollIdx === 0}
            className="p-1.5 rounded-full border border-gray-300 disabled:opacity-30 hover:bg-gray-50 transition-colors"
          >
            <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
            </svg>
          </button>
          <button
            onClick={scrollRight}
            disabled={scrollIdx >= maxScroll}
            className="p-1.5 rounded-full border border-gray-300 disabled:opacity-30 hover:bg-gray-50 transition-colors"
          >
            <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
            </svg>
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {visible.map((rec) => (
          <RecommendationCard key={rec.resource_id} rec={rec} />
        ))}
      </div>

      {/* Scroll indicators */}
      {recommendations.length > visibleCount && (
        <div className="flex justify-center mt-4 space-x-1.5">
          {Array.from({ length: Math.ceil(recommendations.length / visibleCount) }, (_, i) => (
            <button
              key={i}
              onClick={() => setScrollIdx(i * visibleCount)}
              className={`w-2 h-2 rounded-full transition-colors ${
                Math.floor(scrollIdx / visibleCount) === i
                  ? 'bg-primary-600'
                  : 'bg-gray-300'
              }`}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function RecommendationCard({ rec }) {
  const resource = rec.resource;
  if (!resource) return null;

  const reasonLabel = REASON_LABELS[rec.reason] || 'Recommended';
  const reasonColor = REASON_COLORS[rec.reason] || REASON_COLORS.cold_start;

  return (
    <div className="bg-white rounded-lg border border-gray-200 shadow-sm hover:shadow-md transition-shadow overflow-hidden flex flex-col">
      {/* Thumbnail placeholder */}
      <div className="h-32 bg-gradient-to-br from-primary-100 to-blue-50 flex items-center justify-center">
        <span className="text-3xl text-primary-300">
          {resource.resource_type === 'video' ? (
            <svg className="h-10 w-10" fill="currentColor" viewBox="0 0 20 20">
              <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clipRule="evenodd" />
            </svg>
          ) : (
            <svg className="h-10 w-10" fill="currentColor" viewBox="0 0 20 20">
              <path d="M9 4.804A7.968 7.968 0 005.5 4c-1.255 0-2.443.29-3.5.804v10A7.969 7.969 0 015.5 14c1.669 0 3.218.51 4.5 1.385A7.962 7.962 0 0114.5 14c1.255 0 2.443.29 3.5.804v-10A7.968 7.968 0 0014.5 4c-1.255 0-2.443.29-3.5.804V12a1 1 0 11-2 0V4.804z" />
            </svg>
          )}
        </span>
      </div>

      <div className="p-4 flex-1 flex flex-col">
        {/* Why recommended badge */}
        <div
          className={`inline-flex self-start px-2 py-0.5 rounded text-[10px] font-medium border mb-2 ${reasonColor}`}
        >
          {reasonLabel}
        </div>

        <h3 className="text-sm font-semibold text-gray-900 mb-1 line-clamp-2 flex-1">
          {resource.title}
        </h3>

        {resource.description && (
          <p className="text-xs text-gray-500 line-clamp-2 mb-2">
            {resource.description}
          </p>
        )}

        <div className="flex items-center justify-between text-[10px] text-gray-400 mt-auto pt-2 border-t border-gray-100">
          <span className="capitalize">{resource.resource_type}</span>
          {resource.duration_mins && <span>{resource.duration_mins} min</span>}
          <span>{resource.view_count} views</span>
        </div>
      </div>
    </div>
  );
}
