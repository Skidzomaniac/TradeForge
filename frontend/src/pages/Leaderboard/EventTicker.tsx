import { useLeaderboardStore } from '@/store/leaderboard';

// Scrolling commentary ticker fed by WebSocket ticker_event messages.
export function EventTicker() {
  const ticker = useLeaderboardStore((s) => s.ticker);
  if (ticker.length === 0) return null;
  const line = ticker.map((t) => t.message).join('     •     ');
  return (
    <div className="overflow-hidden border-t border-surface-tertiary bg-surface-secondary py-2">
      <div className="whitespace-nowrap font-mono text-sm text-accent-green ticker-scroll">{line}</div>
    </div>
  );
}
