import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { LeaderboardTable } from './LeaderboardTable';
import type { LeaderboardEntry } from '@/types/leaderboard';

const entries: LeaderboardEntry[] = [
  { rank: 1, contestant_id: 'a', contestant_name: 'Alice', score: 88.5, p50_us: 100, p90_us: 200, p99_us: 450, tps: 9000, correctness_rate: 0.999, status: 'running' },
  { rank: 2, contestant_id: 'b', contestant_name: 'Bob', score: 71.2, p50_us: 300, p90_us: 900, p99_us: 2500, tps: 7000, correctness_rate: 0.97, status: 'completed' },
];

describe('LeaderboardTable', () => {
  it('renders all contestants', () => {
    render(<LeaderboardTable entries={entries} />);
    expect(screen.getByText('Alice')).toBeInTheDocument();
    expect(screen.getByText('Bob')).toBeInTheDocument();
  });

  it('shows an empty state with no entries', () => {
    render(<LeaderboardTable entries={[]} />);
    expect(screen.getByText(/No contestants yet/i)).toBeInTheDocument();
  });

  it('formats latency in ms above 1000µs', () => {
    render(<LeaderboardTable entries={entries} />);
    expect(screen.getByText('2.50ms')).toBeInTheDocument();
  });
});
