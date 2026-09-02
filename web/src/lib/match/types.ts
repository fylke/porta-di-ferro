// The match engine's types, mirroring internal/match in Go (design decision 13).
//
// The shapes here are the wire format as well as the in-memory one: an Event is one line
// of a match's .ndjson log, and a State is what both implementations must agree on,
// field for field, against the shared vectors in testdata/vectors/.

export type Side = 'red' | 'blue';

export type EventType = 'exchange' | 'timer' | 'undo' | 'end';

export type Reason = '' | 'time' | 'point_cap' | 'penalty' | 'forfeit';

export type TimerAction = 'start' | 'stop' | 'resume';

export type Pending = 'none' | 'final_exchange' | 'point_cap' | 'penalty_cap';

/**
 * What the head referee announced for one competitor in one exchange.
 *
 * `value` is the raw assessed value, not the resulting score: the log stays raw and the
 * net is derived, so a match recorded under differential scoring stays interpretable
 * under additive scoring with no migration.
 *
 * `penalty` is how many levels this exchange applies. MVP only ever sends 0 or 1;
 * immediate escalation (Milestone 2) is the same field carrying 2 or 3.
 */
export interface Assessment {
  value: number;
  penalty: number;
}

export interface Exchange {
  red: Assessment;
  blue: Assessment;
}

export interface Timer {
  action: TimerAction;
}

export interface Undo {
  seq: number;
}

export interface End {
  reason: Reason;
  forfeiter?: Side;
}

/**
 * One line of a match log. The primary key is (match, seq), which is what makes a retried
 * push idempotent and needs no deduplication logic anywhere.
 *
 * `elapsedMs` is the match clock at the moment of the event. Storing it rather than
 * recomputing from wall-clock timestamps is what keeps replay deterministic.
 */
export interface Event {
  match?: string;
  seq: number;
  type: EventType;
  at?: string;
  elapsedMs: number;
  exchange?: Exchange;
  timer?: Timer;
  undo?: Undo;
  end?: End;
}

export interface Competitor {
  score: number;
  penalty: number;
}

export interface State {
  red: Competitor;
  blue: Competitor;
  exchanges: number;
  elapsedMs: number;
  running: boolean;
  ended: boolean;
  noMatchPoints: boolean;
  endReason: Reason;
  winner: Side | '';
  pending: Pending;
  lastSeq: number;
  undoableSeq: number;
}
