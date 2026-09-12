import { describe, expect, it } from 'vitest';
import { differences, summarise } from './drift';
import { emptyState, type State } from './match';

function state(patch: Partial<State> = {}): State {
  return { ...emptyState(), ...patch };
}

describe('drift between the two engines', () => {
  it('finds nothing when the server agrees', () => {
    const a = state({ red: { score: 3, penalty: 1 }, lastSeq: 4, undoableSeq: 4 });
    expect(differences(a, { ...a, red: { ...a.red } })).toEqual([]);
  });

  it('names every field the two disagree on', () => {
    const local = state({ red: { score: 5, penalty: 0 }, pending: 'none', lastSeq: 6 });
    const server = state({ red: { score: 4, penalty: 0 }, pending: 'final_exchange', lastSeq: 6 });
    expect(differences(local, server)).toEqual(['red', 'pending']);
  });

  it('treats a penalty level as part of the competitor', () => {
    const local = state({ blue: { score: 2, penalty: 1 } });
    const server = state({ blue: { score: 2, penalty: 2 } });
    expect(differences(local, server)).toEqual(['blue']);
  });

  it('summarises the scores for the banner', () => {
    const local = state({ red: { score: 5, penalty: 0 }, blue: { score: 3, penalty: 0 } });
    const server = state({ red: { score: 4, penalty: 0 }, blue: { score: 3, penalty: 0 } });
    expect(summarise(local, server)).toBe('here 5\u20133, server 4\u20133');
  });
});
