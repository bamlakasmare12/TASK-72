import React, { useState, useEffect, useCallback, useRef } from 'react';
import client from '../api/client';

const DIFFICULTY_OPTIONS = [
  { value: '', label: 'All Levels' },
  { value: 'beginner', label: 'Beginner' },
  { value: 'intermediate', label: 'Intermediate' },
  { value: 'advanced', label: 'Advanced' },
];

const TYPE_OPTIONS = [
  { value: '', label: 'All Types' },
  { value: 'course', label: 'Course' },
  { value: 'article', label: 'Article' },
  { value: 'video', label: 'Video' },
  { value: 'document', label: 'Document' },
  { value: 'assessment', label: 'Assessment' },
];

const SORT_OPTIONS = [
  { value: 'relevance', label: 'Most Relevant' },
  { value: 'popularity', label: 'Most Popular' },
  { value: 'recent', label: 'Most Recent' },
];

export default function SearchFilters({ filters, onFilterChange, synonymsApplied }) {
  const [categories, setCategories] = useState([]);
  const [tags, setTags] = useState([]);
  const [showAdvanced, setShowAdvanced] = useState(false);

  useEffect(() => {
    const fetchTaxonomy = async () => {
      try {
        const [catRes, tagRes] = await Promise.all([
          client.get('/taxonomy/tags?type=category'),
          client.get('/taxonomy/tags?type=skill'),
        ]);
        setCategories(catRes.data || []);
        setTags(tagRes.data || []);
      } catch {
        // Non-critical; filters will just be empty
      }
    };
    fetchTaxonomy();
  }, []);

  const handleChange = useCallback(
    (field, value) => {
      onFilterChange({ ...filters, [field]: value });
    },
    [filters, onFilterChange]
  );

  const handleCategoryToggle = useCallback(
    (catId) => {
      const current = filters.categories || [];
      const next = current.includes(catId)
        ? current.filter((c) => c !== catId)
        : [...current, catId];
      onFilterChange({ ...filters, categories: next });
    },
    [filters, onFilterChange]
  );

  const handleTagToggle = useCallback(
    (tagId) => {
      const current = filters.tags || [];
      const next = current.includes(tagId)
        ? current.filter((t) => t !== tagId)
        : [...current, tagId];
      onFilterChange({ ...filters, tags: next });
    },
    [filters, onFilterChange]
  );

  const clearFilters = () => {
    onFilterChange({
      q: filters.q,
      categories: [],
      tags: [],
      date_from: '',
      date_to: '',
      difficulty: '',
      type: '',
      sort_by: 'relevance',
    });
  };

  const hasActiveFilters =
    (filters.categories?.length > 0) ||
    (filters.tags?.length > 0) ||
    filters.date_from ||
    filters.date_to ||
    filters.difficulty ||
    filters.type;

  return (
    <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-4">
      {/* Sort */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center space-x-2">
          <label className="text-sm font-medium text-gray-600">Sort by:</label>
          <select
            value={filters.sort_by || 'relevance'}
            onChange={(e) => handleChange('sort_by', e.target.value)}
            className="text-sm border border-gray-300 rounded-md px-2 py-1 focus:ring-2 focus:ring-primary-500 outline-none"
          >
            {SORT_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </div>

        <div className="flex items-center space-x-3">
          {hasActiveFilters && (
            <button
              onClick={clearFilters}
              className="text-xs text-red-500 hover:text-red-700"
            >
              Clear filters
            </button>
          )}
          <button
            onClick={() => setShowAdvanced(!showAdvanced)}
            className="text-sm text-primary-600 hover:text-primary-800 font-medium"
          >
            {showAdvanced ? 'Hide filters' : 'Show filters'}
          </button>
        </div>
      </div>

      {/* Synonym notice */}
      {synonymsApplied && synonymsApplied.length > 0 && (
        <div className="mb-3 px-3 py-2 bg-blue-50 rounded-md text-sm text-blue-700">
          Synonym expansion applied for:{' '}
          <strong>{synonymsApplied.join(', ')}</strong>
        </div>
      )}

      {showAdvanced && (
        <div className="space-y-4 pt-3 border-t border-gray-100">
          {/* Difficulty & Type row */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1">
                Difficulty
              </label>
              <select
                value={filters.difficulty || ''}
                onChange={(e) => handleChange('difficulty', e.target.value)}
                className="w-full text-sm border border-gray-300 rounded-md px-2 py-1.5 focus:ring-2 focus:ring-primary-500 outline-none"
              >
                {DIFFICULTY_OPTIONS.map((opt) => (
                  <option key={opt.value} value={opt.value}>
                    {opt.label}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1">
                Type
              </label>
              <select
                value={filters.type || ''}
                onChange={(e) => handleChange('type', e.target.value)}
                className="w-full text-sm border border-gray-300 rounded-md px-2 py-1.5 focus:ring-2 focus:ring-primary-500 outline-none"
              >
                {TYPE_OPTIONS.map((opt) => (
                  <option key={opt.value} value={opt.value}>
                    {opt.label}
                  </option>
                ))}
              </select>
            </div>
          </div>

          {/* Date range */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1">
                Published from
              </label>
              <input
                type="date"
                value={filters.date_from || ''}
                onChange={(e) => handleChange('date_from', e.target.value)}
                className="w-full text-sm border border-gray-300 rounded-md px-2 py-1.5 focus:ring-2 focus:ring-primary-500 outline-none"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1">
                Published to
              </label>
              <input
                type="date"
                value={filters.date_to || ''}
                onChange={(e) => handleChange('date_to', e.target.value)}
                className="w-full text-sm border border-gray-300 rounded-md px-2 py-1.5 focus:ring-2 focus:ring-primary-500 outline-none"
              />
            </div>
          </div>

          {/* Categories */}
          {categories.length > 0 && (
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-2">
                Categories
              </label>
              <div className="flex flex-wrap gap-2">
                {categories.map((cat) => {
                  const active = (filters.categories || []).includes(cat.id);
                  return (
                    <button
                      key={cat.id}
                      onClick={() => handleCategoryToggle(cat.id)}
                      className={`px-3 py-1 rounded-full text-xs font-medium transition-colors ${
                        active
                          ? 'bg-primary-600 text-white'
                          : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
                      }`}
                    >
                      {cat.name}
                    </button>
                  );
                })}
              </div>
            </div>
          )}

          {/* Tags */}
          {tags.length > 0 && (
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-2">
                Skills / Tags
              </label>
              <div className="flex flex-wrap gap-2">
                {tags.map((tag) => {
                  const active = (filters.tags || []).includes(tag.id);
                  return (
                    <button
                      key={tag.id}
                      onClick={() => handleTagToggle(tag.id)}
                      className={`px-3 py-1 rounded-full text-xs font-medium transition-colors ${
                        active
                          ? 'bg-green-600 text-white'
                          : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
                      }`}
                    >
                      {tag.name}
                    </button>
                  );
                })}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
