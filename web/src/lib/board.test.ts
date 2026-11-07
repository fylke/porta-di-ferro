import { describe, expect, it } from 'vitest';
import type { ItemTimes, ItemView, MatView } from '../api';
import { breakBands, canMove, cardLayout, dayAxis, drafted, lanesOf, placeOf } from './board';

function item(id: string, mat: number, position: number, over: Partial<ItemView> = {}): ItemView {
  return {
    id,
    discipline: id.split('/')[0],
    disciplineName: 'Sabre',
    kind: 'pool',
    mat,
    position,
    status: 'ready',
    done: 0,
    total: 6,
    movable: true,
    ...over,
  };
}

const mats = [{ mat: 1 }, { mat: 2 }] as MatView[];

function ids(board: Map<number, ItemView[]>, mat: number): string[] {
  return (board.get(mat) ?? []).map((it) => it.id);
}

describe('drafted', () => {
  const items = [
    item('sa/pool-1', 1, 1, { status: 'running', movable: false }),
    item('sa/pool-2', 1, 2),
    item('sa/elim-1', 1, 3, { kind: 'eliminations' }),
    item('ls/pool-1', 2, 1),
  ];

  it('is the server’s board with no moves', () => {
    const board = drafted(mats, items, []);
    expect(ids(board, 1)).toEqual(['sa/pool-1', 'sa/pool-2', 'sa/elim-1']);
    expect(ids(board, 2)).toEqual(['ls/pool-1']);
  });

  it('never puts a card in front of what a mat is running', () => {
    const board = drafted(mats, items, [{ id: 'ls/pool-1', mat: 1, index: 0 }]);
    expect(ids(board, 1)).toEqual(['sa/pool-1', 'ls/pool-1', 'sa/pool-2', 'sa/elim-1']);
  });

  it('never puts a pool behind its own discipline’s eliminations', () => {
    const board = drafted(mats, items, [{ id: 'sa/pool-2', mat: 1, index: 9 }]);
    expect(ids(board, 1)).toEqual(['sa/pool-1', 'sa/pool-2', 'sa/elim-1']);
  });

  it('leaves a card that started since where the server has it', () => {
    const started = items.map((it) => (it.id === 'ls/pool-1' ? { ...it, movable: false } : it));
    const board = drafted(mats, started, [{ id: 'ls/pool-1', mat: 1, index: 1 }]);
    expect(ids(board, 2)).toEqual(['ls/pool-1']);
    expect(placeOf(board, 'ls/pool-1')).toEqual({ mat: 2, index: 0 });
  });

  it('moves only pools by hand', () => {
    expect(canMove(items[1])).toBe(true);
    expect(canMove(items[2])).toBe(false);
    expect(canMove(items[0])).toBe(false);
  });

  it('counts a discipline’s eliminations for naming them', () => {
    expect(lanesOf(items, 'sa')).toBe(1);
    expect(lanesOf(items, 'ls')).toBe(0);
  });
});

function times(id: string, start: string, end: string): ItemTimes {
  return { id, mat: 1, start, end, plannedStart: start, plannedEnd: end };
}

describe('the day down the board', () => {
  const day = [times('sa/pool-1', '2026-11-14T09:20:00Z', '2026-11-14T10:00:00Z'), times('sa/pool-2', '2026-11-14T10:05:00Z', '2026-11-14T11:05:00Z')];

  it('starts on the whole hour before the first item and marks every half hour', () => {
    const axis = dayAxis(day, () => 'pool')!;
    expect(axis.hour).toBe(9);
    expect(axis.start).toBe(Date.parse('2026-11-14T09:00:00Z'));
    expect(axis.minutes).toBe(150);
    expect(axis.ticks.map((t) => t.label)).toEqual(['09', '–', '10', '–', '11', '–']);
  });

  it('is nothing until something is timed', () => {
    expect(dayAxis([], () => 'pool')).toBeNull();
  });

  it('stands a card at its time, or below one too tall for its time', () => {
    const axis = dayAxis(day, () => 'pool')!;
    const board = new Map([[1, [item('sa/pool-1', 1, 1), item('sa/pool-2', 1, 2)]]]);
    const byId = new Map(board.get(1)!.map((it) => [it.id, it]));
    const at = new Map(day.map((x) => [x.id, x]));
    const exact = cardLayout(board, axis, at, {}, [], byId);
    expect(exact.cards.get('sa/pool-1')!.top).toBeCloseTo(20 * axis.scale);
    expect(exact.cards.get('sa/pool-2')!.top).toBeCloseTo(65 * axis.scale);
    const pushed = cardLayout(board, axis, at, { 'sa/pool-1': 1000 }, [], byId);
    expect(pushed.cards.get('sa/pool-2')!.top).toBeGreaterThan(20 * axis.scale + 1000);
  });

  it('draws the hall’s breaks across the mats, an unnamed one as lunch', () => {
    const axis = dayAxis(day, () => 'pool')!;
    const [band] = breakBands([{ label: 'Lunch', from: '10:00', to: '10:30' }], axis, 'Lunch!');
    expect(band.label).toBe('Lunch!');
    expect(band.top).toBeCloseTo(60 * axis.scale);
    expect(band.height).toBeCloseTo(30 * axis.scale);
  });
});
