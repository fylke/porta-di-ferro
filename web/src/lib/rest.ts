/**
 * A rest between back-to-back matches.
 *
 * When a fencer comes straight off one match on a mat into the next, the score keeper is
 * offered a rest timer before starting it: two minutes by default, longer or shorter in
 * steps of thirty seconds as the head referee decides, and cancelled whenever they say so.
 * It is offered, never imposed -- pressing Play without it is declining it.
 *
 * The timer lives on the score keeper's device only. It is not part of the match: nothing
 * about a rest reaches the log, the result or anybody's ranking.
 */

export const REST_DEFAULT_MS = 120_000;
export const REST_STEP_MS = 30_000;

interface Bout {
  id: string;
  red: string;
  blue: string;
}

/**
 * Who in `next` fenced in `previous` -- the match immediately before it on the same mat,
 * once that one is finished. Empty when nobody did, which is when there is nothing to
 * offer.
 */
export function backToBack(previous: Bout | null, next: Bout | null): string[] {
  if (!previous || !next) return [];
  const before = new Set([previous.red, previous.blue].filter(Boolean));
  return [next.red, next.blue].filter((id) => id && before.has(id));
}

/**
 * A rest length after one press of plus or minus. Before it starts it cannot go below one
 * step -- a rest of nothing is Play -- and a running one is adjusted through `remaining`.
 */
export function adjustLength(ms: number, delta: number): number {
  return Math.max(REST_STEP_MS, ms + delta);
}

/** Milliseconds left of a rest ending at `until`, never below zero. */
export function remaining(until: number, now: number): number {
  return Math.max(0, until - now);
}

/**
 * The end of a running rest after one press. Taking time off can end it on the spot; it
 * cannot put the end in the past.
 */
export function adjustUntil(until: number, delta: number, now: number): number {
  return Math.max(now, until + delta);
}

/** Minutes and seconds, rounded up so the last second shows as 0:01 rather than 0:00. */
export function formatRest(ms: number): string {
  const s = Math.ceil(ms / 1000);
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}
