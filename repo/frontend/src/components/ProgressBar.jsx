import React from 'react';

export default function ProgressBar({
  current,
  total,
  label,
  showFraction = true,
  size = 'md',
  color = 'primary',
}) {
  const pct = total > 0 ? Math.round((current / total) * 100) : 0;

  const heightClass = {
    sm: 'h-1.5',
    md: 'h-2.5',
    lg: 'h-4',
  }[size];

  const colorClass = {
    primary: 'bg-primary-600',
    green: 'bg-green-500',
    yellow: 'bg-yellow-500',
    red: 'bg-red-500',
  }[color];

  return (
    <div className="w-full">
      {(label || showFraction) && (
        <div className="flex items-center justify-between mb-1">
          {label && (
            <span className="text-xs font-medium text-gray-600">{label}</span>
          )}
          {showFraction && (
            <span className="text-xs text-gray-500">
              {current}/{total} ({pct}%)
            </span>
          )}
        </div>
      )}
      <div className={`w-full bg-gray-200 rounded-full ${heightClass} overflow-hidden`}>
        <div
          className={`${colorClass} ${heightClass} rounded-full transition-all duration-500`}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}
