/**
 * The push side of local-first. Writes go to the log first and reach the server
 * afterwards; nothing in the scoring path waits on the LAN, and nothing in it waits on
 * storage either.
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
  /** False when the log lives only in memory, so a refresh would lose it. */
  durable = $state(true);

  /**
   * Sequence numbers the server has confirmed. Held here rather than as a flag in
   * storage: it is derivable from what the server hands back, so it stays correct even
   * when there is no storage to write a flag to.
   */
  private pushed = new Set<number>();
  private timer: ReturnType<typeof setInterval> | null = null;

  constructor(matchId: string) {
    this.matchId = matchId;
  }

  /**
   * Loads the match. The server's log and this device's are merged on (match, seq);
   * the union is the answer, because one writer per match means there is nothing to
   * reconcile. Anything the server has not got is outstanding, which is exactly what a
   * device that has been offline needs.
   */
  async load(): Promise<void> {
    const local = await db.read(this.matchId);
    let remote: Event[] = [];
    try {
      remote = await api.events(this.matchId, 0);
      this.sync = 'idle';
    } catch {
      this.sync = 'offline';
    }

    const merged = new Map<number, Event>();
    for (const e of remote) {
      merged.set(e.seq, e);
      this.pushed.add(e.seq);
    }
    for (const e of local) if (!merged.has(e.seq)) merged.set(e.seq, e);
    this.events = [...merged.values()].sort((a, b) => a.seq - b.seq);

    // Give this device a durable copy of anything only the server had, so it can carry
    // on alone from here.
    const known = new Set(local.map((e) => e.seq));
    const missing = remote.filter((e) => !known.has(e.seq));
    if (missing.length > 0) await db.append(this.matchId, missing);

    this.durable = db.usable();
    await this.flush();
    this.start();
  }

  /** Appends to the log and returns. The push happens after; so does the disk write. */
  async write(events: Event[]): Promise<void> {
    this.events = [...this.events, ...events].sort((a, b) => a.seq - b.seq);
    await db.append(this.matchId, events);
    this.durable = db.usable();
    void this.flush();
  }

  nextSeq(): number {
    return this.events.reduce((max, e) => Math.max(max, e.seq), 0) + 1;
  }

  private outstanding(): Event[] {
    return this.events.filter((e) => !this.pushed.has(e.seq));
  }

  /** Hands everything unsent to the server. Safe to call at any time, from anywhere. */
  async flush(): Promise<void> {
    if (this.sync === 'pushing') return;
    const batch = this.outstanding();
    this.pendingCount = batch.length;
    if (batch.length === 0) {
      if (this.sync !== 'idle') this.sync = 'idle';
      return;
    }
    this.sync = 'pushing';
    try {
      await api.pushEvents(this.matchId, batch);
      for (const e of batch) this.pushed.add(e.seq);
      this.pendingCount = 0;
      this.sync = 'idle';
    } catch {
      // The LAN is down, or the server is. Neither stops the match: the log is already on
      // this device and the next flush will carry it.
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
