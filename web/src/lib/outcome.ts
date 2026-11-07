/**
 * How a match's outcome is described to the score keeper: in the dialog that asks whether
 * to end it, and in the centre of the screen once it has ended.
 *
 * Pure over the engine's state and the log, so the same words appear in both places and
 * can be tested without a screen.
 */
import type { Event, Ruleset, Side, State } from './match';
import { t } from './i18n.svelte';

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
    if (levels >= 3) return t('{name} is disqualified', { name: names[side] });
    if (levels === 2) return t('{name} loses the match on a double warning', { name: names[side] });
    return t('{name} loses the match on a third warning', { name: names[side] });
  };

  if (out('red') && out('blue')) {
    return {
      headline: t('Both lose the match on warnings'),
      detail: t('Recorded 0–0. Neither earns match points.'),
    };
  }
  const loser: Side = out('red') ? 'red' : 'blue';
  const winner: Side = loser === 'red' ? 'blue' : 'red';
  return {
    headline: how(loser),
    detail: t('{name} wins {score}', { name: names[winner], score: `${state[winner].score}–${state[loser].score}` }),
  };
}

/** The outcome of a match that has ended, for the centre of the score keeper's screen. */
export function ended(r: Ruleset, state: State, names: Names, events: Event[]): Outcome {
  const score = `${state.red.score}–${state.blue.score}`;
  if (state.endReason === 'penalty') return penaltyLoss(r, state, names, events);
  if (state.endReason === 'forfeit') {
    const loser: Side = state.winner === 'red' ? 'blue' : 'red';
    return { headline: t('{name} forfeits', { name: names[loser] }), detail: score };
  }
  if (!state.winner) return { headline: t('Draw'), detail: score };
  return { headline: t('{name} wins', { name: names[state.winner] }), detail: score };
}

/**
 * What the dialog that asks whether to end the match says. A penalty loss names the loser
 * and why, because that is the one result a head referee will be asked to justify; the
 * others name the winner. decidedOnSuddenDeath is a level match's first point after time.
 */
export function endDialog(r: Ruleset, state: State, names: Names, events: Event[], decidedOnSuddenDeath: boolean): Outcome {
  if (state.pending === 'final_exchange' && decidedOnSuddenDeath) {
    const leader = state.red.score > state.blue.score ? names.red : names.blue;
    return { headline: t('{name} wins on sudden death', { name: leader }), detail: `${state.red.score}–${state.blue.score}` };
  }
  if (state.pending === 'final_exchange') {
    return { headline: t('Was that the final exchange?'), detail: '' };
  }
  if (state.pending === 'penalty_cap') {
    return penaltyLoss(r, state, names, events);
  }
  if (state.red.score === state.blue.score) {
    return { headline: t('Draw {score}', { score: `${state.red.score}–${state.blue.score}` }), detail: '' };
  }
  const leader = state.red.score > state.blue.score ? names.red : names.blue;
  const high = Math.max(state.red.score, state.blue.score);
  const low = Math.min(state.red.score, state.blue.score);
  return { headline: t('{name} wins {score}', { name: leader, score: `${high}–${low}` }), detail: '' };
}

/** Why a match the dialog ends ended: the cap it reached, or the time. */
export function endReasonOf(state: State): 'penalty' | 'point_cap' | 'time' {
  return state.pending === 'penalty_cap' ? 'penalty' : state.pending === 'point_cap' ? 'point_cap' : 'time';
}

/**
 * How one side finished, for the panel it is shown on (issue #86), or null while the match
 * is running, which is what keeps the scoring buttons on screen.
 *
 * Two warnings out of both of them is the case a plain winner/loser reading gets wrong: the
 * engine records no winner, and "draw" on both halves would be a lie about a match that
 * neither of them won. Both are beaten, so neither half says WINNER, and the centre says
 * why.
 */
export function sideOutcome(r: Ruleset, state: State | null, side: Side): 'won' | 'lost' | 'drew' | null {
  if (!state?.ended) return null;
  if (state.winner) return state.winner === side ? 'won' : 'lost';
  const bothOut = state.endReason === 'penalty' && state.red.penalty >= r.penaltyLoss && state.blue.penalty >= r.penaltyLoss;
  return bothOut ? 'lost' : 'drew';
}
