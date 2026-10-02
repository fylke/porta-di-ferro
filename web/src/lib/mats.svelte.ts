/**
 * The hall's mats, as the score keepers and the screens follow them (phase 2).
 *
 * Mats belong to the event. One stream carries every mat's queue -- the work items of any
 * discipline placed on it, match by match, with the names in them -- and what each mat is
 * running right now. A tablet bound to mat 2 follows mat 2 from a Longsword pool into a
 * Sabre pool without anybody touching it, and a mat display says which discipline it is on.
 *
 * Like Live it keeps the last copy it saw, so a score keeper's tablet opened with the
 * server out of reach still has the mat's whole queue and can score it offline; every
 * exchange is kept with the discipline its match is in, and reaches that discipline when
 * the LAN comes back (lib/db.ts).
 */
import { api, type MatView, type MatsView, type Slot } from '../api';

const CACHE = 'porta.mats';

export class MatsLive {
  #view = $state<MatsView | null>(null);
  stale = $state(false);
  cachedAt = $state(0);
  /** When the view in hand arrived, by this device's clock: displays count on from here. */
  receivedAt = $state(0);
  connected = $state(false);
  error = $state('');
  /** The last match whose log the organizer rewrote, by discipline, with a nonce. */
  replaced = $state<{ discipline: string; match: string; nonce: number } | null>(null);

  private source: EventSource | null = null;

  get view(): MatsView | null {
    return this.#view;
  }

  set view(next: MatsView | null) {
    this.#view = next;
    this.receivedAt = Date.now();
    this.stale = false;
    if (!next) return;
    try {
      localStorage.setItem(CACHE, JSON.stringify({ at: Date.now(), view: next }));
    } catch {
      // No room or no storage. The live copy still works.
    }
  }

  async start(): Promise<void> {
    await this.refresh();
    if (!this.#view) this.restore();
    this.connect();
  }

  async refresh(): Promise<void> {
    try {
      this.view = await api.mats();
      this.error = '';
    } catch (e) {
      this.error = e instanceof Error ? e.message : String(e);
    }
  }

  private restore(): void {
    try {
      const raw = localStorage.getItem(CACHE);
      if (!raw) return;
      const { at, view } = JSON.parse(raw) as { at: number; view: MatsView };
      this.#view = view;
      this.receivedAt = Date.now();
      this.cachedAt = at;
      this.stale = true;
    } catch {
      // A corrupt or absent cache is the same as no cache.
    }
  }

  private connect(): void {
    if (this.source) return;
    const source = new EventSource('/api/mats/stream');
    this.source = source;
    source.onopen = () => {
      this.connected = true;
      this.error = '';
      void this.refresh();
    };
    source.onmessage = (ev) => {
      try {
        const u = JSON.parse(ev.data) as { kind: string; match?: string; data: unknown };
        if (u.kind === 'mats') this.view = u.data as MatsView;
        if (u.kind === 'log-replaced' && typeof u.match === 'string') {
          const discipline = (u.data as { discipline?: string } | null)?.discipline ?? '';
          this.replaced = { discipline, match: u.match, nonce: Date.now() };
        }
      } catch {
        // A malformed frame is not worth taking the stream down for.
      }
    };
    source.onerror = () => {
      this.connected = false;
    };
  }

  stop(): void {
    this.source?.close();
    this.source = null;
    this.connected = false;
  }
}

/** One mat of the view, or null. */
export function matOf(view: MatsView | null, mat: number): MatView | null {
  return view?.mats.find((m) => m.mat === mat) ?? null;
}

/** A slot's identity across the event: match ids repeat from one discipline to the next. */
export function slotKey(s: { discipline: string; match: { id: string } }): string {
  return `${s.discipline}/${s.match.id}`;
}

/**
 * The matches after the current one on a mat, in running order, up to count. "After" is by
 * position, not status: the current match may be a finished one the score keeper is holding.
 */
export function upcoming(mat: MatView | null, count: number): Slot[] {
  if (!mat) return [];
  const playable = mat.queue.filter((s) => s.match.red && s.match.blue);
  const i = mat.current ? playable.findIndex((s) => slotKey(s) === slotKey(mat.current!)) : -1;
  return playable
    .slice(i + 1)
    .filter((s) => s.match.status !== 'complete')
    .slice(0, count);
}

/**
 * The match clock as a display should show it: placed from the server's "how long ago",
 * then counted on from when the view arrived (lib-display liveElapsed, for a slot).
 */
export function slotElapsed(slot: Slot | null | undefined, receivedAt: number, now: number): number {
  const state = slot?.match.state;
  if (!state) return 0;
  if (!state.running) return state.elapsedMs;
  return state.elapsedMs + (slot?.match.sinceMs ?? 0) + Math.max(0, now - receivedAt);
}
