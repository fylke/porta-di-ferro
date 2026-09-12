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
import { differences } from './drift';
import { MSL, replay, type Event, type State } from './match';

export type SyncState = 'idle' | 'pushing' | 'offline';

let flushingAll = false;

/**
 * Hands the server every match log on this device that it does not have yet, not only the
 * one on screen.
 *
 * A pool run offline is several finished matches, and by the time the LAN is back only
 * the last of them is open in a MatchLog. Without this, the earlier ones would sit in
 * IndexedDB until somebody happened to reopen each on the same device -- which is to say
 * never, and the whole point of running the pool offline was to report it afterwards.
 * Outstanding is derived the same way load() derives it, by asking the server what it has,
 * so nothing has to be flagged in storage.
 */
export async function flushAll(except = ''): Promise<void> {
  if (flushingAll) return;
  flushingAll = true;
  try {
    for (const id of await db.matches()) {
      if (id === except) continue;
      const local = await db.read(id);
      if (local.length === 0) continue;
      const remote = await api.events(id, 0);
      const have = new Set(remote.map((e) => e.seq));
      const missing = local.filter((e) => !have.has(e.seq));
      if (missing.length > 0) await api.pushEvents(id, missing);
    }
  } catch {
    // The LAN went again. The next successful flush of the open match tries the rest.
  } finally {
    flushingAll = false;
  }
}

export class MatchLog {
  matchId = $state('');
  events = $state<Event[]>([]);
  sync = $state<SyncState>('idle');
  pendingCount = $state(0);
  /** False when the log lives only in memory, so a refresh would lose it. */
  durable = $state(true);

  /**
   * The state the server derived from this log the last time it answered a push. The
   * engine exists twice, and this is the one place a live match can show the two apart.
   */
  serverState = $state<State | null>(null);
  /** Set when the server disagreed with this device about the same log. */
  drift = $state<{ local: State; server: State; fields: string[] } | null>(null);
  /**
   * True once the score keeper has chosen the server's numbers over this device's. The
   * session then shows the server's state whenever it has one for the current log, which
   * is the fallback for a dual-engine bug found in a live match: the match goes on under
   * the server's rules, and the bug earns a vector afterwards.
   */
  aligned = $state(false);
  private driftDismissedAt = 0;

  /**
   * Sequence numbers the server has confirmed. Held here rather than as a flag in
   * storage: it is derivable from what the server hands back, so it stays correct even
   * when there is no storage to write a flag to.
   */
  private pushed = new Set<number>();
  // Note: reload() replaces this set outright, so it cannot be readonly.
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
    if (this.sync === 'idle') void flushAll(this.matchId);
    this.start();
  }

  /** Appends to the log and returns. The push happens after; so does the disk write. */
  async write(events: Event[]): Promise<void> {
    this.events = [...this.events, ...events].sort((a, b) => a.seq - b.seq);
    await db.append(this.matchId, events);
    this.durable = db.usable();
    void this.flush();
  }

  /**
   * The organizer has rewritten this match's log. The server's copy is now the match;
   * whatever this device had is replaced by it, unsent events included -- they were
   * written against a history that no longer exists, and the organizer's edit is the
   * later and more deliberate act.
   */
  async reload(): Promise<void> {
    let remote: Event[];
    try {
      remote = await api.events(this.matchId, 0);
    } catch {
      // Offline: the old copy stays until the next contact.
      return;
    }
    await db.clear(this.matchId);
    await db.append(this.matchId, remote);
    this.pushed = new Set(remote.map((e) => e.seq));
    this.events = remote;
    this.pendingCount = 0;
    this.drift = null;
    this.serverState = null;
    if (this.sync === 'offline') this.sync = 'idle';
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
      const res = await api.pushEvents(this.matchId, batch);
      for (const e of batch) this.pushed.add(e.seq);
      this.check(res.state);
      this.pendingCount = 0;
      this.sync = 'idle';
      // Reaching the server with this match's backlog means the LAN is back: the matches
      // finished before it are waiting too.
      void flushAll(this.matchId);
    } catch {
      // The LAN is down, or the server is. Neither stops the match: the log is already on
      // this device and the next flush will carry it.
      this.sync = 'offline';
    }
  }

  /**
   * Compares what the server derived with what this device derived from the same log.
   * Only when the two logs are the same length: a server that is ahead has another
   * writer, which is a handover matter, not an engine one.
   */
  private check(server: State): void {
    this.serverState = server;
    const local = replay(MSL, this.events);
    if (server.lastSeq !== local.lastSeq) return;
    const fields = differences(local, server);
    if (fields.length === 0) {
      this.drift = null;
      return;
    }
    if (this.driftDismissedAt === local.lastSeq) return;
    this.drift = { local, server, fields };
    // Loud in the console as well as on screen: this is the bug report.
    console.error('porta: the server derived a different state from the same log', {
      match: this.matchId,
      fields,
      local,
      server,
    });
  }

  /** The score keeper has seen the disagreement and is carrying on with this device's numbers. */
  dismissDrift(): void {
    this.driftDismissedAt = this.drift?.local.lastSeq ?? 0;
    this.drift = null;
  }

  /** The score keeper has chosen the server's numbers. */
  alignToServer(): void {
    this.aligned = true;
    this.drift = null;
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
