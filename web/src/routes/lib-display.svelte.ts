/**
 * What every display needs: the live snapshot, a running clock, a wake lock, and the
 * lookups that turn a mat number into the match on it and the names of the people in it.
 */
import { Live } from '../lib/live.svelte';
import { Clock } from '../lib/clock.svelte';
import type { MatchView, Snapshot } from '../api';

export function matchOn(snapshot: Snapshot | null, mat: number): MatchView | null {
  const id = snapshot?.mats?.[String(mat)] ?? '';
  if (!id) return null;
  return snapshot?.pools.flatMap((p) => p.matches).find((m) => m.id === id) ?? null;
}

export function nextOn(snapshot: Snapshot | null, mat: number): MatchView | null {
  const current = matchOn(snapshot, mat);
  if (!current) return null;
  const onMat = (snapshot?.pools ?? [])
    .filter((p) => p.mat === mat)
    .flatMap((p) => p.matches)
    .filter((m) => m.status !== 'complete');
  const i = onMat.findIndex((m) => m.id === current.id);
  return i >= 0 && i + 1 < onMat.length ? onMat[i + 1] : null;
}

export function nameLookup(snapshot: Snapshot | null): (id: string) => string {
  const byId = new Map((snapshot?.competitors ?? []).map((c) => [c.id, c.name]));
  return (id: string) => byId.get(id) ?? '—';
}

export function namesFor(
  snapshot: Snapshot | null,
  match: MatchView | null,
): { red: string; blue: string } {
  const name = nameLookup(snapshot);
  return { red: match ? name(match.red) : 'Red', blue: match ? name(match.blue) : 'Blue' };
}

export { Live, Clock };
