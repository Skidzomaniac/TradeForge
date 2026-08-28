import { Line, LineChart, ResponsiveContainer } from 'recharts';

// Tiny inline trend chart for table rows.
export function Sparkline({ data, color = '#00d1a0', height = 28 }: { data: number[]; color?: string; height?: number }) {
  const points = data.map((v, i) => ({ i, v }));
  if (points.length < 2) {
    return <div style={{ height }} />;
  }
  return (
    <ResponsiveContainer width="100%" height={height}>
      <LineChart data={points} margin={{ top: 2, bottom: 2, left: 0, right: 0 }}>
        <Line type="monotone" dataKey="v" stroke={color} strokeWidth={1.5} dot={false} isAnimationActive={false} />
      </LineChart>
    </ResponsiveContainer>
  );
}
