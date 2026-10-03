import { describe, expect, it } from 'vitest';
import { clockOf, minutesBetween, mmss } from './clock-of-day';

describe('times of day from the forecast', () => {
  it('prints the clock the hall wrote, whatever zone the phone is in', () => {
    expect(clockOf('2026-11-14T10:40:00+01:00')).toBe('10:40');
    expect(clockOf('2026-11-14T23:05:00-08:00')).toBe('23:05');
    expect(clockOf('')).toBe('');
    expect(clockOf(undefined)).toBe('');
  });
  it('measures drift in minutes', () => {
    expect(minutesBetween('2026-11-14T10:15:00+01:00', '2026-11-14T10:40:00+01:00')).toBe(25);
    expect(minutesBetween('2026-11-14T10:40:00+01:00', '2026-11-14T10:30:00+01:00')).toBe(-10);
    expect(minutesBetween(undefined, '2026-11-14T10:30:00+01:00')).toBe(0);
  });
  it('writes durations as minutes and seconds', () => {
    expect(mmss(270)).toBe('4:30');
    expect(mmss(60)).toBe('1:00');
  });
});
