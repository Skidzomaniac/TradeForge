import React from 'react';
import clsx from 'clsx';
import type { LeaderboardEntry } from '@/types/leaderboard';
import { Badge, statusVariant } from '@/components/UI';

function latencyColor(us: number): string {
  if (us > 0 && us < 500) return 'text-accent-green';
  if (us < 2000) return 'text-accent-yellow';
  return 'text-accent-red';
}

function fmtUs(us: number): string {
  if (us >= 1000) return `${(us / 1000).toFixed(2)}ms`;
  return `${us}µs`;
}

const Row = React.memo(function Row({ entry }: { entry: LeaderboardEntry }) {
  const change = entry.rankChange ?? 0;
  const arrow = change > 0 ? '▲' : change < 0 ? '▼' : '';
  const arrowColor = change > 0 ? 'text-accent-green' : 'text-accent-red';
  const flash = change > 0 ? 'flash-up' : change < 0 ? 'flash-down' : '';
  return (
    <tr className={clsx('border-b border-surface-tertiary hover:bg-surface-hover transition-colors', flash)}>
      <td className="py-2 px-3 text-center font-mono">
        {entry.rank} <span className={arrowColor}>{arrow}</span>
      </td>
      <td className="py-2 px-3 font-mono text-gray-200">{entry.contestant_name}</td>
      <td className="py-2 px-3 text-center font-mono text-gray-100">
        <div className="flex items-center gap-2">
          <span className="w-12 text-right">{entry.score.toFixed(1)}</span>
          <span className="flex-1 bg-surface-tertiary rounded h-2 hidden sm:block">
            <span
              className="block bg-accent-green h-2 rounded"
              style={{ width: `${Math.min(entry.score, 100)}%` }}
            />
          </span>
        </div>
      </td>
      <td className={clsx('py-2 px-3 text-right font-mono hidden md:table-cell', latencyColor(entry.p50_us))}>
        {fmtUs(entry.p50_us)}
      </td>
      <td className={clsx('py-2 px-3 text-right font-mono hidden md:table-cell', latencyColor(entry.p90_us))}>
        {fmtUs(entry.p90_us)}
      </td>
      <td className={clsx('py-2 px-3 text-right font-mono', latencyColor(entry.p99_us))}>
        {fmtUs(entry.p99_us)}
      </td>
      <td className="py-2 px-3 text-right font-mono text-gray-200">{entry.tps.toFixed(0)}</td>
      <td
        className={clsx(
          'py-2 px-3 text-right font-mono',
          entry.correctness_rate < 0.99 ? 'text-accent-red' : 'text-gray-200',
        )}
      >
        {(entry.correctness_rate * 100).toFixed(1)}%
      </td>
      <td className="py-2 px-3 text-center">
        <Badge variant={statusVariant(entry.status)}>{entry.status}</Badge>
      </td>
    </tr>
  );
});

export function LeaderboardTable({ entries }: { entries: LeaderboardEntry[] }) {
  if (entries.length === 0) {
    return <p className="text-gray-500 font-mono p-6">No contestants yet…</p>;
  }
  return (
    <table className="w-full text-sm">
      <thead className="sticky top-0 bg-surface-secondary">
        <tr className="text-gray-400 font-mono text-xs uppercase">
          <th className="py-2 px-3">Rank</th>
          <th className="py-2 px-3 text-left">Contestant</th>
          <th className="py-2 px-3">Score</th>
          <th className="py-2 px-3 text-right hidden md:table-cell">P50</th>
          <th className="py-2 px-3 text-right hidden md:table-cell">P90</th>
          <th className="py-2 px-3 text-right">P99</th>
          <th className="py-2 px-3 text-right">TPS</th>
          <th className="py-2 px-3 text-right">Correct</th>
          <th className="py-2 px-3">Status</th>
        </tr>
      </thead>
      <tbody>
        {entries.map((e) => (
          <Row key={e.contestant_id} entry={e} />
        ))}
      </tbody>
    </table>
  );
}
