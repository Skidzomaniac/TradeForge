// Simple API-key auth for the hackathon (NOT for production use).
const KEY = 'trade_eval_api_key';

export function getApiKey(): string | null {
  return localStorage.getItem(KEY);
}

export function setApiKey(key: string): void {
  localStorage.setItem(KEY, key);
}

export function clearApiKey(): void {
  localStorage.removeItem(KEY);
}

export function isAuthenticated(): boolean {
  return !!getApiKey();
}
