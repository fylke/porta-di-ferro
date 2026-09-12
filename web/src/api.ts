import type { Event, Options, Ruleset, State } from './lib/match';

export interface Competitor {
  id: string;
  name: string;
  club: string;
  withdrawn: boolean;
}

/**
 * One address on the organizer's PC that a client could open. The organizer's own browser
 * is on localhost, which is the one address no other device can reach, so the client URL
 * and the QR code are composed from this list rather than from window.location.
 */
export interface Address {
  ip: string;
  interface: string;
  private: boolean;
  /** The wifi network's name, when the adapter is wireless and the platform can say. */
  ssid?: string;
}

export interface MatchView {
  id: string;
  pool: number;
  order: number;
  mat: number;
  red: string;
  blue: string;
  state: State;
  status: 'pending' | 'running' | 'complete';
  /**
   * How long ago the server saw this match's last event, in milliseconds. Present only
   * while the clock is running. A display adds it to state.elapsedMs to place its clock
   * where the mat's actually is, instead of restarting from whenever the page loaded.
   */
  sinceMs?: number;
  /** How the match is presented: colours and display sides, read from the log. */
  options: Options;
}

export interface Standing {
  competitor: string;
  name: string;
  club: string;
  completed: number;
  wins: number;
  draws: number;
  losses: number;
  matchPoints: number;
  scored: number;
  conceded: number;
  matchPointIndex: number;
  victoryIndex: number;
  scoreIndex: number;
  receptionIndex: number;
  rank: number;
}

export interface PoolView {
  number: number;
  mat: number;
  /** The pool's place in its mat's queue. Pools arrive sorted by mat, then by this. */
  sequence: number;
  /** The organizer put it somewhere other than the default mapping would. */
  overridden: boolean;
  competitors: string[];
  matches: MatchView[];
  standings: Standing[];
  complete: boolean;
}

export interface Snapshot {
  competitors: Competitor[];
  tournament: {
    mats: number;
    minPoolSize: number;
    maxPoolSize: number;
    seed: number;
    pools: unknown[];
    generatedAt?: string;
    violations?: string[];
  };
  pools: PoolView[];
  ruleset: Ruleset;
  mats: Record<string, string>;
  dir: string;
}

async function req<T>(method: string, url: string, body?: unknown): Promise<T> {
  const res = await fetch(url, {
    method,
    headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!res.ok) {
    const text = await res.text();
    let message = text;
    try {
      message = (JSON.parse(text) as { error?: string }).error ?? text;
    } catch {
      // Not JSON; the raw body is the best message available.
    }
    throw new Error(message || `${method} ${url} failed with ${res.status}`);
  }
  return (await res.json()) as T;
}

export const api = {
  state: () => req<Snapshot>('GET', '/api/state'),
  addresses: () => req<Address[]>('GET', '/api/addresses'),
  addCompetitor: (name: string, club: string) =>
    req<Competitor>('POST', '/api/competitors', { name, club }),
  updateCompetitor: (id: string, patch: Partial<Pick<Competitor, 'name' | 'club' | 'withdrawn'>>) =>
    req<{ ok: boolean }>('PATCH', `/api/competitors/${id}`, patch),
  removeCompetitor: (id: string) => req<{ ok: boolean }>('DELETE', `/api/competitors/${id}`),
  saveTournament: (mats: number, minPoolSize: number, maxPoolSize: number) =>
    req<unknown>('PUT', '/api/tournament', { mats, minPoolSize, maxPoolSize }),
  generatePools: () => req<unknown>('POST', '/api/tournament/pools'),
  movePool: (number: number, mat: number) =>
    req<unknown>('PATCH', `/api/tournament/pools/${number}`, { mat }),
  reorderPool: (number: number, move: 'up' | 'down') =>
    req<unknown>('PATCH', `/api/tournament/pools/${number}`, { move }),
  events: (matchId: string, after = 0) =>
    req<Event[]>('GET', `/api/matches/${matchId}/events?after=${after}`),
  /**
   * The organizer's editor saving: the whole log, rewritten. The server keeps the version
   * being replaced as a backup and tells a score keeper holding the match to reload.
   */
  replaceEvents: (matchId: string, events: Event[]) =>
    req<{ backup: string; state: State }>('PUT', `/api/matches/${matchId}/events`, events),
  backups: (matchId: string) => req<string[]>('GET', `/api/matches/${matchId}/backups`),
  pushEvents: (matchId: string, events: Event[]) =>
    req<{ written: number; state: State; lastSeq: number }>(
      'POST',
      `/api/matches/${matchId}/events`,
      events,
    ),
};
