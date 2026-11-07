import type { Break, ItemTimes, ItemView, MatView } from '../api';

/**
 * The mat board's arithmetic (MatBoard.svelte): the board with the draft's moves made on
 * it, the strip of hours down its side, and where each card stands. Pure, so it is tested
 * on its own; the component keeps the dragging and the talking to the server.
 */

/** A move in the draft: this item, to this mat, at this place, as the board then looked. */
export interface DraftMove {
  id: string;
  mat: number;
  index: number;
}

/**
 * Only a pool is moved by hand (#149): the eliminations follow the pools, and the bronze
 * match and the final follow the eliminations. Nor is one under way or finished.
 */
export function canMove(it: ItemView): boolean {
  return it.movable && it.kind === 'pool';
}

/** The board as the server has it, then with the draft's moves made on it. */
export function drafted(mats: MatView[], items: ItemView[], draft: DraftMove[]): Map<number, ItemView[]> {
  const out = new Map<number, ItemView[]>();
  for (const m of mats) out.set(m.mat, []);
  for (const it of items) out.get(it.mat)?.push(it);
  for (const list of out.values()) list.sort((a, b) => a.position - b.position);
  for (const mv of draft) {
    const target = out.get(mv.mat);
    let it: ItemView | undefined;
    for (const list of out.values()) {
      const i = list.findIndex((o) => o.id === mv.id);
      if (i >= 0) [it] = list.splice(i, 1);
    }
    if (!it) continue;
    if (!canMove(it) || !target) {
      // It started since, or its mat is gone: it stays where the server has it.
      out.get(it.mat)?.splice(it.position - 1, 0, it);
      continue;
    }
    // Never in front of what a mat has under way or finished, and never behind its own
    // discipline's eliminations or finals, which wait for it (#149).
    let floor = 0;
    while (floor < target.length && !target[floor].movable) floor++;
    let ceiling = target.findIndex((o) => o.discipline === it.discipline && o.kind !== 'pool');
    if (ceiling < 0) ceiling = target.length;
    target.splice(Math.min(Math.max(ceiling, floor), Math.max(floor, mv.index)), 0, it);
  }
  return out;
}

/** Where a card is on a board: its mat and its place among the others. */
export function placeOf(board: Map<number, ItemView[]>, id: string): { mat: number; index: number } | null {
  for (const [mat, list] of board) {
    const i = list.findIndex((o) => o.id === id);
    if (i >= 0) return { mat, index: i };
  }
  return null;
}

/** How many eliminations items a discipline has, for naming them. */
export function lanesOf(items: ItemView[], discipline: string): number {
  return items.filter((o) => o.discipline === discipline && o.kind === 'eliminations').length;
}

// How tall a minute is: enough for the shortest pool's card to hold its text, within
// reason. A day of long pools stays on a screen or two; a day of short ones stays legible.
const SHORTEST_POOL_PX = 92;
const MIN_SCALE = 1.5;
const MAX_SCALE = 3.5;
export const CARD_GAP = 4;

/** The strip of hours: from the whole hour before the first item, a mark every half hour. */
export interface Axis {
  /** When the strip starts, in milliseconds, and the hour that is. */
  start: number;
  hour: number;
  /** Pixels a minute. */
  scale: number;
  minutes: number;
  ticks: { y: number; label: string; hour: boolean }[];
}

