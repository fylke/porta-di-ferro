/**
 * How a match's outcome is described to the score keeper: in the dialog that asks whether
 * to end it, and in the centre of the screen once it has ended.
 *
 * Pure over the engine's state and the log, so the same words appear in both places and
 * can be tested without a screen.
 */
import type { Event, Ruleset, Side, State } from './match';

export interface Names {
  red: string;
  blue: string;
}

export interface Outcome {
  /** The sentence that matters: who won, or who lost and why. */
  headline: string;
  /** The score line under it, when the headline is about the loser. */
  detail: string;
}

/** The last exchange still standing in the log -- the one whose penalty tripped a loss. */
export function lastStandingExchange(events: Event[]): Event | null {
  const undone = new Set<number>();
  for (const e of events) if (e.type === 'undo' && e.undo) undone.add(e.undo.seq);
  for (let i = events.length - 1; i >= 0; i--) {
    const e = events[i];
    if (e.type === 'exchange' && !undone.has(e.seq)) return e;
  }
  return null;
}

/**
 * A penalty loss, worded by how it was reached.
 *
 * "Bo wins 8-0" was true and useless: a match that ends on warnings is the one result a
 * head referee will be asked to justify, so the screen has to say who lost it and on what.
 * How many levels the deciding exchange applied is what separates a third ordinary
 * warning from an immediate escalation, and the log carries that.
 */
export function penaltyLoss(r: Ruleset, state: State, names: Names, events: Event[]): Outcome {
  const out = (side: Side) => state[side].penalty >= r.penaltyLoss;
  const last = lastStandingExchange(events);
  const how = (side: Side): string => {
    const levels = last?.exchange?.[side].penalty ?? 1;
    if (levels >= 3) return 'is disqualified';
    if (levels === 2) return 'loses the match on a double warning';
    return 'loses the match on a third warning';
  };

  if (out('red') && out('blue')) {
    return {
      headline: 'Both lose the match on warnings',
      detail: 'Recorded 0–0. Neither earns match points.',
    };
  }
  const loser: Side = out('red') ? 'red' : 'blue';
  const winner: Side = loser === 'red' ? 'blue' : 'red';
  return {
    headline: `${names[loser]} ${how(loser)}`,
    detail: `${names[winner]} wins ${state[winner].score}–${state[loser].score}`,
  };
}

/** The outcome of a match that has ended, for the centre of the score keeper's screen. */
export function ended(r: Ruleset, state: State, names: Names, events: Event[]): Outcome {
  const score = `${state.red.score}–${state.blue.score}`;
  if (state.endReason === 'penalty') return penaltyLoss(r, state, names, events);
  if (state.endReason === 'forfeit') {
    const loser: Side = state.winner === 'red' ? 'blue' : 'red';
    return { headline: `${names[loser]} forfeits`, detail: score };
  }
  if (!state.winner) return { headline: 'Draw', detail: score };
  return { headline: `${names[state.winner]} wins`, detail: score };
}
