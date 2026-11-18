import { describe, expect, it } from 'vitest';
import { runnable } from './mats.svelte';
import type { ItemView, MatView, Slot } from '../api';

function slot(discipline: string, item: string, id: string): Slot {
  return {
    discipline,
    disciplineName: discipline,
    item,
    red: 'A',
    blue: 'B',
    match: { id, red: 'a', blue: 'b', status: 'pending' } as Slot['match'],
  };
}

function item(id: string, status: ItemView['status']): ItemView {
  return { id, status } as ItemView;
}

describe('runnable', () => {
  const mat: MatView = {
    mat: 1,
    queue: [slot('ls', 'ls/pool-1', 'p1m1'), slot('ls', 'ls/pool-1', 'p1m2'), slot('sa', 'sa/pool-1', 'p1m1'), slot('ls', 'ls/elim-1', 'e-qf1')],
  } as MatView;

  it('stops in front of the item the mat is held at', () => {
    const got = runnable(mat, [item('ls/pool-1', 'done'), item('sa/pool-1', 'held'), item('ls/elim-1', 'ready')]);
    expect(got.map((s) => `${s.discipline}/${s.match.id}`)).toEqual(['ls/p1m1', 'ls/p1m2']);
  });

  it('stops in front of an item waiting for its block of the day', () => {
    const got = runnable(mat, [item('ls/pool-1', 'running'), item('sa/pool-1', 'queued')]);
    expect(got).toHaveLength(2);
  });

  it('runs the whole queue once nothing is held', () => {
    expect(runnable(mat, [item('sa/pool-1', 'ready')])).toHaveLength(4);
    expect(runnable(null, [])).toEqual([]);
  });
});
