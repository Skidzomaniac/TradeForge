import { useEffect, useState } from 'react';
import { Card, Badge } from '@/components/UI';

interface Insights {
  contestant_id: string;
  standing: { rank: number; score: number; tps: number; p99_us: number; correctness_rate: number };
  weakness: string;
  tips: string[];
  field_size: number;
}

const BASE = import.meta.env.VITE_API_URL || '';

interface Prediction {
  predicted_score: number;
  trend: string;
  confidence: number;
  samples: number;
}

export function ProgressPage() {
  const [contestantId, setContestantId] = useState('');
  const [data, setData] = useState<Insights | null>(null);
  const [pred, setPred] = useState<Prediction | null>(null);
  const [error, setError] = useState('');

  const load = async (id: string) => {
    setError('');
    try {
      const res = await fetch(`${BASE}/v1/contestants/${id}/insights`);
      if (!res.ok) throw new Error((await res.json()).error || 'not found');
      setData(await res.json());
      const pr = await fetch(`${BASE}/v1/contestants/${id}/prediction`);
      if (pr.ok) setPred(await pr.json());
    } catch (e) {
      setData(null);
      setError((e as Error).message);
    }
  };

  useEffect(() => {
    const t = window.setInterval(() => {
      if (contestantId) load(contestantId);
    }, 5000);
    return () => window.clearInterval(t);
  }, [contestantId]);

  return (
    <div className="p-8 max-w-2xl">
      <h1 className="text-2xl font-bold mb-4 font-mono text-accent-green">My Progress</h1>
      <Card className="mb-4">
        <div className="flex gap-3">
          <input
            value={contestantId}
            onChange={(e) => setContestantId(e.target.value)}
            placeholder="Your contestant id"
            className="flex-1 bg-surface-primary border border-surface-hover rounded p-2 font-mono text-gray-200 focus:outline-none focus:border-accent-green"
          />
          <button
            onClick={() => load(contestantId)}
            className="bg-accent-green text-surface-primary font-mono font-bold px-4 rounded"
          >
            Load
          </button>
        </div>
        {error && <p className="text-accent-red font-mono text-sm mt-2">{error}</p>}
      </Card>

      {data && (
        <Card>
          <div className="flex items-center gap-3 mb-4 font-mono">
            <span className="text-3xl text-gray-100">#{data.standing.rank}</span>
            <span className="text-gray-400">of {data.field_size}</span>
            <Badge variant="info">weakness: {data.weakness}</Badge>
          </div>
          <div className="grid grid-cols-3 gap-3 font-mono text-sm mb-4">
            <Metric label="Score" value={data.standing.score.toFixed(1)} />
            <Metric label="TPS" value={data.standing.tps.toFixed(0)} />
            <Metric label="P99" value={`${data.standing.p99_us}µs`} />
          </div>
          {pred && pred.samples >= 3 && (
            <div className="mb-4 font-mono text-sm flex items-center gap-2">
              <span className="text-gray-400">Projected final score:</span>
              <span className="text-gray-100 text-lg">{pred.predicted_score.toFixed(1)}</span>
              <Badge variant={pred.trend === 'up' ? 'success' : pred.trend === 'down' ? 'danger' : 'neutral'}>
                {pred.trend === 'up' ? '▲ trending up' : pred.trend === 'down' ? '▼ trending down' : 'flat'}
              </Badge>
              <span className="text-gray-600 text-xs">({(pred.confidence * 100).toFixed(0)}% conf)</span>
            </div>
          )}
          <ul className="space-y-2">
            {data.tips.map((t, i) => (
              <li key={i} className="font-mono text-sm text-gray-300">{t}</li>
            ))}
          </ul>
        </Card>
      )}
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-surface-primary border border-surface-tertiary rounded p-3">
      <div className="text-xs text-gray-500">{label}</div>
      <div className="text-gray-200">{value}</div>
    </div>
  );
}
