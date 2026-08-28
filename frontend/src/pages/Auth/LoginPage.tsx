import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Button, Card } from '@/components/UI';
import { setApiKey } from '@/lib/auth';

export function LoginPage() {
  const [key, setKey] = useState('');
  const navigate = useNavigate();
  const submit = () => {
    if (!key.trim()) return;
    setApiKey(key.trim());
    navigate('/submit');
  };
  return (
    <div className="p-8">
      <h1 className="text-2xl font-bold mb-4 font-mono text-accent-green">Enter your contestant API key</h1>
      <Card className="max-w-md">
        <input
          type="password"
          value={key}
          onChange={(e) => setKey(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && submit()}
          placeholder="API Key…"
          className="w-full bg-surface-primary border border-surface-hover rounded p-2 text-gray-200 font-mono focus:outline-none focus:border-accent-green"
        />
        <Button onClick={submit} className="mt-4">
          Login
        </Button>
      </Card>
    </div>
  );
}
