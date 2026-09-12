import { describe, expect, it } from 'vitest';
import { emptyState, type State } from './match';
import { inSuddenDeath, suddenDeathDecided } from './knockout';

function past(red: number, blue: number, extra: Partial<State> = {}): State {
  return {
    ...emptyState(),
    red: { score: red, penalty: 0 },
    blue: { score: blue, penalty: 0 },
    elapsedMs: 175_000,
    pending: 'final_exchange',
    ...extra,
  };
}

describe('sudden death', () => {
  it('begins when a knockout match reaches the threshold level', () => {
    expect(inSuddenDeath(past(4, 4), true)).toBe(true);
  });

  it('is never a pool matter: pools may draw', () => {
    expect(inSuddenDeath(past(4, 4), false)).toBe(false);
    expect(suddenDeathDecided(past(5, 4), false, true)).toBe(false);
  });

  it('is not entered when the scores differ at the threshold', () => {
    // An ordinary final exchange: the referee may still continue.
    expect(inSuddenDeath(past(5, 4), true)).toBe(false);
    expect(suddenDeathDecided(past(5, 4), true, false)).toBe(false);
  });

  it('is decided by the first exchange that breaks the tie', () => {
    expect(suddenDeathDecided(past(5, 4), true, true)).toBe(true);
    // A double that nets zero keeps it going.
    expect(inSuddenDeath(past(5, 5), true)).toBe(true);
    expect(suddenDeathDecided(past(5, 5), true, true)).toBe(false);
  });

  it('is over once the match has ended', () => {
    expect(inSuddenDeath(past(4, 4, { ended: true, pending: 'none' }), true)).toBe(false);
    expect(suddenDeathDecided(past(5, 4, { ended: true, pending: 'none' }), true, true)).toBe(false);
  });
});
