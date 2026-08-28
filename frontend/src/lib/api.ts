import type {
  LeaderboardResponse,
  Submission,
  TestConfig,
  TestResult,
} from '@/types/leaderboard';

const BASE_URL = import.meta.env.VITE_API_URL || '';

export class ApiError extends Error {
  status: number;
  retryAfter?: number;
  constructor(message: string, status: number, retryAfter?: number) {
    super(message);
    this.status = status;
    this.retryAfter = retryAfter;
  }
}

async function handle<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const retryAfter = Number(res.headers.get('Retry-After')) || undefined;
    let msg = res.statusText;
    try {
      const body = await res.json();
      msg = body?.error?.message || body?.error || msg;
    } catch {
      /* ignore */
    }
    throw new ApiError(msg, res.status, retryAfter);
  }
  return res.json() as Promise<T>;
}

export async function getLeaderboard(): Promise<LeaderboardResponse> {
  return handle(await fetch(`${BASE_URL}/v1/leaderboard`));
}

export async function createSubmission(
  file: File,
  language: string,
  apiKey: string,
): Promise<{ submission_id: string; status: string }> {
  const form = new FormData();
  form.append('file', file);
  form.append('language', language);
  const res = await fetch(`${BASE_URL}/v1/submissions`, {
    method: 'POST',
    headers: { 'X-API-Key': apiKey },
    body: form,
  });
  return handle(res);
}

export async function getSubmission(id: string, apiKey: string): Promise<Submission> {
  return handle(
    await fetch(`${BASE_URL}/v1/submissions/${id}`, { headers: { 'X-API-Key': apiKey } }),
  );
}

export async function getSubmissionLogs(id: string, apiKey: string): Promise<{ logs: string }> {
  return handle(
    await fetch(`${BASE_URL}/v1/submissions/${id}/logs`, { headers: { 'X-API-Key': apiKey } }),
  );
}

export async function createTest(
  config: TestConfig,
  apiKey: string,
): Promise<{ test_id: string; status: string }> {
  return handle(
    await fetch(`${BASE_URL}/v1/tests`, {
      method: 'POST',
      headers: { 'X-API-Key': apiKey, 'Content-Type': 'application/json' },
      body: JSON.stringify(config),
    }),
  );
}

export async function getTest(id: string, apiKey: string): Promise<TestResult> {
  return handle(
    await fetch(`${BASE_URL}/v1/tests/${id}`, { headers: { 'X-API-Key': apiKey } }),
  );
}
