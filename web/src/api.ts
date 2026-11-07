import type { Event, State } from './lib/match';
import type {
  Address,
  Break,
  Client,
  Competitor,
  DisciplineSummary,
  EventInfo,
  EventSignupPreview,
  EventSignupReady,
  EventView,
  ForecastView,
  MatsView,
  PeopleView,
  PersonView,
  Prep,
  Presence,
  ReportView,
  SignupPreview,
  SignupReady,
  SignupUpload,
  Snapshot,
  StaffSuggestion,
  StaffingView,
  SuggestionView,
  Timings,
} from './api-types';
import { apiBase } from './lib/paths';
import { discipline } from './router.svelte';

export type * from './api-types';

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
    let parsed: unknown = text;
    let message = text;
    try {
      parsed = JSON.parse(text);
      message = (parsed as { error?: string } | null)?.error ?? text;
    } catch {
      // Not JSON; the raw body is the best message available.
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
     * saves mats and pool sizes without touching what the eliminations were set to, and the
     * eliminations panel leaves the capacity alone.
     */
    saveTournament: (mats: number, minPoolSize: number, maxPoolSize: number, elimMats?: number, capacity?: number, setUp?: boolean) =>
      req<unknown>('PUT', `${base()}/tournament`, {
        mats,
        minPoolSize,
        maxPoolSize,
        ...(elimMats === undefined ? {} : { elimMats }),
        ...(capacity === undefined ? {} : { capacity }),
        ...(setUp === undefined ? {} : { setUp }),
      }),
    /**
     * The welcome message, the agenda, the wifi and the signup settings, through the
     * discipline: the event keeps the day. Written from the admin view only.
     */
    saveEvent: (event: EventInfo) => req<EventInfo>('PUT', `${base()}/event`, event),
    /** What is still missing before the signup files can go out. */
    signupReady: () => req<SignupReady>('GET', `${base()}/signup/ready`),
    /**
     * What an import would do. Writes nothing: the organizer looks first, and confirming
     * runs the same check again on the server rather than trusting what came back here.
     */
    previewSignups: (files: SignupUpload[]) =>
      req<SignupPreview>('POST', `${base()}/signup/preview`, { files }),
    importSignups: (files: SignupUpload[]) =>
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
     * or silently dropped. No stamp is the anonymous path, for the tests: refused while
     * a device holds the match.
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
    /** Who holds a match now: how a score keeper finds out it has been taken over. */
    claimOf: (matchId: string) =>
      req<{ client: string; epoch: number; holder?: Client }>('GET', `${base()}/matches/${matchId}/claim`),
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
  /** Puts the whole event aside under deleted/ and starts a new, empty one (#141). */
  deleteEvent: () => req<{ deleted: string }>('DELETE', '/api/event'),
  addresses: () => req<Address[]>('GET', '/api/addresses'),

  /** Everybody in the event, and who might be the same person. */
  people: () => req<PeopleView>('GET', '/api/people'),

  // The event's staff (phase 5).
  staff: () => req<StaffingView>('GET', '/api/staff'),
  /** want: somebody already in the event this member is. */
  addStaff: (m: { name: string; club: string; roles: string[]; disciplines: string[]; want?: string }) =>
    req<StaffingView>('POST', '/api/staff', m),
  updateStaff: (id: string, patch: { name?: string; club?: string; roles?: string[]; disciplines?: string[] }) =>
    req<StaffingView>('PATCH', `/api/staff/${id}`, patch),
  /** Takes somebody off the event's staff altogether. */
  removeMember: (id: string) => req<StaffingView>('DELETE', `/api/staff/${id}`),
  setCrew: (crew: Record<string, number>) => req<StaffingView>('PUT', '/api/staff/crew', crew),
  /** Puts somebody in a slot by hand, or empties it with staff "". */
  assign: (a: { item: string; role: string; slot: number; staff: string }) =>
    req<StaffingView>('PUT', '/api/staff/assignments', a),
  suggestStaff: () => req<StaffSuggestion>('POST', '/api/staff/suggest'),
  applyStaff: (signature: string) => req<StaffingView>('POST', '/api/staff/apply', { signature }),
  /** One person's entries across the event, by any id they have had. */
  person: (id: string) => req<PersonView>('GET', `/api/people/${encodeURIComponent(id)}`),
  mergePerson: (id: string, into: string) => req<PeopleView>('POST', `/api/people/${id}/merge`, { into }),
  unmergePerson: (id: string) => req<PeopleView>('POST', `/api/people/${id}/unmerge`, {}),
  keepApart: (id: string, other: string) => req<PeopleView>('POST', `/api/people/${id}/apart`, { other }),

  /** The signup for the whole event: one file out, one folder back, every discipline its share. */
  eventSignupReady: () => req<EventSignupReady>('GET', '/api/event/signup/ready'),
  previewEventSignups: (files: SignupUpload[]) =>
    req<EventSignupPreview>('POST', '/api/event/signup/preview', { files }),
  importEventSignups: (files: SignupUpload[]) =>
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
  /** A discipline's identifier: its address, and its name in the signup files (#153). */
  setIdentifier: (slug: string, identifier: string) =>
    req<DisciplineSummary>('PUT', `/api/disciplines/${slug}/identifier`, { identifier }),
  /** Takes a discipline out of the event. Its folder is kept, under retired/. */
  retireDiscipline: (slug: string) => req<{ retired: string }>('DELETE', `/api/disciplines/${slug}`),
  /** Reads a discipline's files again, after a hand edit that stopped it loading was fixed. */
  reloadDiscipline: (slug: string) => req<DisciplineSummary>('POST', `/api/disciplines/${slug}/reload`),

  /** The hall's mats: every queue across disciplines, and what each mat is running. */
  mats: () => req<MatsView>('GET', '/api/mats'),
  /** One mat's name and the times it is not available (#123). */
  setMat: (mat: number, setting: { name: string; away: { from: string; to: string }[] }) =>
    req<MatsView>('PUT', `/api/mats/${mat}`, setting),
  /** What every mat screen shows of the matches to come. */
  setScreens: (screens: { upcoming: 'bottom' | 'list' | 'none' }) => req<MatsView>('PUT', '/api/screens', screens),
  /** How many mats the hall has. */
  setMats: (count: number) => req<MatsView>('PUT', '/api/mats', { count }),
  /** Moves a work item: to a place on a mat, or a step along its own. */
  moveItem: (id: string, to: { mat: number; index?: number } | { move: 'up' | 'down' }) =>
    req<MatsView>('PATCH', `/api/plan/items/${id.split('/').map(encodeURIComponent).join('/')}`, to),
  /** Pins or unpins a work item, holds it until a time ("" lets it go), or lets a bronze match run with the final. */
  flagItem: (id: string, flags: { pinned?: boolean; notBefore?: string; withFinal?: boolean }) =>
    req<MatsView>('PATCH', `/api/plan/items/${id.split('/').map(encodeURIComponent).join('/')}`, flags),

  // The plan's times (phase 4).
  forecast: () => req<ForecastView>('GET', '/api/forecast'),
  setTimings: (t: Timings) => req<ForecastView>('PUT', '/api/plan/timings', t),
  learnTimings: () => req<ForecastView>('POST', '/api/plan/timings/learn'),
  keepTimings: () => req<{ file: string; timings: Timings }>('POST', '/api/plan/timings/default'),
  setExpected: (expected: Record<string, number>) => req<ForecastView>('PUT', '/api/plan/expected', expected),
  /** Which block of the day each discipline runs in (#136). */
  setSessions: (sessions: Record<string, number>) => req<ForecastView>('PUT', '/api/plan/sessions', sessions),
  /** Holds every final to the end of the day, one after another on mat 1. */
  setFinalsLast: (last: boolean) => req<ForecastView>('PUT', '/api/plan/finals', { last }),
  /** The hall's breaks, the whole list (#149). */
  setBreaks: (breaks: Break[]) => req<ForecastView>('PUT', '/api/plan/breaks', breaks),
  /** The gear check and staff briefing of the disciplines named. */
  setPrep: (prep: Record<string, Prep>) => req<ForecastView>('PUT', '/api/plan/prep', prep),
  /** When the disciplines named start at the earliest; "" takes it off. */
  setStarts: (starts: Record<string, string>) => req<ForecastView>('PUT', '/api/plan/starts', starts),
  report: () => req<ReportView>('GET', '/api/plan/report'),
  setAnomaly: (key: string, anomaly: boolean) => req<ReportView>('PUT', '/api/plan/anomalies', { key, anomaly }),
  /** A suggested plan. Writes nothing. */
  suggest: () => req<SuggestionView>('POST', '/api/plan/suggest'),
  /** Applies the suggestion with this signature; a 409 carries a fresh one. */
  applySuggestion: (signature: string) => req<ForecastView>('POST', '/api/plan/apply', { signature }),

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
