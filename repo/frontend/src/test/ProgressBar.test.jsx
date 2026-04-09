import React from 'react';
import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import ProgressBar from '../components/ProgressBar';

describe('ProgressBar', () => {
  it('renders correct percentage text', () => {
    render(<ProgressBar current={3} total={10} />);
    expect(screen.getByText('3/10 (30%)')).toBeInTheDocument();
  });

  it('renders 0% when current is 0', () => {
    render(<ProgressBar current={0} total={10} />);
    expect(screen.getByText('0/10 (0%)')).toBeInTheDocument();
  });

  it('renders 100% when complete', () => {
    render(<ProgressBar current={10} total={10} />);
    expect(screen.getByText('10/10 (100%)')).toBeInTheDocument();
  });

  it('renders label when provided', () => {
    render(<ProgressBar current={5} total={10} label="Course Progress" />);
    expect(screen.getByText('Course Progress')).toBeInTheDocument();
    expect(screen.getByText('5/10 (50%)')).toBeInTheDocument();
  });
});
