import type { Event, Ruleset, State } from './lib/match';

export interface Competitor {
  id: string;
  name: string;
  club: string;
  withdrawn: boolean;
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
  addCompetitor: (name: string, club: string) =>
    req<Competitor>('POST', '/api/competitors', { name, club }),
  updateCompetitor: (id: string, patch: Partial<Pick<Competitor, 'name' | 'club' | 'withdrawn'>>) =>
    req<{ ok: boolean }>('PATCH', `/api/competitors/${id}`, patch),
  removeCompetitor: (id: string) => req<{ ok: boolean }>('DELETE', `/api/competitors/${id}`),
  saveTournament: (mats: number, minPoolSize: number, maxPoolSize: number) =>
    req<unknown>('PUT', '/api/tournament', { mats, minPoolSize, maxPoolSize }),
  generatePools: () => req<unknown>('POST', '/api/tournament/pools'),
  events: (matchId: string, after = 0) =>
    req<Event[]>('GET', `/api/matches/${matchId}/events?after=${after}`),
  pushEvents: (matchId: string, events: Event[]) =>
    req<{ written: number; state: State; lastSeq: number }>(
      'POST',
      `/api/matches/${matchId}/events`,
      events,
    ),
};
