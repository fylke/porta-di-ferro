import type { Event, Options, Ruleset, State } from './lib/match';

/**
 * What the server says, in the shapes it says it: the wire types of every call in api.ts.
 * Go's own are in internal/hall and internal/http, and a change to one is a change to both.
 */

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
  /** When the match started and ended, by its log (phase 4). */
  startedAt?: string;
  endedAt?: string;
  /** When the forecast expects a match still to come to start, in the hall's zone. */
  eta?: string;
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
  /** A display's assignment: "mat/1", "mats", "mats/1,2", "d/{slug}/roster". Empty until set. */
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
  /** What the organizer calls the mat, and when it is not available (#123). */
  name?: string;
  away?: { from: string; to: string }[];
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
  /**
   * "planned" is an item not drawn yet (phase 4); "queued" waits for an earlier block of
   * the day to finish (#136).
   */
  status: 'waiting' | 'ready' | 'running' | 'done' | 'planned' | 'queued';
  done: number;
  total: number;
  movable: boolean;
  projected?: boolean;
  /** Put on its mat by hand: a suggestion keeps it there. */
  pinned?: boolean;
  /** The earliest it may start, "16:30". */
  notBefore?: string;
  /** A bronze match let run at the same time as the final (#149). */
  withFinal?: boolean;
}

/** How long things take, in seconds, and the day's start and close as "HH:MM" (phase 4). */
export interface Timings {
  match?: number;
  changeover?: number;
  beforeElims?: number;
  start?: string;
  close?: string;
}

/** One work item's forecast and planned times, as RFC 3339 in the hall's zone. */
export interface ItemTimes {
  id: string;
  mat: number;
  start: string;
  end: string;
  plannedStart: string;
  plannedEnd: string;
}

export interface Warning {
  kind: 'overlap' | 'dependency' | 'overrun';
  items?: string[];
  person?: string;
  personName?: string;
  from?: string;
  to?: string;
}

/**
 * One row of the programme, generated from the plan (#149): a discipline's gear check,
 * staff briefing, pools, eliminations or final, or a break for the whole hall -- which has
 * no discipline, and its own name.
 */
export interface ProgrammeRow {
  discipline: string;
  name: string;
  stage: 'gear-check' | 'briefing' | 'pools' | 'eliminations' | 'final' | 'break';
  start: string;
  end: string;
  mats: number[];
  done?: boolean;
}

/** The plan re-timed against what has happened. */
export interface ForecastView {
  items: ItemTimes[];
  end?: string;
  plannedEnd?: string;
  pace: { mat: number; match: number; changeover: number; samples: number }[];
  warnings: Warning[];
  programme: ProgrammeRow[];
  timings: Timings;
  live: boolean;
  /** How many each discipline expects, by slug. */
  expected?: Record<string, number>;
  /** The block of the day each discipline runs in, and whether every final is held to the end (#136). */
  sessions?: Record<string, number>;
  finalsLast?: boolean;
  /** The hall's breaks, each discipline's gear check and briefing, and its earliest start (#149). */
  breaks: Break[];
  prep?: Record<string, Prep>;
  starts?: Record<string, string>;
  /** What a day usually has and this plan does not. Pointed out, never required. */
  gaps: { kind: 'lunch' | 'gear-check' | 'briefing'; discipline?: string; name?: string }[];
  /** Each discipline's mats and what its numbers recommend, by slug (#153). */
  mats?: Record<string, MatsAdvice>;
  /** What that comes to for the hall: its busiest block, side by side. */
  matsRecommended?: number;
}

/** How many mats a discipline's pools use, and how many its numbers recommend. */
export interface MatsAdvice {
  mats: number;
  /** Absent until there is a number to go on. */
  recommended?: number;
  /** What it expects or, without that, the ceiling on its entries. */
  for?: number;
  minPoolSize: number;
  maxPoolSize: number;
  /** How many pools are drawn. */
  pools?: number;
}

/** A time the whole hall stops, "12:00" to "13:00". */
export interface Break {
  label?: string;
  from: string;
  to: string;
}

/** How long a discipline's gear check and staff briefing take, in seconds. */
export interface Prep {
  gearCheck?: number;
  briefing?: number;
}

/** A suggested plan: what moves, and when the day would end with it and without it. */
export interface SuggestionView {
  moves: { id: string; fromMat: number; fromPosition: number; toMat: number; toPosition: number; start: string }[];
  end: string;
  before: string;
  signature: string;
}

