import { useLeaderboardFeed } from '@/hooks/useWebSocket';
import { LeaderboardTable } from '@/components/LeaderboardTable/LeaderboardTable';
import { EventTicker } from './EventTicker';
import { Card } from '@/components/UI';

const dot = {
  connected: 'bg-accent-green',
  connecting: 'bg-accent-yellow',
  disconnected: 'bg-accent-red',
} as const;

export function LeaderboardPage() {
  const { status, entries, lastUpdated } = useLeaderboardFeed();
  const ago = lastUpdated ? ((Date.now() - lastUpdated) / 1000).toFixed(1) : '--';
  const leader = entries[0];

  return (
    <div className="flex flex-col h-full">
      <div className="p-6 md:p-8 flex-1">
        <div className="flex items-center justify-between mb-6 flex-wrap gap-3">
          <h1 className="text-3xl font-bold font-mono text-accent-green ">Live Leaderboard</h1>
          <div className="flex items-center gap-2 font-mono text-xs text-gray-400">
            <span className={`inline-block w-2 h-2 rounded-full ${dot[status]} animate-pulse`} />
            {status} · updated {ago}s ago · {entries.length} contestants
          </div>
        </div>

        {leader && (
          <div className="mb-6 grid grid-cols-1 sm:grid-cols-3 gap-4">
            <Stat label="Leader" value={leader.contestant_name} accent />
            <Stat label="Top score" value={leader.score.toFixed(1)} />
            <Stat label="Best p99" value={`${Math.min(...entries.map((e) => e.p99_us || Infinity))}µs`} />
          </div>
        )}

        <Card className="overflow-x-auto">
          <LeaderboardTable entries={entries} />
        </Card>
      </div>
      <EventTicker />
    </div>
  );
}

function Stat({ label, value, accent }: { label: string; value: string; accent?: boolean }) {
  return (
    <div className="bg-surface-secondary border border-surface-tertiary rounded-lg p-4">
      <div className="text-xs font-mono text-gray-500 mb-1">{label}</div>
      <div className={`text-xl font-mono ${accent ? 'text-accent-green' : 'text-gray-200'}`}>{value}</div>
    </div>
  );
}
