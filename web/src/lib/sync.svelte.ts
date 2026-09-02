/**
 * The push side of local-first. Writes go to IndexedDB first and reach the server
 * afterwards; nothing in the scoring path waits on the LAN.
 *
 * Retries are safe because the server is idempotent on (match, seq), so this needs no
 * deduplication logic and no acknowledgement protocol -- just "try again later".
 */
import { api } from '../api';
import * as db from './db';
import type { Event } from './match';

export type SyncState = 'idle' | 'pushing' | 'offline';

export class MatchLog {
  matchId = $state('');
  events = $state<Event[]>([]);
  sync = $state<SyncState>('idle');
  pendingCount = $state(0);

  private timer: ReturnType<typeof setInterval> | null = null;

  constructor(matchId: string) {
    this.matchId = matchId;
  }

  /**
   * Loads the match. The server's log and this device's log are merged on (match, seq);
   * whichever has more is simply the union, because one writer per match means there is
   * nothing to reconcile.
   */
  async load(): Promise<void> {
    const local = db.available() ? await db.read(this.matchId) : [];
    let remote: Event[] = [];
    try {
      remote = await api.events(this.matchId, 0);
      this.sync = 'idle';
    } catch {
      this.sync = 'offline';
    }
    const merged = new Map<number, Event>();
    for (const e of remote) merged.set(e.seq, e);
    for (const e of local) if (!merged.has(e.seq)) merged.set(e.seq, e);
    this.events = [...merged.values()].sort((a, b) => a.seq - b.seq);

    // Anything the server had that this device did not is written locally, so the device
    // can carry on alone from here.
    if (db.available()) {
      const known = new Set(local.map((e) => e.seq));
      const missing = remote.filter((e) => !known.has(e.seq));
      if (missing.length > 0) {
        await db.append(this.matchId, missing);
        await db.markPushed(this.matchId, missing.map((e) => e.seq));
      }
    }
    await this.flush();
    this.start();
  }

  /** Appends locally and returns immediately. The push happens after. */
  async write(events: Event[]): Promise<void> {
    this.events = [...this.events, ...events].sort((a, b) => a.seq - b.seq);
    if (db.available()) await db.append(this.matchId, events);
    void this.flush();
  }

  nextSeq(): number {
    return this.events.reduce((max, e) => Math.max(max, e.seq), 0) + 1;
  }

  /** Hands everything unsent to the server. Safe to call at any time, from anywhere. */
  async flush(): Promise<void> {
    if (this.sync === 'pushing') return;
    const outstanding = db.available()
      ? await db.unpushed(this.matchId)
      : this.events;
    this.pendingCount = outstanding.length;
    if (outstanding.length === 0) {
      if (this.sync !== 'idle') this.sync = 'idle';
      return;
    }
    this.sync = 'pushing';
    try {
      await api.pushEvents(this.matchId, outstanding);
      if (db.available()) await db.markPushed(this.matchId, outstanding.map((e) => e.seq));
      this.pendingCount = 0;
      this.sync = 'idle';
    } catch {
      // The LAN is down, or the server is. Neither stops the match: the log is already
      // on this device and the next flush will carry it.
      this.sync = 'offline';
    }
  }

  private start(): void {
    if (this.timer) return;
    this.timer = setInterval(() => void this.flush(), 5000);
  }

  stop(): void {
    if (this.timer) clearInterval(this.timer);
    this.timer = null;
  }
}
