/**
 * The event as the client knows it: its day, and a summary of every discipline in it
 * (docs/proposals/one-event-many-disciplines.md §12).
 *
 * Two things read it. The router, to know whether an address with no discipline in it
 * means the event's one discipline -- as every address did before there were several -- or
 * the event's own pages. And the event's pages, the landing page and the event admin,
 * which follow it live: one stream for the whole hall, carrying a summary per discipline
 * rather than every discipline's full snapshot, so a phone never holds more than one.
 *
 * A device that cannot reach the server starts from the last copy it saw, so a score
 * keeper's tablet opened with the LAN down still knows what kind of event it is in.
 */
import { api, type EventView, type Presence } from '../api';

const CACHE = 'porta.event';
/** How long the first look waits for the server before going on without it. */
const FIRST_LOOK_MS = 3000;

export class EventLive {
  view = $state<EventView | null>(null);
  /** True once the first look has come back, from the server or from the cache. */
  ready = $state(false);
  connected = $state(false);
  error = $state('');
  /** Every device in the hall and everything set aside, for the admin's Screens panel. */
  presence = $state<Presence | null>(null);
  private source: EventSource | null = null;

  /** Several disciplines, so an address with none in it is the event's page. */
  get multi(): boolean {
    return (this.view?.disciplines.length ?? 1) > 1;
  }

  /** The first look, for the router. Never longer than a few seconds, never an error. */
  async load(): Promise<void> {
    this.restore();
    const timeout = new Promise<void>((resolve) => setTimeout(resolve, FIRST_LOOK_MS));
    await Promise.race([this.refresh(), timeout]);
    this.ready = true;
  }

  async refresh(): Promise<void> {
    try {
      this.set(await api.event());
      this.error = '';
    } catch (e) {
      this.error = e instanceof Error ? e.message : String(e);
    }
  }

  /** Follows the event from here on: what the landing page and the event admin do. */
  follow(): void {
    if (this.source) return;
    const source = new EventSource('/api/event/stream');
    this.source = source;
    source.onopen = () => {
      this.connected = true;
      // Whatever changed while the stream was down, as Live does for a discipline.
      void this.refresh();
    };
    source.onmessage = (ev) => {
      try {
        const update = JSON.parse(ev.data) as { kind: string; data: unknown };
        if (update.kind === 'event') this.set(update.data as EventView);
        if (update.kind === 'presence') this.presence = update.data as Presence;
      } catch {
        // A malformed frame is not worth taking the stream down for.
      }
    };
    source.onerror = () => {
      this.connected = false;
    };
  }

  unfollow(): void {
    this.source?.close();
    this.source = null;
    this.connected = false;
  }

  private set(view: EventView): void {
    this.view = view;
    try {
      localStorage.setItem(CACHE, JSON.stringify(view));
    } catch {
      // No room or no storage: the live copy still works.
    }
  }

  private restore(): void {
    if (this.view) return;
    try {
      const raw = localStorage.getItem(CACHE);
      if (raw) this.view = JSON.parse(raw) as EventView;
    } catch {
      // A corrupt or absent cache is the same as no cache.
    }
  }
}

/** The one copy the whole page shares. */
export const hall = new EventLive();
