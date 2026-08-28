import { create } from 'zustand';
import type { ConnectionStatus, LeaderboardEntry } from '@/types/leaderboard';
import { getLeaderboard } from '@/lib/api';
import { wsManager } from '@/lib/websocket';

export interface TickerEvent {
  message: string;
  priority: number;
  created_at: number;
}

const REST_POLL_INTERVAL_MS = 3000;

interface LeaderboardState {
  entries: LeaderboardEntry[];
  lastUpdated: number;
  connectionStatus: ConnectionStatus;
  ticker: TickerEvent[];
  _pollTimer: ReturnType<typeof setInterval> | null;
  setEntries: (entries: LeaderboardEntry[]) => void;
  pushTicker: (e: TickerEvent) => void;
  setConnectionStatus: (s: ConnectionStatus) => void;
  fetchLeaderboard: () => Promise<void>;
  connectWebSocket: () => void;
}

export const useLeaderboardStore = create<LeaderboardState>((set, get) => ({
  entries: [],
  lastUpdated: 0,
  connectionStatus: 'disconnected',
  ticker: [],
  _pollTimer: null,

  setEntries: (incoming) => {
    const prev = get().entries;
    const prevRank = new Map(prev.map((e) => [e.contestant_id, e.rank]));
    const entries = incoming.map((e) => {
      const before = prevRank.get(e.contestant_id);
      return {
        ...e,
        previousRank: before,
        rankChange: before !== undefined ? before - e.rank : 0,
      };
    });
    set({ entries, lastUpdated: Date.now() });
  },

  pushTicker: (e) => set({ ticker: [e, ...get().ticker].slice(0, 30) }),

  setConnectionStatus: (connectionStatus) => {
    set({ connectionStatus });
    const state = get();
    if (connectionStatus === 'connected') {
      if (state._pollTimer) {
        clearInterval(state._pollTimer);
        set({ _pollTimer: null });
      }
    } else if (connectionStatus === 'disconnected' && !state._pollTimer) {
      const timer = setInterval(() => get().fetchLeaderboard(), REST_POLL_INTERVAL_MS);
      set({ _pollTimer: timer });
    }
  },

  fetchLeaderboard: async () => {
    try {
      const res = await getLeaderboard();
      get().setEntries(res.entries || []);
    } catch {
      /* keep stale data */
    }
  },

  connectWebSocket: () => {
    wsManager.onStatus((s) => get().setConnectionStatus(s));
    wsManager.onMessage((data) => {
      const msg = data as { type?: string; entries?: LeaderboardEntry[]; message?: string; priority?: number; created_at?: number };
      if (msg?.type === 'ticker_event' && msg.message) {
        get().pushTicker({ message: msg.message, priority: msg.priority ?? 1, created_at: msg.created_at ?? Date.now() });
      } else if (msg?.entries) {
        get().setEntries(msg.entries);
      }
    });
    wsManager.connect();
  },
}));
