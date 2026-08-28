import React from 'react';
import clsx from 'clsx';

type BtnVariant = 'primary' | 'secondary' | 'danger' | 'ghost';

export function Button({
  variant = 'primary',
  loading,
  className,
  children,
  ...rest
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { variant?: BtnVariant; loading?: boolean }) {
  return (
    <button
      {...rest}
      disabled={rest.disabled || loading}
      className={clsx(
        'font-mono py-2 px-4 rounded transition-colors disabled:opacity-50 disabled:cursor-not-allowed',
        variant === 'primary' && 'bg-accent-green text-surface-primary font-bold hover:bg-opacity-90',
        variant === 'secondary' && 'border border-surface-hover text-gray-200 hover:bg-surface-tertiary',
        variant === 'danger' && 'bg-accent-red text-white hover:bg-opacity-90',
        variant === 'ghost' && 'text-gray-400 hover:text-gray-200',
        className,
      )}
    >
      {loading ? '…' : children}
    </button>
  );
}

type BadgeVariant = 'success' | 'warning' | 'danger' | 'info' | 'neutral';

export function Badge({ variant = 'neutral', children }: { variant?: BadgeVariant; children: React.ReactNode }) {
  return (
    <span
      className={clsx(
        'inline-block px-2 py-0.5 rounded text-xs font-mono',
        variant === 'success' && 'bg-accent-green/20 text-accent-green',
        variant === 'warning' && 'bg-accent-yellow/20 text-accent-yellow',
        variant === 'danger' && 'bg-accent-red/20 text-accent-red',
        variant === 'info' && 'bg-accent-blue/20 text-accent-blue',
        variant === 'neutral' && 'bg-surface-hover text-gray-400',
      )}
    >
      {children}
    </span>
  );
}

export function Card({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <div className={clsx('bg-surface-secondary p-6 rounded-lg border border-surface-tertiary', className)}>
      {children}
    </div>
  );
}

export function Spinner() {
  return <span className="inline-block animate-spin text-accent-green">◌</span>;
}

export function statusVariant(status: string): BadgeVariant {
  switch (status) {
    case 'ready':
    case 'completed':
      return 'success';
    case 'building':
    case 'pending':
    case 'running':
      return 'info';
    case 'failed':
      return 'danger';
    default:
      return 'neutral';
  }
}
