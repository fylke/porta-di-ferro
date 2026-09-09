/**
 * The read side: one SSE stream carries every kind of update, and the client filters by
 * what it is showing. It reconnects itself, and nothing a client renders depends on the
 * connection being up -- a display that loses the server shows stale data rather than
 * breaking (design decision 15).
 */
import { api, type Snapshot } from '../api';

export class Live {
  #snapshot = $state<Snapshot | null>(null);
  /**
   * When the snapshot in hand arrived, by this device's clock. A display counts its match
   * clock on from here, so it has to move with every assignment -- which is why the
   * snapshot goes through an accessor rather than being a bare field.
   */
  receivedAt = $state(0);
  connected = $state(false);
  error = $state('');

  private source: EventSource | null = null;

  get snapshot(): Snapshot | null {
    return this.#snapshot;
  }

  set snapshot(next: Snapshot | null) {
    this.#snapshot = next;
    this.receivedAt = Date.now();
  }

  async start(): Promise<void> {
    await this.refresh();
    this.connect();
  }

  /** Asks for the whole picture again. Cheap at this size, and always safe. */
  async refresh(): Promise<void> {
    try {
      this.snapshot = await api.state();
      this.error = '';
    } catch (e) {
      this.error = e instanceof Error ? e.message : String(e);
    }
  }

  private connect(): void {
    if (this.source) return;
    const source = new EventSource('/api/stream');
    this.source = source;
    source.onopen = () => {
      this.connected = true;
      this.error = '';
      // The server only pushes when something changes, and EventSource reconnects on its
      // own without telling anyone what was missed -- so a display that lost the stream
      // would sit on a stale score until the next event happened to arrive. Asking again
      // on every open is what stops a scoreboard showing yesterday's score for the rest
      // of a match, and it closes the same gap between the initial fetch and the stream
      // being live.
      void this.refresh();
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
