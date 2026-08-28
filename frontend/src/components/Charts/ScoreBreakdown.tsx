import React from 'react';

// Inline SVG-free score breakdown: three weighted contribution bars.
export function ScoreBreakdown({
  tps,
  latency,
  correctness,
  score,
}: {
  tps: number;
  latency: number;
  correctness: number;
  score: number;
}) {
  const rows = [
    { label: 'TPS', weight: '40%', value: tps, color: 'bg-accent-green' },
    { label: 'Latency', weight: '40%', value: latency, color: 'bg-accent-blue' },
    { label: 'Correctness', weight: '20%', value: correctness, color: 'bg-accent-blue' },
  ];
  return (
    <div className="font-mono">
      <div className="text-2xl text-gray-100 mb-3">Composite Score: {score.toFixed(1)}</div>
      {rows.map((r) => (
        <div key={r.label} className="flex items-center gap-2 mb-2 text-sm">
          <span className="w-28 text-gray-400">{r.label}</span>
          <span className="flex-1 bg-surface-tertiary rounded h-3">
            <span className={`block ${r.color} h-3 rounded`} style={{ width: `${Math.min(r.value, 100)}%` }} />
          </span>
          <span className="w-12 text-right text-gray-500">{r.weight}</span>
          <span className="w-12 text-right text-gray-200">{r.value.toFixed(0)}</span>
        </div>
      ))}
    </div>
  );
}
