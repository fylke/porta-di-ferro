import { describe, expect, it } from 'vitest';
import { MSL, replay, type Event } from './match';
import { endDialog, endReasonOf, ended, lastStandingExchange, penaltyLoss, sideOutcome } from './outcome';

const names = { red: 'Ada', blue: 'Bo' };

function exchange(seq: number, red: [number, number], blue: [number, number]): Event {
  return {
    seq,
    type: 'exchange',
    elapsedMs: seq * 1000,
    exchange: { red: { value: red[0], penalty: red[1] }, blue: { value: blue[0], penalty: blue[1] } },
  };
}

describe('a penalty loss is described by how it was reached', () => {
  it('names a third ordinary warning', () => {
    const log = [exchange(1, [0, 1], [0, 0]), exchange(2, [0, 1], [0, 0]), exchange(3, [0, 1], [0, 0])];
    const out = penaltyLoss(MSL, replay(MSL, log), names, log);
    expect(out.headline).toBe('Ada loses the match on a third warning');
    expect(out.detail).toBe('Bo wins 8–0');
  });

  it('names an immediate disqualification', () => {
    const log = [exchange(1, [0, 0], [0, 3])];
    const out = penaltyLoss(MSL, replay(MSL, log), names, log);
    expect(out.headline).toBe('Bo is disqualified');
    expect(out.detail).toBe('Ada wins 8–0');
  });

  it('names a double warning that reached the losing level', () => {
    const log = [exchange(1, [0, 1], [0, 0]), exchange(2, [0, 2], [0, 0])];
    const out = penaltyLoss(MSL, replay(MSL, log), names, log);
    expect(out.headline).toBe('Ada loses the match on a double warning');
  });

  it('looks past an undone exchange for the one that decided it', () => {
    const log: Event[] = [
      exchange(1, [0, 0], [0, 3]),
      { seq: 2, type: 'undo', elapsedMs: 2000, undo: { seq: 1 } },
      exchange(3, [0, 1], [0, 0]),
      exchange(4, [0, 1], [0, 0]),
      exchange(5, [0, 1], [0, 0]),
    ];
    expect(lastStandingExchange(log)?.seq).toBe(5);
    expect(penaltyLoss(MSL, replay(MSL, log), names, log).headline).toBe(
      'Ada loses the match on a third warning',
    );
  });

  it('says so when both reach the losing level at once', () => {
    const log = [exchange(1, [0, 3], [0, 3])];
    expect(penaltyLoss(MSL, replay(MSL, log), names, log).headline).toBe(
      'Both lose the match on warnings',
    );
  });
});

describe('the outcome of an ended match', () => {
  it('names the winner for a win on points and the score for a draw', () => {
    const win: Event[] = [
      exchange(1, [2, 0], [0, 0]),
      { seq: 2, type: 'end', elapsedMs: 5000, end: { reason: 'time' } },
    ];
    expect(ended(MSL, replay(MSL, win), names, win)).toEqual({ headline: 'Ada wins', detail: '2–0' });

    const draw: Event[] = [{ seq: 1, type: 'end', elapsedMs: 5000, end: { reason: 'time' } }];
    expect(ended(MSL, replay(MSL, draw), names, draw)).toEqual({ headline: 'Draw', detail: '0–0' });
  });

  it('names the forfeiter, not the winner', () => {
    const log: Event[] = [
      { seq: 1, type: 'end', elapsedMs: 0, end: { reason: 'forfeit', forfeiter: 'blue' } },
    ];
    expect(ended(MSL, replay(MSL, log), names, log)).toEqual({ headline: 'Bo forfeits', detail: '8–0' });
  });

  it('keeps the penalty wording after the match has ended', () => {
    const log: Event[] = [
      exchange(1, [0, 0], [0, 3]),
      { seq: 2, type: 'end', elapsedMs: 2000, end: { reason: 'penalty' } },
    ];
    expect(ended(MSL, replay(MSL, log), names, log).headline).toBe('Bo is disqualified');
  });
});

describe('the dialog that asks whether to end the match', () => {
  it('names the winner and the score at a point cap', () => {
    const state = { ...replay(MSL, [exchange(1, [2, 0], [0, 0])]), pending: 'point_cap' as const };
    state.red = { ...state.red, score: 8 };
    expect(endDialog(MSL, state, names, [], false)).toEqual({ headline: 'Ada wins 8–0', detail: '' });
    expect(endReasonOf(state)).toBe('point_cap');
  });

  it('asks about the final exchange, unless sudden death decided it', () => {
    const state = { ...replay(MSL, [exchange(1, [1, 0], [0, 0])]), pending: 'final_exchange' as const };
    expect(endDialog(MSL, state, names, [], false).headline).toBe('Was that the final exchange?');
    expect(endDialog(MSL, state, names, [], true)).toEqual({ headline: 'Ada wins on sudden death', detail: '1–0' });
    expect(endReasonOf(state)).toBe('time');
  });

  it('says a level score is a draw', () => {
    const state = { ...replay(MSL, [exchange(1, [1, 0], [1, 0])]), pending: 'point_cap' as const };
    expect(endDialog(MSL, state, names, [], false).headline).toMatch(/^Draw /);
  });
});

describe('how each side finished', () => {
  it('is nothing while the match runs', () => {
    expect(sideOutcome(MSL, replay(MSL, [exchange(1, [1, 0], [0, 0])]), 'red')).toBeNull();
    expect(sideOutcome(MSL, null, 'red')).toBeNull();
  });

  it('beats both when both are out on warnings', () => {
    const log = [exchange(1, [0, 3], [0, 3])];
    const state = { ...replay(MSL, log), ended: true, endReason: 'penalty' as const, winner: '' as const };
    expect(sideOutcome(MSL, state, 'red')).toBe('lost');
    expect(sideOutcome(MSL, state, 'blue')).toBe('lost');
  });
});
