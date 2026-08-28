import type { ConnectionStatus } from '@/types/leaderboard';

type MessageHandler = (data: unknown) => void;
type StatusHandler = (status: ConnectionStatus) => void;

// Derive the scheme from the page protocol so HTTPS pages use wss and avoid
// mixed-content blocking. VITE_WS_URL overrides this entirely when set.
function defaultWsUrl(): string {
  const scheme = location.protocol === 'https:' ? 'wss' : 'ws';
  return `${scheme}://${location.host}/ws`;
}

const WS_URL = import.meta.env.VITE_WS_URL || defaultWsUrl();

// Singleton WebSocket manager with exponential-backoff reconnection.
class WebSocketManager {
  private ws: WebSocket | null = null;
  private messageHandlers = new Set<MessageHandler>();
  private statusHandlers = new Set<StatusHandler>();
  private backoff = 1000;
  private readonly maxBackoff = 30000;
  private shouldRun = false;

  // Idempotent: a second call while already running or connected is a no-op, so
  // React StrictMode double-invokes and remounts do not open a second socket.
  connect(): void {
    if (this.shouldRun) return;
    this.shouldRun = true;
    this.open();
  }

  disconnect(): void {
    this.shouldRun = false;
    this.ws?.close();
    this.ws = null;
  }

  onMessage(h: MessageHandler): () => void {
    this.messageHandlers.add(h);
    return () => this.messageHandlers.delete(h);
  }

  onStatus(h: StatusHandler): () => void {
    this.statusHandlers.add(h);
    return () => this.statusHandlers.delete(h);
  }

  private emitStatus(s: ConnectionStatus) {
    this.statusHandlers.forEach((h) => h(s));
  }

  private open() {
    this.emitStatus('connecting');
    try {
      this.ws = new WebSocket(WS_URL);
    } catch {
      this.scheduleReconnect();
      return;
    }
    this.ws.onopen = () => {
      this.backoff = 1000;
      this.emitStatus('connected');
    };
    this.ws.onmessage = (ev) => {
      try {
        this.messageHandlers.forEach((h) => h(JSON.parse(ev.data)));
      } catch {
        /* ignore malformed */
      }
    };
    this.ws.onclose = () => {
      this.emitStatus('disconnected');
      this.scheduleReconnect();
    };
    this.ws.onerror = () => this.ws?.close();
  }

  private scheduleReconnect() {
    if (!this.shouldRun) return;
    setTimeout(() => this.open(), this.backoff);
    this.backoff = Math.min(this.backoff * 2, this.maxBackoff);
  }
}

export const wsManager = new WebSocketManager();
