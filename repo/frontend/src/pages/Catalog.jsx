import React, { useState, useEffect, useCallback, useRef } from 'react';
import { Link } from 'react-router-dom';
import client from '../api/client';
import SearchFilters from '../components/SearchFilters';
import LoadingSpinner from '../components/LoadingSpinner';

const DEBOUNCE_MS = 350;

const TYPE_COLORS = {
  course: 'bg-blue-100 text-blue-700',
  article: 'bg-green-100 text-green-700',
  video: 'bg-purple-100 text-purple-700',
  document: 'bg-yellow-100 text-yellow-700',
  assessment: 'bg-red-100 text-red-700',
  link: 'bg-gray-100 text-gray-700',
};

export default function Catalog() {
  const [filters, setFilters] = useState({
    q: '',
    categories: [],
    tags: [],
    date_from: '',
    date_to: '',
    difficulty: '',
    type: '',
    sort_by: 'relevance',
    page: 1,
    page_size: 20,
  });

  const [results, setResults] = useState([]);
  const [total, setTotal] = useState(0);
  const [synonymsApplied, setSynonymsApplied] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const debounceRef = useRef(null);

  const fetchResults = useCallback(async (currentFilters) => {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams();
      if (currentFilters.q) params.set('q', currentFilters.q);
      if (currentFilters.categories?.length)
        params.set('categories', currentFilters.categories.join(','));
      if (currentFilters.tags?.length)
        params.set('tags', currentFilters.tags.join(','));
      if (currentFilters.date_from) params.set('date_from', currentFilters.date_from);
      if (currentFilters.date_to) params.set('date_to', currentFilters.date_to);
      if (currentFilters.difficulty) params.set('difficulty', currentFilters.difficulty);
      if (currentFilters.type) params.set('type', currentFilters.type);
      if (currentFilters.sort_by) params.set('sort_by', currentFilters.sort_by);
      params.set('page', String(currentFilters.page || 1));
      params.set('page_size', String(currentFilters.page_size || 20));

      const res = await client.get(`/search?${params.toString()}`);
      setResults(res.data.results || []);
      setTotal(res.data.total || 0);
      setSynonymsApplied(res.data.synonyms_applied || []);
    } catch (err) {
      setError(err.response?.data?.message || 'Search failed');
      setResults([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  }, []);

  // Debounced search on query text changes
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      fetchResults(filters);
    }, DEBOUNCE_MS);
    return () => clearTimeout(debounceRef.current);
  }, [filters, fetchResults]);

  const handleFilterChange = (newFilters) => {
    setFilters({ ...newFilters, page: 1 });
  };

  const handlePageChange = (newPage) => {
    setFilters((prev) => ({ ...prev, page: newPage }));
  };

  const totalPages = Math.ceil(total / (filters.page_size || 20));

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900 mb-4">Learning Library</h1>

        {/* Search bar */}
        <div className="relative">
          <input
            type="text"
            value={filters.q}
            onChange={(e) =>
              setFilters((prev) => ({ ...prev, q: e.target.value, page: 1 }))
            }
            placeholder="Search resources by title, description, or skills... (supports typo tolerance and pinyin)"
            className="w-full px-4 py-3 pl-10 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-primary-500 focus:border-primary-500 outline-none transition-all"
          />
          <svg
            className="absolute left-3 top-3.5 h-4 w-4 text-gray-400"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
            />
          </svg>
          {loading && (
            <div className="absolute right-3 top-3.5">
              <div className="h-4 w-4 border-2 border-primary-600 border-t-transparent rounded-full animate-spin" />
            </div>
          )}
        </div>
      </div>

      {/* Filters */}
      <SearchFilters
        filters={filters}
        onFilterChange={handleFilterChange}
        synonymsApplied={synonymsApplied}
      />

      {/* Results */}
      <div className="mt-6">
        {/* Status bar */}
        {!loading && !error && (
          <p className="text-sm text-gray-500 mb-4">
            {total > 0
              ? `Showing ${(filters.page - 1) * filters.page_size + 1}-${Math.min(
                  filters.page * filters.page_size,
                  total
                )} of ${total} results`
              : filters.q
              ? 'No results found'
              : 'Browse all available resources'}
          </p>
        )}

        {/* Error state */}
        {error && (
          <div className="bg-red-50 border border-red-200 rounded-lg p-6 text-center">
            <p className="text-red-600 font-medium">Search Error</p>
            <p className="text-sm text-red-500 mt-1">{error}</p>
            <button
              onClick={() => fetchResults(filters)}
              className="mt-3 text-sm text-primary-600 hover:text-primary-800 font-medium"
            >
              Try again
            </button>
          </div>
        )}

        {/* Loading state */}
        {loading && results.length === 0 && (
          <LoadingSpinner text="Searching..." />
        )}

        {/* Empty state */}
        {!loading && !error && results.length === 0 && filters.q && (
          <div className="bg-gray-50 rounded-lg p-12 text-center">
            <p className="text-gray-500 text-lg">No resources found</p>
            <p className="text-sm text-gray-400 mt-2">
              Try adjusting your search terms or filters
            </p>
          </div>
        )}

        {/* Result cards */}
        <div className="space-y-4">
          {results.map((resource) => (
            <ResourceCard key={resource.id} resource={resource} />
          ))}
        </div>

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="mt-6 flex items-center justify-center space-x-2">
            <button
              disabled={filters.page <= 1}
              onClick={() => handlePageChange(filters.page - 1)}
              className="px-3 py-1.5 text-sm border border-gray-300 rounded-md disabled:opacity-50 hover:bg-gray-50 transition-colors"
            >
              Previous
            </button>
            {Array.from({ length: Math.min(totalPages, 7) }, (_, i) => {
              let pageNum;
              if (totalPages <= 7) {
                pageNum = i + 1;
              } else if (filters.page <= 4) {
                pageNum = i + 1;
              } else if (filters.page >= totalPages - 3) {
                pageNum = totalPages - 6 + i;
              } else {
                pageNum = filters.page - 3 + i;
              }
              return (
                <button
                  key={pageNum}
                  onClick={() => handlePageChange(pageNum)}
                  className={`px-3 py-1.5 text-sm rounded-md transition-colors ${
                    filters.page === pageNum
                      ? 'bg-primary-600 text-white'
                      : 'border border-gray-300 hover:bg-gray-50'
                  }`}
                >
                  {pageNum}
                </button>
              );
            })}
            <button
              disabled={filters.page >= totalPages}
              onClick={() => handlePageChange(filters.page + 1)}
              className="px-3 py-1.5 text-sm border border-gray-300 rounded-md disabled:opacity-50 hover:bg-gray-50 transition-colors"
            >
              Next
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

