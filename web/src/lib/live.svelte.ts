/**
 * The read side: one SSE stream carries every kind of update, and the client filters by
 * what it is showing. It reconnects itself, and nothing a client renders depends on the
 * connection being up -- a display that loses the server shows stale data rather than
 * breaking (design decision 15).
 */
import { api, type Snapshot } from '../api';

/** Where the last snapshot this device saw is kept, so a client can start with no server. */
const CACHE = 'porta.snapshot';

export class Live {
  #snapshot = $state<Snapshot | null>(null);
  /**
   * True while the snapshot in hand is the cached copy rather than one the server sent
   * this session. The schedule is complete and the names are right; only the statuses
   * may be behind, and the score keeper client keeps its own account of those.
   */
  stale = $state(false);
  /** When the cached copy was saved, for the screen to say how old the schedule is. */
  cachedAt = $state(0);
  /**
   * When the snapshot in hand arrived, by this device's clock. A display counts its match
   * clock on from here, so it has to move with every assignment -- which is why the
   * snapshot goes through an accessor rather than being a bare field.
   */
  receivedAt = $state(0);
  connected = $state(false);
  error = $state('');
  /**
   * The last match whose log the organizer rewrote, with a nonce so two edits of the
   * same match in a row both register. A score keeper holding that match reloads it.
   */
  replaced = $state<{ match: string; nonce: number } | null>(null);

  private source: EventSource | null = null;

  get snapshot(): Snapshot | null {
    return this.#snapshot;
  }

  set snapshot(next: Snapshot | null) {
    this.#snapshot = next;
    this.receivedAt = Date.now();
    this.stale = false;
    if (!next) return;
    try {
      localStorage.setItem(CACHE, JSON.stringify({ at: Date.now(), snapshot: next }));
    } catch {
      // No room or no storage. The live copy still works; only the offline start is lost.
    }
  }

  async start(): Promise<void> {
    await this.refresh();
    if (!this.#snapshot) this.restore();
    this.connect();
  }

  /**
   * Starts from the last snapshot this device saw. This is what lets a score keeper client
   * open with the server unreachable and still know the pool, the names and the running
   * order -- a whole pool can be scored before the LAN is back (issue #70). The service
   * worker keeps the app shell for the same reason; this keeps the data.
   */
  private restore(): void {
    try {
      const raw = localStorage.getItem(CACHE);
      if (!raw) return;
      const { at, snapshot } = JSON.parse(raw) as { at: number; snapshot: Snapshot };
      this.#snapshot = snapshot;
      this.receivedAt = Date.now();
      this.cachedAt = at;
      this.stale = true;
    } catch {
      // A corrupt or absent cache is the same as no cache.
    }
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
        const update = JSON.parse(ev.data) as { kind: string; match?: string; data: unknown };
        if (update.kind === 'state') this.snapshot = update.data as Snapshot;
        if (update.kind === 'log-replaced' && typeof update.match === 'string') {
          this.replaced = { match: update.match, nonce: Date.now() };
        }
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
