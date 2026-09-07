/**
 * The read side: one SSE stream carries every kind of update, and the client filters by
 * what it is showing. It reconnects itself, and nothing a client renders depends on the
 * connection being up -- a display that loses the server shows stale data rather than
 * breaking (design decision 15).
 */
import { api, type Snapshot } from '../api';

export class Live {
  snapshot = $state<Snapshot | null>(null);
  connected = $state(false);
  error = $state('');

  private source: EventSource | null = null;

  async start(): Promise<void> {
    try {
      this.snapshot = await api.state();
      this.error = '';
    } catch (e) {
      this.error = e instanceof Error ? e.message : String(e);
    }
    this.connect();
  }

  private connect(): void {
    if (this.source) return;
    const source = new EventSource('/api/stream');
    this.source = source;
    source.onopen = () => {
      this.connected = true;
      this.error = '';
    };
    source.onmessage = (ev) => {
      try {
        const update = JSON.parse(ev.data) as { kind: string; data: unknown };
        if (update.kind === 'state') this.snapshot = update.data as Snapshot;
      } catch {
        // A malformed frame is not worth taking the stream down for.
      }
    };
    source.onerror = () => {
      // EventSource retries on its own; all this has to do is say so on screen.
      this.connected = false;
    };
  }

  stop(): void {
    this.source?.close();
    this.source = null;
    this.connected = false;
  }
}
