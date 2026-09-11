/**
 * Drift between the two match engines.
 *
 * The engine exists twice, in Go and in TypeScript, and the shared vectors are the only
 * thing holding the two together (docs/tech-stack.md §4). If they ever disagree on a real
 * match, the score keeper is the first to be able to see it: every push comes back with
 * the server's derived state for the same log, and this says whether it matches.
 */
import type { State } from './match';

/** The fields of a State, in the order they are compared and named. */
const FIELDS = [
  'red',
  'blue',
  'exchanges',
  'elapsedMs',
  'running',
  'ended',
  'noMatchPoints',
  'endReason',
  'winner',
  'pending',
  'lastSeq',
  'undoableSeq',
] as const;

/**
 * The names of the fields on which two derived states disagree. Empty when they agree.
 * Only meaningful for two states derived from the same log -- compare `lastSeq` first.
 */
export function differences(local: State, server: State): string[] {
  const out: string[] = [];
  for (const f of FIELDS) {
    const a = local[f];
    const b = server[f];
    const same =
      typeof a === 'object' && a !== null && typeof b === 'object' && b !== null
        ? a.score === b.score && a.penalty === b.penalty
        : a === b;
    if (!same) out.push(f);
  }
  return out;
}

/** A short line for the banner: "here 5-3, server 4-3". */
export function summarise(local: State, server: State): string {
  const score = (s: State) => `${s.red.score}\u2013${s.blue.score}`;
  return `here ${score(local)}, server ${score(server)}`;
}
