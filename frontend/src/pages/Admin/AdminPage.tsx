import { useEffect, useState } from 'react';
import toast from 'react-hot-toast';
import { Button, Card, Badge } from '@/components/UI';

// Admin endpoints live on the leaderboard-api. Defaults to same-origin (the
// production nginx routes /admin there); override in dev with VITE_LEADERBOARD_URL.
const LB = import.meta.env.VITE_LEADERBOARD_URL || '';
const ADMIN_KEY_STORE = 'trade_eval_admin_key';

interface SystemStatus {
  redis: { healthy: boolean; memory: string };
  active_contestants: number;
  leaderboard_frozen: boolean;
}

export function AdminPage() {
  const [key, setKey] = useState(localStorage.getItem(ADMIN_KEY_STORE) || '');
  const [status, setStatus] = useState<SystemStatus | null>(null);

  const call = async (path: string, method = 'POST', body?: unknown) => {
    const res = await fetch(`${LB}/admin/v1${path}`, {
      method,
      headers: { 'X-Admin-Key': key, 'Content-Type': 'application/json' },
      body: body ? JSON.stringify(body) : undefined,
    });
    if (!res.ok) {
      toast.error(`${path}: ${res.status}`);
      return null;
    }
    return res.json();
  };

  const refresh = async () => {
    const s = await call('/system/status', 'GET');
    if (s) setStatus(s);
  };

  useEffect(() => {
    if (!key) return;
    localStorage.setItem(ADMIN_KEY_STORE, key);
    refresh();
    const t = window.setInterval(refresh, 5000);
    return () => window.clearInterval(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);

  return (
    <div className="p-8 max-w-3xl">
      <h1 className="text-2xl font-bold mb-4 font-mono text-accent-green">Operations Console</h1>

      <Card className="mb-4">
        <label className="block text-xs font-mono text-gray-500 mb-1">Admin API key</label>
        <input
          type="password"
          value={key}
          onChange={(e) => setKey(e.target.value)}
          placeholder="X-Admin-Key"
          className="w-full bg-surface-primary border border-surface-hover rounded p-2 font-mono text-gray-200 focus:outline-none focus:border-accent-green"
        />
      </Card>

      {status && (
        <Card className="mb-4">
          <div className="grid grid-cols-3 gap-3 font-mono text-sm">
            <Stat label="Redis" value={status.redis.healthy ? 'healthy' : 'down'} ok={status.redis.healthy} />
            <Stat label="Active contestants" value={String(status.active_contestants)} ok />
            <Stat label="Leaderboard" value={status.leaderboard_frozen ? 'FROZEN' : 'live'} ok={!status.leaderboard_frozen} />
          </div>
        </Card>
      )}

      <Card>
        <div className="flex flex-wrap gap-3">
          <Button onClick={async () => { await call('/leaderboard/freeze'); toast.success('Frozen'); refresh(); }}>
            Freeze leaderboard
          </Button>
          <Button variant="secondary" onClick={async () => { await call('/leaderboard/unfreeze'); toast.success('Unfrozen'); refresh(); }}>
            Unfreeze
          </Button>
        </div>
      </Card>
    </div>
  );
}

function Stat({ label, value, ok }: { label: string; value: string; ok: boolean }) {
  return (
    <div className="bg-surface-primary border border-surface-tertiary rounded p-3">
      <div className="text-xs text-gray-500 mb-1">{label}</div>
      <Badge variant={ok ? 'success' : 'danger'}>{value}</Badge>
    </div>
  );
}
