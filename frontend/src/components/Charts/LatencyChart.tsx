import {
  Area,
  AreaChart,
  CartesianGrid,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';

export interface LatencyPoint {
  t: string;
  p50: number;
  p90: number;
  p99: number;
}

const axis = { stroke: '#555', fontSize: 10, fontFamily: 'monospace' };

export function LatencyChart({ data }: { data: LatencyPoint[] }) {
  return (
    <ResponsiveContainer width="100%" height={240}>
      <AreaChart data={data} margin={{ top: 8, right: 12, left: 0, bottom: 0 }}>
        <defs>
          <linearGradient id="p99" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="#ff4d6a" stopOpacity={0.4} />
            <stop offset="100%" stopColor="#ff4d6a" stopOpacity={0} />
          </linearGradient>
        </defs>
        <CartesianGrid strokeDasharray="3 3" stroke="#1a1a1a" />
        <XAxis dataKey="t" tick={axis} />
        <YAxis tick={axis} width={48} unit="µs" />
        <Tooltip
          contentStyle={{ background: '#111', border: '1px solid #1a1a1a', fontFamily: 'monospace', fontSize: 12 }}
        />
        <ReferenceLine y={500} stroke="#00d1a0" strokeDasharray="2 2" />
        <ReferenceLine y={2000} stroke="#fbbf24" strokeDasharray="2 2" />
        <Area type="monotone" dataKey="p99" stroke="#ff4d6a" fill="url(#p99)" strokeWidth={2} isAnimationActive={false} />
        <Area type="monotone" dataKey="p90" stroke="#fbbf24" fillOpacity={0} strokeWidth={1.5} isAnimationActive={false} />
        <Area type="monotone" dataKey="p50" stroke="#00d1a0" fillOpacity={0} strokeWidth={1.5} isAnimationActive={false} />
      </AreaChart>
    </ResponsiveContainer>
  );
}
