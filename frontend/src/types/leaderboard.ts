export interface LeaderboardEntry {
  rank: number;
  previousRank?: number;
  contestant_id: string;
  contestant_name: string;
  score: number;
  p50_us: number;
  p90_us: number;
  p99_us: number;
  tps: number;
  correctness_rate: number;
  status: 'running' | 'completed' | 'failed' | 'idle';
  last_updated_ns?: number;
  rankChange?: number;
}

export interface LeaderboardResponse {
  updated_at: number;
  entries: LeaderboardEntry[];
}

export interface LeaderboardUpdate {
  type: string;
  timestamp: number;
  entries: LeaderboardEntry[];
}

export type ConnectionStatus = 'connecting' | 'connected' | 'disconnected';

export interface Submission {
  id: string;
  contestant_id: string;
  language: string;
  status: 'pending' | 'building' | 'ready' | 'failed' | 'stopped';
  container_ip?: string;
  container_port?: number;
  error_log?: string;
  created_at: string;
  updated_at: string;
}

export interface TestConfig {
  submission_id: string;
  duration_seconds?: number;
  bot_count?: number;
  bot_personas?: string[];
}

export interface TestResult {
  test: {
    id: string;
    status: string;
    final_score?: number;
  };
  live_metrics: Record<string, string>;
}