/** The measured day: every fenced match, and what a template learned from it would say. */
export interface ReportView {
  matches: {
    key: string;
    discipline: string;
    disciplineName: string;
    match: string;
    red: string;
    blue: string;
    mat: number;
    started: string;
    seconds: number;
    changeover: number;
    anomaly: boolean;
  }[];
  match: number;
  changeover: number;
  samples: number;
}

/** The hall: every mat, and every work item placed on them. */
export interface MatsView {
  mats: MatView[];
  items: ItemView[];
  /** What the mat screens show of the matches to come (#110). */
  upcoming?: 'bottom' | 'list' | 'none';
  /** The event's identity (#141). */
  event?: string;
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
  /** The organizer finished its setup, or it has a draw (#149). */
  setUp: boolean;
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
  /** The event's staff, for the landing page's list of everyone (#135). */
  staff?: { id: string; name: string; club?: string; person?: string }[];
  name: string;
  info: EventInfo;
  /** The event's identity: a new one after a delete (#141). */
  id?: string;
  /** event.json could not be read; the disciplines run regardless. */
  infoError?: string;
  disciplines: DisciplineSummary[];
  dir: string;
  /** The fencing part of the day, as the forecast has it (phase 4). */
  programme?: ProgrammeRow[];
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
export interface SignupUpload {
  source: string;
  body: string;
}

export interface SignupInfo {
  definitionId?: string;
  name?: string;
  venue?: string;
  date?: string;
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
  /** Who they are across the event (phase 5). */
  person?: string;
  /** The disciplines, by slug, they will work. Empty or absent is any. */
  disciplines?: string[];
}

/** One member working one role on one work item (phase 5). */
export interface Assignment {
  item: string;
  role: string;
  slot: number;
  staff: string;
  /** Chosen by hand: a suggestion keeps it. */
  pinned?: boolean;
}

/** A work item as the staff panel lists it. */
export interface StaffItem {
  id: string;
  discipline: string;
  disciplineName: string;
  kind: 'pool' | 'eliminations' | 'bronze' | 'final';
  number?: number;
  mat: number;
  start: string;
  end: string;
  started?: boolean;
  projected?: boolean;
}

/** The event's staff, the crew each mat needs, who works what, and what is missing or wrong. */
export interface StaffingView {
  members: StaffMember[];
  crew: Record<string, number>;
  assignments: Assignment[];
  items: StaffItem[];
  short: { item: string; role: string; slot: number }[];
  warnings: { kind: 'fencing' | 'double' | 'role' | 'discipline'; item: string; role: string; staff: string; other?: string }[];
  /** Members on call as physicians, by id. */
  physicians: string[];
  /** Members whose person may be somebody else in the event too, by id (#133). */
  unsure?: string[];
  /** The fewest people who could staff the plan: a crew on every mat at the busiest moment. */
  needed: number;
}

export interface StaffSuggestion {
  assignments: Assignment[];
  changes: number;
  short: { item: string; role: string; slot: number }[];
  signature: string;
}

/** One stretch of work for a member. */
export interface Duty {
  item: string;
  discipline: string;
  disciplineName: string;
  kind: 'pool' | 'eliminations' | 'bronze' | 'final';
  number?: number;
  mat: number;
  role: string;
  start: string;
  end: string;
}

/** One response file as the organizer sees it before deciding. */
export interface SignupRow {
  source: string;
  verdict: 'new' | 'staff' | 'already' | 'repeat' | 'other-event' | 'not-here' | 'unknown' | 'invalid';
  name?: string;
  club?: string;
  /** Who they are across responses (#142). For the organizer's eyes only. */
  email?: string;
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
  /** The disciplines a participant chooses from, by name (#142). */
  tournaments: { id: string; label: string; capacity?: number; startsAt?: string }[];
  filename: string;
  definition: string;
}

/** One discipline's part in the event's signup import (phase 3). */
export interface SignupShare {
  discipline: string;
  name: string;
  adding: number;
  addingStaff: number;
  capacity?: string[];
  poolsDrawn?: boolean;
  error?: string;
}

export type EventSignupReady = SignupReady;

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
  /** Their work as staff (phase 5), and whether they are on call as a physician. */
  duties?: Duty[];
  physician?: boolean;
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
    /** How many this discipline takes, or 0 for no limit (#142). */
    capacity?: number;
    /** The organizer finished its setup (#149). */
    setUp?: boolean;
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
