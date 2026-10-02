import type { Event, Options, Ruleset, State } from './lib/match';
import { apiBase } from './lib/paths';
import { discipline } from './router.svelte';

export interface Competitor {
  id: string;
  name: string;
  club: string;
  withdrawn: boolean;
  /** Who they are across the event (phase 3): their page is /who/{person}. */
  person?: string;
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
  /** Which discipline the score keeper's match is in: match ids repeat across them. */
  discipline?: string;
  /** A display's assignment: "mat/1", "mats", "audience/2", "d/{slug}/roster". Empty until set. */
  target?: string;
  lastSeen: string;
  alive: boolean;
}

/** An event a device wrote after its match had been handed to another. */
export interface Quarantined {
  /** The discipline the match is in, in an event's list. */
  discipline?: string;
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

/** Which discipline a snapshot is: its name, its address in the event and its folder. */
export interface Instance {
  name: string;
  /** The discipline's address: /d/{slug}/ and /api/d/{slug}/. Never changes once made. */
  slug?: string;
  dir: string;
  /** Where its pages are, relative to the event's address: "/d/open-sabre/". */
  url: string;
}

/** One match in a mat's queue, with its discipline and the names in it (phase 2). */
export interface Slot {
  discipline: string;
  disciplineName: string;
  item: string;
  match: MatchView;
  red: string;
  blue: string;
}

/** One physical mat of the event: everything queued on it, and what it is running. */
export interface MatView {
  mat: number;
  /** The match the mat is on: its score keeper's, or the head item's next. */
  current?: Slot;
  /** Every match of every item on the mat, in running order, finished ones included. */
  queue: Slot[];
  /** Matches of the head item still waiting on a feeder. */
  waiting?: Slot[];
}

/** One work item on the event's mats, for the mat board. */
export interface ItemView {
  id: string;
  discipline: string;
  disciplineName: string;
  kind: 'pool' | 'eliminations' | 'bronze' | 'final';
  number?: number;
  mat: number;
  position: number;
  status: 'waiting' | 'ready' | 'running' | 'done';
  done: number;
  total: number;
  movable: boolean;
}

/** The hall: every mat, and every work item placed on them. */
export interface MatsView {
  mats: MatView[];
  items: ItemView[];
}

/** What one of a discipline's mats is running or has up next, for the event's pages. */
export interface MatSummary {
  mat: number;
  match?: string;
  status?: 'pending' | 'running' | 'complete';
  pool?: number;
  round?: string;
  red?: string;
  blue?: string;
  redColour?: string;
  blueColour?: string;
  redScore: number;
  blueScore: number;
}

/** One discipline, compactly: what the event's landing page and admin show of it. */
export interface DisciplineSummary {
  slug: string;
  name: string;
  url: string;
  /** Set when the discipline could not be read; the rest is then its last known summary. */
  error?: string;
  stale?: boolean;
  stage: 'setup' | 'pools' | 'waiting' | 'eliminations' | 'done';
  competitors: number;
  pools: number;
  matchesDone: number;
  matchesTotal: number;
  podium?: { first: string; second: string; third: string };
  mats: MatSummary[];
  entrants: { id: string; name: string; club?: string; person?: string }[];
}

/** The whole event: the day around the fencing, and every discipline in it. */
export interface EventView {
  name: string;
  info: EventInfo;
  /** event.json could not be read; the disciplines run regardless. */
  infoError?: string;
  disciplines: DisciplineSummary[];
  dir: string;
}

/** One line of the day's agenda. `at` is free text: "after the pools" is a valid time. */
export interface ScheduleItem {
  at?: string;
  label: string;
  kind?: 'discipline' | 'break' | '';
}

/** The venue network, for the code on the printed info sheet. */
export interface Wifi {
  ssid?: string;
  password?: string;
  security?: 'WPA' | 'WEP' | 'nopass';
  hidden?: boolean;
}

/**
 * The day around the tournament (issue #98): what the participant view and the printed
 * info sheet put around the fencing. None of it reaches a result.
 */
/** What the offline signup files carry about the event (issue #91). */
export interface SignupInfo {
  definitionId?: string;
  name?: string;
  venue?: string;
  date?: string;
  /** Which programme row this discipline is. The discipline's own; never the event's. */
  tournament?: string;
  contact?: boolean;
}

export interface EventInfo {
  welcome?: string;
  schedule?: ScheduleItem[];
  wifi?: Wifi;
  signup?: SignupInfo;
}

/** Somebody who offered to work this discipline rather than fence in it (issue #5). */
export interface StaffMember {
  id: string;
  name: string;
  club?: string;
  /** "head-ref", "assistant-ref", "score-keeper", "physician". */
  roles: string[];
  signup?: string;
}

/** One response file as the organizer sees it before deciding. */
export interface SignupRow {
  source: string;
  verdict: 'new' | 'staff' | 'already' | 'repeat' | 'other-event' | 'not-here' | 'unknown' | 'invalid';
  name?: string;
  club?: string;
  contact?: string;
  submissionId?: string;
  entries?: string[];
  /** The offer to work (issue #5): which disciplines, in which roles. */
  staffing?: string[];
  roles?: string[];
  problem?: string;
}

export interface SignupPreview {
  rows: SignupRow[];
  adding: number;
  addingStaff: number;
  capacity?: string[];
  tournament?: string;
  poolsDrawn: boolean;
}

export interface SignupReady {
  missing: string[];
  tournaments: { id: string; label: string; capacity?: number }[];
  filename: string;
  definition: string;
}

/** Which discipline takes which programme row, in the event's signup (phase 3). */
export interface SignupShare {
  discipline: string;
  name: string;
  tournament: string;
  /** The discipline chose the row; otherwise it was matched by name. */
  chosen: boolean;
  adding: number;
  addingStaff: number;
  capacity?: string[];
  poolsDrawn?: boolean;
  error?: string;
}

export interface EventSignupReady extends SignupReady {
  disciplines: SignupShare[];
  /** Programme rows no discipline takes: whoever enters them is imported nowhere. */
  unclaimed: { id: string; label: string }[];
}

/** One response, and the disciplines it adds to. */
export interface EventSignupRow extends SignupRow {
  into?: { discipline: string; name: string; verdict: 'new' | 'staff' }[];
}

export interface EventSignupPreview {
  rows: EventSignupRow[];
  disciplines: SignupShare[];
  adding: number;
  addingStaff: number;
}

/** One of a person's entries, in one discipline. */
export interface PersonEntry {
  discipline: string;
  disciplineName: string;
  competitor: string;
  name: string;
  club?: string;
  withdrawn?: boolean;
}

/** One person of the event's, with every entry of theirs. */
export interface PersonView {
  id: string;
  name: string;
  club?: string;
  entries: PersonEntry[];
  /** People merged into this one, whose merge can be undone. */
  mergedFrom?: { id: string; name: string }[];
}

export interface PeopleView {
  people: PersonView[];
  /** Groups of people who might be one, by id, for the organizer to decide. */
  duplicates: string[][];
}

export interface Snapshot {
  competitors: Competitor[];
  /** Everyone ranked across the pools by the pool chain: the seeding for the eliminations. */
  overall: Standing[];
  poolsComplete: boolean;
  /** How many mats the eliminations will be drawn across as things stand. */
  elimMats: number;
  /** What the rule would pick on its own: the fewest that costs the bracket no extra pass. */
  elimMatsSuggested: number;
  bracket?: BracketView;
  instance: Instance;
  /**
   * How many physical mats the event has. Every mat in this snapshot is then one of the
   * event's, placed by its plan. Absent for a discipline on its own.
   */
  eventMats?: number;
  tournament: {
    event?: EventInfo;
    mats: number;
    minPoolSize: number;
    maxPoolSize: number;
    /** The organizer's answer for the eliminations, or 0 to take the suggestion. */
    elimMats?: number;
    seed: number;
    pools: unknown[];
    generatedAt?: string;
    violations?: string[];
    staff?: StaffMember[];
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

/**
 * Everything one discipline answers, at the address it is mounted under. `base` is read on
 * every call rather than once, so the page's discipline -- from its own address -- is the
 * one asked, and a score keeper's backlog can be sent to the discipline it was scored in
 * whatever page is open (apiIn).
 */
function disciplineApi(base: () => string) {
  return {
    state: () => req<Snapshot>('GET', `${base()}/state`),
    /** person: somebody already in the event this entry is, from the desk's suggestion. */
    addCompetitor: (name: string, club: string, person?: string) =>
      req<Competitor>('POST', `${base()}/competitors`, { name, club, ...(person ? { person } : {}) }),
    updateCompetitor: (id: string, patch: Partial<Pick<Competitor, 'name' | 'club' | 'withdrawn'>>) =>
      req<{ ok: boolean }>('PATCH', `${base()}/competitors/${id}`, patch),
    removeCompetitor: (id: string) => req<{ ok: boolean }>('DELETE', `${base()}/competitors/${id}`),
    /**
     * Left out rather than sent as 0 when the caller has no opinion on it: the setup screen
     * saves mats and pool sizes without touching what the eliminations were set to.
     */
    saveTournament: (mats: number, minPoolSize: number, maxPoolSize: number, elimMats?: number) =>
      req<unknown>('PUT', `${base()}/tournament`, {
        mats,
        minPoolSize,
        maxPoolSize,
        ...(elimMats === undefined ? {} : { elimMats }),
      }),
    /**
     * The welcome message, the agenda, the wifi and the signup settings, through the
     * discipline: the event keeps the day, and the discipline keeps which programme row it
     * is. Written from the admin view only.
     */
    saveEvent: (event: EventInfo) => req<EventInfo>('PUT', `${base()}/event`, event),
    /** What is still missing before the signup files can go out. */
    signupReady: () => req<SignupReady>('GET', `${base()}/signup/ready`),
    /**
     * What an import would do. Writes nothing: the organizer looks first, and confirming
     * runs the same check again on the server rather than trusting what came back here.
     */
    previewSignups: (files: { source: string; body: string }[]) =>
      req<SignupPreview>('POST', `${base()}/signup/preview`, { files }),
    importSignups: (files: { source: string; body: string }[]) =>
      req<{ added: number; addedStaff: number; preview: SignupPreview }>('POST', `${base()}/signup/import`, {
        files,
      }),
    /** Takes somebody off the staff. They are in no match, so the draw does not stop it. */
    removeStaff: (id: string) => req<{ ok: boolean }>('DELETE', `${base()}/staff/${id}`),
    generatePools: () => req<unknown>('POST', `${base()}/tournament/pools`),
    drawBracket: () => req<unknown>('POST', `${base()}/tournament/bracket`),
    movePool: (number: number, mat: number) =>
      req<unknown>('PATCH', `${base()}/tournament/pools/${number}`, { mat }),
    reorderPool: (number: number, move: 'up' | 'down') =>
      req<unknown>('PATCH', `${base()}/tournament/pools/${number}`, { move }),
    events: (matchId: string, after = 0) =>
      req<Event[]>('GET', `${base()}/matches/${matchId}/events?after=${after}`),
    /**
     * The organizer's editor saving: the whole log, rewritten. The server keeps the version
     * being replaced as a backup and tells a score keeper holding the match to reload.
     */
    replaceEvents: (matchId: string, events: Event[]) =>
      req<{ backup: string; state: State }>('PUT', `${base()}/matches/${matchId}/events`, events),
    backups: (matchId: string) => req<string[]>('GET', `${base()}/matches/${matchId}/backups`),
    /**
     * Stamped with who is pushing and which epoch it holds, so a device whose match has
     * been handed to another is told so -- a 409 -- rather than having its backlog appended
     * or silently dropped. No stamp is the anonymous path: the tests and paper entry.
     */
    pushEvents: (matchId: string, events: Event[], writer?: { client: string; epoch: number }) =>
      req<{ written: number; state: State; lastSeq: number }>(
        'POST',
        `${base()}/matches/${matchId}/events`,
        events,
        writer ? { 'X-Porta-Client': writer.client, 'X-Porta-Epoch': String(writer.epoch) } : {},
      ),
    claim: (matchId: string, client: string, force = false) =>
      req<{ epoch: number; tookOverFrom?: string }>('POST', `${base()}/matches/${matchId}/claim`, {
        client,
        force,
      }),
    releaseClaim: (matchId: string, client: string) =>
      req<{ ok: boolean }>('DELETE', `${base()}/matches/${matchId}/claim?client=${encodeURIComponent(client)}`),
    discardQuarantine: (matchId: string) => req<{ ok: boolean }>('DELETE', `${base()}/quarantine/${matchId}`),
  };
}

/** One discipline's API by name, whatever page is open. */
export function apiIn(slug: string) {
  return disciplineApi(() => apiBase(slug));
}

/**
 * The page's discipline -- the one in its address, or the event's only one -- and the
 * event's own calls beside it.
 */
export const api = {
  ...disciplineApi(() => apiBase(discipline())),

  /** The whole event: the day, and a summary of every discipline. */
  event: () => req<EventView>('GET', '/api/event'),
  /** The welcome, the programme, the wifi and the signup settings, typed once for the event. */
  saveEventInfo: (info: EventInfo) => req<EventInfo>('PUT', '/api/event/info', info),
  addresses: () => req<Address[]>('GET', '/api/addresses'),

  /** Everybody in the event, and who might be the same person. */
  people: () => req<PeopleView>('GET', '/api/people'),
  /** One person's entries across the event, by any id they have had. */
  person: (id: string) => req<PersonView>('GET', `/api/people/${encodeURIComponent(id)}`),
  mergePerson: (id: string, into: string) => req<PeopleView>('POST', `/api/people/${id}/merge`, { into }),
  unmergePerson: (id: string) => req<PeopleView>('POST', `/api/people/${id}/unmerge`, {}),
  keepApart: (id: string, other: string) => req<PeopleView>('POST', `/api/people/${id}/apart`, { other }),

  /** The signup for the whole event: one file out, one folder back, every discipline its share. */
  eventSignupReady: () => req<EventSignupReady>('GET', '/api/event/signup/ready'),
  setSignupRows: (rows: Record<string, string>) => req<EventSignupReady>('PUT', '/api/event/signup/rows', rows),
  previewEventSignups: (files: { source: string; body: string }[]) =>
    req<EventSignupPreview>('POST', '/api/event/signup/preview', { files }),
  importEventSignups: (files: { source: string; body: string }[]) =>
    req<{ added: number; addedStaff: number; preview: EventSignupPreview; error?: string }>(
      'POST',
      '/api/event/signup/import',
      { files },
    ),
  /** The preloaded list of disciplines the organizer picks from rather than types out. */
  presets: () => req<string[]>('GET', '/api/disciplines/presets'),
  /** Another discipline in the event: another folder and tournament at the same address (#4). */
  addDiscipline: (name: string) => req<DisciplineSummary>('POST', '/api/disciplines', { name }),
  renameDiscipline: (slug: string, name: string) =>
    req<DisciplineSummary>('PATCH', `/api/disciplines/${slug}`, { name }),
  /** Takes a discipline out of the event. Its folder is kept, under retired/. */
  retireDiscipline: (slug: string) => req<{ retired: string }>('DELETE', `/api/disciplines/${slug}`),
  /** Reads a discipline's files again, after a hand edit that stopped it loading was fixed. */
  reloadDiscipline: (slug: string) => req<DisciplineSummary>('POST', `/api/disciplines/${slug}/reload`),

  /** The hall's mats: every queue across disciplines, and what each mat is running. */
  mats: () => req<MatsView>('GET', '/api/mats'),
  /** How many mats the hall has. */
  setMats: (count: number) => req<MatsView>('PUT', '/api/mats', { count }),
  /** Moves a work item: to a place on a mat, or a step along its own. */
  moveItem: (id: string, to: { mat: number; index?: number } | { move: 'up' | 'down' }) =>
    req<MatsView>('PATCH', `/api/plan/items/${id.split('/').map(encodeURIComponent).join('/')}`, to),

  // The devices at the mats belong to the event, whatever discipline they are scoring.
  presence: () => req<Presence>('GET', '/api/presence'),
  register: (
    id: string,
    fields: { role: 'scorekeeper' | 'display'; name: string; mat?: number; match?: string; discipline?: string },
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
};