/** The day's hours from the whole hour before the first item, with a mark every half hour. */
export function dayAxis(times: ItemTimes[], kindOf: (id: string) => string | undefined): Axis | null {
  let from = Infinity;
  let to = -Infinity;
  let first = '';
  let shortest = Infinity;
  for (const x of times) {
    const s = Date.parse(x.start);
    const e = Date.parse(x.end);
    if (Number.isNaN(s) || Number.isNaN(e)) continue;
    if (s < from) {
      from = s;
      first = x.start;
    }
    to = Math.max(to, e);
    if (kindOf(x.id) === 'pool' && e > s) shortest = Math.min(shortest, (e - s) / 60000);
  }
  if (!Number.isFinite(from) || to <= from) return null;
  const hour = Number(first.slice(11, 13));
  const start = from - (Number(first.slice(14, 16)) * 60 + Number(first.slice(17, 19) || 0)) * 1000;
  const scale = Math.min(MAX_SCALE, Math.max(MIN_SCALE, SHORTEST_POOL_PX / shortest));
  const minutes = Math.ceil((to - start) / 1800000) * 30;
  const ticks: { y: number; label: string; hour: boolean }[] = [];
  for (let m = 0; m <= minutes; m += 30) {
    const whole = m % 60 === 0;
    ticks.push({ y: m * scale, label: whole ? String((hour + m / 60) % 24).padStart(2, '0') : '–', hour: whole });
  }
  return { start, hour, scale, minutes, ticks };
}

export interface Layout {
  cards: Map<string, { top: number; height: number }>;
  /** Where each mat's last card ends. */
  ends: Map<number, number>;
  height: number;
}

/**
 * Where every card goes on its mat, and how tall the board has to be. A card stands at its
 * forecast time, or pushed down by one too short for its text (drawn says how tall each card
 * was drawn); a mat with moves in the draft runs its movable cards one after another.
 */
export function cardLayout(
  board: Map<number, ItemView[]>,
  axis: Axis | null,
  times: Map<string, ItemTimes>,
  drawn: Record<string, number>,
  draft: DraftMove[],
  itemById: Map<string, ItemView>,
): Layout {
  const cards = new Map<string, { top: number; height: number }>();
  const ends = new Map<number, number>();
  let height = 0;
  if (!axis) return { cards, ends, height };
  height = axis.minutes * axis.scale;
  const inDraft = new Set<number>();
  for (const mv of draft) {
    inDraft.add(mv.mat);
    const it = itemById.get(mv.id);
    if (it) inDraft.add(it.mat);
  }
  const y = (ms: number) => ((ms - axis.start) / 60000) * axis.scale;
  const span = (it: ItemView) => {
    const x = times.get(it.id);
    return { s: Date.parse(x?.start ?? ''), e: Date.parse(x?.end ?? '') };
  };
  for (const [mat, list] of board) {
    // A mat with moves in the draft runs its movable cards one after another.
    let cursor = Infinity;
    if (inDraft.has(mat)) {
      for (const it of list) {
        const { s } = span(it);
        if (it.movable && !Number.isNaN(s)) cursor = Math.min(cursor, s);
      }
    }
    let edge = 0;
    for (const it of list) {
      const { s, e } = span(it);
      const timed = !Number.isNaN(s) && !Number.isNaN(e);
      let at = timed ? s : axis.start;
      if (timed && inDraft.has(mat) && it.movable) {
        at = cursor;
        cursor = at + (e - s);
      } else if (timed) {
        cursor = Math.max(cursor === Infinity ? 0 : cursor, e);
      }
      const top = Math.max(y(at), edge);
      const tall = timed ? ((e - s) / 60000) * axis.scale : 0;
      cards.set(it.id, { top, height: tall });
      edge = top + Math.max(tall, drawn[it.id] ?? 0) + CARD_GAP;
    }
    ends.set(mat, edge);
    height = Math.max(height, edge);
  }
  return { cards, ends, height };
}

/** The hall's breaks, as bands across every mat. lunch is what an unnamed break is called. */
export function breakBands(breaks: Break[], axis: Axis | null, lunch: string): { label: string; top: number; height: number }[] {
  if (!axis) return [];
  const at = (clock: string) => {
    const [h, m] = clock.split(':').map(Number);
    return (h * 60 + m - axis.hour * 60) * axis.scale;
  };
  return breaks.map((b) => ({ label: b.label === 'Lunch' ? lunch : (b.label ?? ''), top: at(b.from), height: at(b.to) - at(b.from) }));
}