function ResourceCard({ resource }) {
  const typeColor = TYPE_COLORS[resource.resource_type] || TYPE_COLORS.link;

  return (
    <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-5 hover:shadow-md transition-shadow">
      <div className="flex items-start justify-between">
        <div className="flex-1">
          <div className="flex items-center space-x-2 mb-2">
            <span
              className={`inline-flex px-2 py-0.5 rounded text-xs font-medium ${typeColor}`}
            >
              {resource.resource_type}
            </span>
            {resource.difficulty && (
              <span className="text-xs text-gray-400 capitalize">
                {resource.difficulty}
              </span>
            )}
            {resource.category_name && (
              <span className="text-xs text-gray-400">
                {resource.category_name}
              </span>
            )}
          </div>

          <h3 className="text-lg font-semibold text-gray-900 mb-1">
            <Link
              to={`/resources/${resource.id}`}
              className="hover:text-primary-600 transition-colors"
            >
              {resource.title}
            </Link>
          </h3>

          {resource.description && (
            <p className="text-sm text-gray-600 line-clamp-2 mb-3">
              {resource.description}
            </p>
          )}

          {/* Tags */}
          {resource.tags && resource.tags.length > 0 && (
            <div className="flex flex-wrap gap-1.5 mb-3">
              {resource.tags.map((tag) => (
                <span
                  key={tag.id}
                  className="px-2 py-0.5 bg-gray-100 text-gray-600 rounded text-xs"
                >
                  {tag.name}
                </span>
              ))}
            </div>
          )}

          {/* Meta */}
          <div className="flex items-center space-x-4 text-xs text-gray-400">
            {resource.duration_mins && (
              <span>{resource.duration_mins} min</span>
            )}
            <span>{resource.view_count} views</span>
            <span>{resource.completion_count} completions</span>
            {resource.published_at && (
              <span>
                {new Date(resource.published_at).toLocaleDateString()}
              </span>
            )}
            {resource.search_rank > 0 && (
              <span className="text-primary-500">
                relevance: {(resource.search_rank * 100).toFixed(0)}%
              </span>
            )}
          </div>
        </div>

        {/* Popularity badge */}
        <div className="ml-4 text-center">
          <div
            className={`w-12 h-12 rounded-full flex items-center justify-center text-sm font-bold ${
              resource.popularity_score >= 80
                ? 'bg-green-100 text-green-700'
                : resource.popularity_score >= 60
                ? 'bg-yellow-100 text-yellow-700'
                : 'bg-gray-100 text-gray-600'
            }`}
          >
            {Math.round(resource.popularity_score)}
          </div>
          <p className="text-[10px] text-gray-400 mt-1">score</p>
        </div>
      </div>
    </div>
  );
}
