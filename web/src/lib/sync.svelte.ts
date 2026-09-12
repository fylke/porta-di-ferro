/**
 * The push side of local-first. Writes go to the log first and reach the server
 * afterwards; nothing in the scoring path waits on the LAN, and nothing in it waits on
 * storage either.
 *
 * Retries are safe because the server is idempotent on (match, seq), so this needs no
 * deduplication logic and no acknowledgement protocol -- just "try again later".
 */
import { api, ApiError, type Client } from '../api';
import * as db from './db';
import { differences } from './drift';
import { MSL, replay, type Event, type State } from './match';
import { clientId } from './presence.svelte';

/**
 * 'stale' is the one state a match does not come back from on its own: another device
 * has taken this match over, and whatever this one still holds has been set aside for
 * the organizer (design §7 item 10).
 */
export type SyncState = 'idle' | 'pushing' | 'offline' | 'stale';

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
      if (missing.length === 0) continue;
      // Claim it first, the way the open match was claimed. If somebody else holds it
      // now, push anyway on a dead epoch: the server sets the events aside and the
      // organizer sees them, which beats leaving them here where nobody will.
      let epoch = 0;
      try {
        epoch = (await api.claim(id, clientId())).epoch;
      } catch (e) {
        if (!(e instanceof ApiError) || e.status !== 409) throw e;
      }
      try {
        await api.pushEvents(id, missing, { client: clientId(), epoch });
      } catch (e) {
        if (!(e instanceof ApiError) || e.status !== 409) throw e;
      }
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
   * This device's tenure of the match: the epoch the server granted, or 0 until it has.
   * Every push is stamped with it. A device that never reached the server -- a match run
   * with no LAN -- claims on its first successful contact and pushes after.
   */
  epoch = $state(0);
  /** Set when another live device holds this match. The score keeper decides what to do. */
  contested = $state<Client | null>(null);
  /** How many of this device's events the server set aside when it found them stale. */
  quarantined = $state(0);

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
    // Claimed as soon as it is opened, backlog or not, so a second device opening the
    // same mat is told this one is here rather than finding the match free.
    await this.claim();
    await this.flush();
    if (this.sync === 'idle') void flushAll(this.matchId);
    this.start();
  }

  /**
   * Asks the server for the match. Granted when nobody holds it, or the holder is this
   * device, or the holder has gone quiet; refused when another live device has it, in
   * which case `contested` names it and nothing is pushed until the score keeper takes
   * over or walks away.
   */
  private async claim(force = false): Promise<boolean> {
    if (this.epoch > 0 && !force) return true;
    try {
      const res = await api.claim(this.matchId, clientId(), force);
      this.epoch = res.epoch;
      this.contested = null;
      return true;
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        const holder = (e.body as { holder?: Client } | null)?.holder ?? null;
        this.contested = holder ?? { id: '?', role: 'scorekeeper', name: 'another device', lastSeen: '', alive: true };
        return false;
      }
      // Offline. The claim happens on the first contact that works.
      return false;
    }
  }

  /** The score keeper has chosen to take the match from the device that holds it. */
  async takeOver(): Promise<void> {
    if (await this.claim(true)) await this.flush();
  }

  /**
   * The graceful half of handover: everything this device holds goes to the server, then
   * the match is let go, so the next device claims it without having to take it over.
   */
  async release(): Promise<void> {
    this.stop();
    await this.flush();
    if (this.epoch > 0 && this.sync === 'idle') {
      try {
        await api.releaseClaim(this.matchId, clientId());
      } catch {
        // The server will treat this device as gone soon enough.
      }
    }
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
    if (this.sync === 'pushing' || this.sync === 'stale') return;
    const batch = this.outstanding();
    this.pendingCount = batch.length;
    if (batch.length === 0) {
      if (this.sync !== 'idle') this.sync = 'idle';
      return;
    }
    // Nothing is pushed under nobody's name: the claim comes first, and a refused claim
    // leaves the backlog here for the score keeper to decide about.
    if (!(await this.claim())) {
      if (!this.contested) this.sync = 'offline';
      return;
    }
    this.sync = 'pushing';
    try {
      const res = await api.pushEvents(this.matchId, batch, { client: clientId(), epoch: this.epoch });
      for (const e of batch) this.pushed.add(e.seq);
      this.check(res.state);
      this.pendingCount = 0;
      this.sync = 'idle';
      // Reaching the server with this match's backlog means the LAN is back: the matches
      // finished before it are waiting too.
      void flushAll(this.matchId);
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        // Another device has taken this match over. The server has set these events
        // aside for the organizer; this device stops writing and says so.
        this.quarantined = batch.length;
        this.sync = 'stale';
        this.stop();
        return;
      }
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
