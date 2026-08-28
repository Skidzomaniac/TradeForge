import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { Card, Badge, statusVariant } from '@/components/UI';
import { ScoreBreakdown } from '@/components/Charts/ScoreBreakdown';
import { CorrectnessGauge } from '@/components/Charts/CorrectnessGauge';
import { getTest } from '@/lib/api';
import { getApiKey } from '@/lib/auth';
import type { TestResult } from '@/types/leaderboard';

export function ResultsPage() {
  const { testId } = useParams();
  const apiKey = getApiKey() || '';
  const [result, setResult] = useState<TestResult | null>(null);

  useEffect(() => {
    if (!testId) return;
    let stop = false;
    const poll = async () => {
      try {
        const r = await getTest(testId, apiKey);
        if (!stop) setResult(r);
      } catch {
        /* ignore */
      }
    };
    poll();
    const id = window.setInterval(poll, 2000);
    return () => {
      stop = true;
      window.clearInterval(id);
    };
  }, [testId, apiKey]);

  const m = result?.live_metrics ?? {};
  const p99 = Number(m['p99_latency_us'] || 0);
  const tps = Number(m['tps'] || 0);
  const correctness = Number(m['correctness_rate'] || 0);
  const score = result?.test.final_score ?? 0;

  return (
    <div className="p-8 max-w-3xl">
      <h1 className="text-2xl font-bold mb-4 font-mono text-accent-green">Test Results</h1>
      <div className="grid md:grid-cols-2 gap-4">
        <Card>
          <div className="flex items-center gap-2 mb-3">
            <span className="font-mono text-gray-400">Status:</span>
            <Badge variant={statusVariant(result?.test.status || 'idle')}>
              {result?.test.status || 'loading…'}
            </Badge>
          </div>
          <ScoreBreakdown
            tps={tps > 0 ? 100 : 0}
            latency={p99 > 0 && p99 < 1000 ? 90 : 50}
            correctness={correctness * 100}
            score={score}
          />
        </Card>
        <Card>
          <div className="flex items-center gap-6">
            <CorrectnessGauge value={correctness} />
            <div className="font-mono text-sm space-y-2 flex-1">
              <Metric label="P99 latency" value={`${p99} µs`} />
              <Metric label="TPS" value={tps.toFixed(0)} />
              <Metric label="Correctness" value={`${(correctness * 100).toFixed(2)}%`} />
            </div>
          </div>
        </Card>
      </div>
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between">
      <span className="text-gray-400">{label}</span>
      <span className="text-gray-200">{value}</span>
    </div>
  );
}
