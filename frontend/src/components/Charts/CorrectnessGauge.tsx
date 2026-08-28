// Custom SVG circular gauge for correctness percentage (no chart lib).
export function CorrectnessGauge({ value, size = 140 }: { value: number; size?: number }) {
  const pct = Math.max(0, Math.min(1, value));
  const r = size / 2 - 12;
  const c = 2 * Math.PI * r;
  const offset = c * (1 - pct);
  const color = pct >= 0.99 ? '#00d1a0' : pct >= 0.95 ? '#fbbf24' : '#ff4d6a';
  return (
    <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`}>
      <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke="#1a1a1a" strokeWidth={10} />
      <circle
        cx={size / 2}
        cy={size / 2}
        r={r}
        fill="none"
        stroke={color}
        strokeWidth={10}
        strokeLinecap="round"
        strokeDasharray={c}
        strokeDashoffset={offset}
        transform={`rotate(-90 ${size / 2} ${size / 2})`}
        style={{ transition: 'stroke-dashoffset 0.6s ease' }}
      />
      <text x="50%" y="50%" dominantBaseline="middle" textAnchor="middle" fill={color} fontFamily="monospace" fontSize={size / 6}>
        {(pct * 100).toFixed(1)}%
      </text>
    </svg>
  );
}
