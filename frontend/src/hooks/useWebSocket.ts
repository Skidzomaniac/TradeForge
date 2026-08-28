import { useEffect } from 'react';
import { useLeaderboardStore } from '@/store/leaderboard';

// Connects the leaderboard store to the WebSocket + REST fallback once.
export function useLeaderboardFeed() {
  const connect = useLeaderboardStore((s) => s.connectWebSocket);
  const fetchLeaderboard = useLeaderboardStore((s) => s.fetchLeaderboard);
  useEffect(() => {
    fetchLeaderboard();
    connect();
  }, [connect, fetchLeaderboard]);
  return {
    status: useLeaderboardStore((s) => s.connectionStatus),
    entries: useLeaderboardStore((s) => s.entries),
    lastUpdated: useLeaderboardStore((s) => s.lastUpdated),
  };
}
