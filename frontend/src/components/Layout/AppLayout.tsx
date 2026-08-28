import React from 'react';
import { Link, useLocation } from 'react-router-dom';
import { clearApiKey, getApiKey } from '@/lib/auth';

function NavLink({ to, children }: { to: string; children: React.ReactNode }) {
  const { pathname } = useLocation();
  const active = pathname === to;
  return (
    <Link
      to={to}
      className={`font-mono px-4 py-2 rounded transition-colors ${
        active ? 'bg-surface-hover text-accent-green' : 'text-gray-400 hover:text-gray-200 hover:bg-surface-tertiary'
      }`}
    >
      {children}
    </Link>
  );
}

export function AppLayout({ children }: { children: React.ReactNode }) {
  const key = getApiKey();
  return (
    <div className="min-h-screen flex flex-col">
      <nav className="bg-surface-secondary border-b border-surface-tertiary p-4 flex gap-4 items-center">
        <div className="font-bold font-mono text-xl mr-8 flex items-center">
          <span className="text-accent-green mr-2">█</span> TradeForge
        </div>
        <NavLink to="/leaderboard">Leaderboard</NavLink>
        <NavLink to="/submit">Submit</NavLink>
        <NavLink to="/progress">Progress</NavLink>
        <NavLink to="/admin">Admin</NavLink>
        <div className="ml-auto font-mono text-xs text-gray-500">
          {key ? (
            <button onClick={() => { clearApiKey(); location.reload(); }} className="hover:text-gray-300">
              key …{key.slice(-4)} (logout)
            </button>
          ) : (
            <NavLink to="/login">Login</NavLink>
          )}
        </div>
      </nav>
      <main className="flex-1">{children}</main>
    </div>
  );
}
