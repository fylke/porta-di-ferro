/**
 * Times of day from the forecast (phase 4). The server writes them in the hall's zone, and
 * a page prints the clock as written there: a phone that thinks it is somewhere else must
 * still say 10:40 when the hall says 10:40.
 */

/** "10:40" from "2026-11-14T10:40:00+01:00", or "" for nothing. */
export function clockOf(stamp: string | undefined | null): string {
  if (!stamp || stamp.length < 16) return '';
  return stamp.slice(11, 16);
}

/** Minutes from a to b, both stamps from the same server, rounded; 0 if either is missing. */
export function minutesBetween(a: string | undefined, b: string | undefined): number {
  if (!a || !b) return 0;
  return Math.round((Date.parse(b) - Date.parse(a)) / 60000);
}

/** "4:30" from 270 seconds. */
export function mmss(seconds: number): string {
  const s = Math.max(0, Math.round(seconds));
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}
