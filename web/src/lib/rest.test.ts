import { describe, expect, it } from 'vitest';
import {
  REST_DEFAULT_MS,
  REST_STEP_MS,
  adjustLength,
  adjustUntil,
  backToBack,
  formatRest,
  remaining,
} from './rest';

describe('the rest between back-to-back matches', () => {
  it('is offered for whoever fenced the match before', () => {
    const before = { id: 'm1', red: 'c1', blue: 'c2' };
    expect(backToBack(before, { id: 'm2', red: 'c3', blue: 'c1' })).toEqual(['c1']);
    expect(backToBack(before, { id: 'm2', red: 'c2', blue: 'c1' })).toEqual(['c2', 'c1']);
    expect(backToBack(before, { id: 'm2', red: 'c3', blue: 'c4' })).toEqual([]);
  });

  it('is not offered with nothing before it, or with a bracket slot not yet filled', () => {
    expect(backToBack(null, { id: 'm2', red: 'c1', blue: 'c2' })).toEqual([]);
    expect(backToBack({ id: 'm1', red: 'c1', blue: '' }, { id: 'm2', red: '', blue: 'c3' })).toEqual([]);
  });

  it('starts at two minutes and moves in thirty seconds', () => {
    expect(REST_DEFAULT_MS).toBe(120_000);
    expect(adjustLength(REST_DEFAULT_MS, REST_STEP_MS)).toBe(150_000);
    expect(adjustLength(REST_DEFAULT_MS, -REST_STEP_MS)).toBe(90_000);
  });

  it('cannot be set to nothing before it starts', () => {
    expect(adjustLength(REST_STEP_MS, -REST_STEP_MS)).toBe(REST_STEP_MS);
  });

  it('can be shortened to its end while running, but not past it', () => {
    const now = 1_000_000;
    expect(adjustUntil(now + 45_000, -REST_STEP_MS, now)).toBe(now + 15_000);
    expect(adjustUntil(now + 15_000, -REST_STEP_MS, now)).toBe(now);
    expect(adjustUntil(now + 15_000, REST_STEP_MS, now)).toBe(now + 45_000);
    expect(remaining(now - 5, now)).toBe(0);
  });

  it('shows the last second as a second', () => {
    expect(formatRest(120_000)).toBe('2:00');
    expect(formatRest(89_001)).toBe('1:30');
    expect(formatRest(400)).toBe('0:01');
    expect(formatRest(0)).toBe('0:00');
  });
});
