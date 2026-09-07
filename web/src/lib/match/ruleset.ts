import type { State } from './types';

/**
 * The hardcoded MSL ruleset (design §5), mirroring internal/match/ruleset.go.
 *
 * A structure rather than constants because Milestone 3 makes rulesets data-driven, and
 * the shared vectors are already the specification such a definition will have to satisfy.
 */
export interface Ruleset {
  maxValue: number;
  pointCap: number;
  matchTimeMs: number;
  finalWarningMs: number;
  penaltyLoss: number;
  penaltyDeduction: number;
  forfeitScore: number;
  winPoints: number;
  drawPoints: number;
  lossPoints: number;
}

export const MSL: Ruleset = {
  maxValue: 2,
  pointCap: 8,
  matchTimeMs: 3 * 60 * 1000,
  finalWarningMs: 2 * 60 * 1000 + 50 * 1000,
  penaltyLoss: 3,
  penaltyDeduction: 2,
  forfeitScore: 8,
  winPoints: 9,
  drawPoints: 6,
  lossPoints: 3,
};

/**
 * Ten seconds remain. A cue for the score keeper only: it has no effect on scoring, and
 * the clock does not stop when it fires.
 */
export function flashing(r: Ruleset, elapsedMs: number, ended: boolean): boolean {
  return !ended && elapsedMs >= r.finalWarningMs;
}

export function emptyState(): State {
  return {
    red: { score: 0, penalty: 0 },
    blue: { score: 0, penalty: 0 },
    exchanges: 0,
    elapsedMs: 0,
    running: false,
    ended: false,
    noMatchPoints: false,
    endReason: '',
    winner: '',
    pending: 'none',
    lastSeq: 0,
    undoableSeq: 0,
  };
}
