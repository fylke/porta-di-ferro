/**
 * Sudden death in the eliminations (design §7 item 3; MSL's SM rules): a bracket match
 * cannot be drawn. When the final exchange leaves the scores level the match goes on,
 * with no dialog and the clock running, and the first point wins.
 *
 * The engine still raises the final-exchange question on every confirmation past the
 * threshold. These two say when the client should decline to ask it, and when it should
 * ask a different one -- so the rule lives in one testable place rather than in the
 * component's conditionals.
 */
import type { State } from './match';

/** The match is in sudden death: past the threshold, scores level, not over. */
export function inSuddenDeath(state: State | null, knockout: boolean): boolean {
  return (
    knockout &&
    !!state &&
    !state.ended &&
    state.pending === 'final_exchange' &&
    state.red.score === state.blue.score
  );
}

/**
 * Sudden death has just been decided: the scores were level at the threshold and the
 * latest exchange broke the tie. The match ends on it; the only alternative is that the
 * entry was a mistake, so the second action is undo, not continue.
 */
export function suddenDeathDecided(state: State | null, knockout: boolean, wasLevel: boolean): boolean {
  return (
    knockout &&
    wasLevel &&
    !!state &&
    !state.ended &&
    state.pending === 'final_exchange' &&
    state.red.score !== state.blue.score
  );
}
