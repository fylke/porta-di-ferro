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
  /** 0 for a bracket match. */
  pool: number;
  order: number;
  mat: number;
  /** Empty on a bracket match whose feeder has not been decided yet. */
  red: string;
  blue: string;
  /** Set on a bracket match: which round, and the match's number within it. */
  round?: 'quarter' | 'semi' | 'bronze' | 'final';
  slot?: number;
  feedRed?: string;
  feedBlue?: string;
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

export interface Podium {
  first: string;
  second: string;
  third: string;
}

export interface BracketView {
  matches: MatchView[];
  podium: Podium;
  complete: boolean;
}

/** One device on the LAN, as the server sees it. */
export interface Client {
  id: string;
  role: 'scorekeeper' | 'display';
  name: string;
  mat?: number;
  match?: string;
  /** A display's assignment: "mat/1", "mats", "roster", "audience/2". Empty until set. */
  target?: string;
  lastSeen: string;
  alive: boolean;
}

/** An event a device wrote after its match had been handed to another. */
export interface Quarantined {
  match: string;
  client: string;
  clientName: string;
  receivedAt: string;
  event: Event;
}

export interface Presence {
  clients: Client[];
  quarantined: Quarantined[];
}

export interface Snapshot {
  competitors: Competitor[];
  /** Everyone ranked across the pools by the pool chain: the seeding for the eliminations. */
  overall: Standing[];
  poolsComplete: boolean;
  bracket?: BracketView;
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

/** Thrown for a non-2xx answer, with the status so a caller can tell a 409 from a 500. */
export class ApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly body: unknown,
  ) {
    super(message);
  }
}

async function req<T>(
  method: string,
  url: string,
  body?: unknown,
  headers: Record<string, string> = {},
): Promise<T> {
  const res = await fetch(url, {
    method,
    headers: { ...(body === undefined ? {} : { 'Content-Type': 'application/json' }), ...headers },
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
    let parsed: unknown = text;
    try {
      parsed = JSON.parse(text);
    } catch {
      // Not JSON.
    }
    throw new ApiError(message || `${method} ${url} failed with ${res.status}`, res.status, parsed);
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
  drawBracket: () => req<unknown>('POST', '/api/tournament/bracket'),
  movePool: (number: number, mat: number) =>
    req<unknown>('PATCH', `/api/tournament/pools/${number}`, { mat }),
  reorderPool: (number: number, move: 'up' | 'down') =>
    req<unknown>('PATCH', `/api/tournament/pools/${number}`, { move }),
  events: (matchId: string, after = 0) =>
    req<Event[]>('GET', `/api/matches/${matchId}/events?after=${after}`),
  /**
   * Stamped with who is pushing and which epoch it holds, so a device whose match has
   * been handed to another is told so -- a 409 -- rather than having its backlog appended
   * or silently dropped. No stamp is the anonymous path: the tests and paper entry.
   */
  pushEvents: (matchId: string, events: Event[], writer?: { client: string; epoch: number }) =>
    req<{ written: number; state: State; lastSeq: number }>(
      'POST',
      `/api/matches/${matchId}/events`,
      events,
      writer ? { 'X-Porta-Client': writer.client, 'X-Porta-Epoch': String(writer.epoch) } : {},
    ),
  claim: (matchId: string, client: string, force = false) =>
    req<{ epoch: number; tookOverFrom?: string }>('POST', `/api/matches/${matchId}/claim`, { client, force }),
  releaseClaim: (matchId: string, client: string) =>
    req<{ ok: boolean }>('DELETE', `/api/matches/${matchId}/claim?client=${encodeURIComponent(client)}`),

  presence: () => req<Presence>('GET', '/api/presence'),
  register: (
    id: string,
    fields: { role: 'scorekeeper' | 'display'; name: string; mat?: number; match?: string },
  ) => req<Client>('POST', `/api/clients/${id}`, fields),
  release: (id: string) => req<{ ok: boolean }>('POST', `/api/clients/${id}/release`),
  /** A goodbye from a page that is closing: fire and forget, the only kind it can send. */
  releaseBeacon: (id: string) => {
    try {
      navigator.sendBeacon(`/api/clients/${id}/release`, '');
    } catch {
      // The server notices on its own.
    }
  },
  assignDisplay: (id: string, target: string) =>
    req<{ target: string }>('PUT', `/api/clients/${id}/target`, { target }),
  discardQuarantine: (matchId: string) => req<{ ok: boolean }>('DELETE', `/api/quarantine/${matchId}`),
};
