import { describe, expect, it } from 'vitest';
import { formatClock, isFlashing, reanchor } from './clock.svelte';
import { MSL } from './match';

/**
 * The live readout is `base + (now - anchor)`, where the base is the elapsed time carried
 * by the last event in the log. These pin the arithmetic that keeps the two in step.
 */
describe('the clock anchor', () => {
  it('holds the displayed time still when an event moves the base', () => {
    // The clock was started 30 s ago, so the readout says 30 s against a base of 0.
    const anchor = 1_000_000;
    const now = anchor + 30_000;
    const shown = 0 + (now - anchor);

    // Confirming an exchange writes elapsedMs 30000, which becomes the new base.
    const moved = reanchor(anchor, 0, 30_000)!;

    expect(30_000 + (now - moved)).toBe(shown);
  });

  it('does not let a confirmed exchange burst the timer forward', () => {
    // The regression from issue #56: without the correction, the readout doubled on every
    // confirmation, because the interval since the start was counted a second time.
    const anchor = 1_000_000;
    let base = 0;
    let cursor = anchor;
    let live = anchor;

    for (const at of [30_000, 70_000, 95_000]) {
      cursor = anchor + at;
      const shown = base + (cursor - live);
      live = reanchor(live, base, shown)!;
      base = shown;
      expect(base).toBe(at);
    }
  });

  it('follows the base backwards when an undo drops an exchange', () => {
    const anchor = 1_000_000;
    const now = anchor + 5_000;
    const shown = 45_000 + (now - anchor);
    // Undoing the exchange at 45 s leaves the one at 40 s as the last surviving event.
    const moved = reanchor(anchor, 45_000, 40_000)!;
    expect(40_000 + (now - moved)).toBe(shown);
  });

  it('stays stopped while the clock is not running', () => {
    expect(reanchor(null, 0, 30_000)).toBeNull();
  });
});

describe('the readout', () => {
  it('counts up in minutes and seconds, and past the match time', () => {
    expect(formatClock(0)).toBe('00:00');
    expect(formatClock(59_999)).toBe('00:59');
    expect(formatClock(MSL.matchTimeMs)).toBe('03:00');
    // design §4: the clock does not stop at 03:00, so a finished match routinely reads
    // more than three minutes.
    expect(formatClock(191_000)).toBe('03:11');
  });

  it('flashes from the final-exchange threshold until the match ends', () => {
    expect(isFlashing(MSL.finalWarningMs - 1, false)).toBe(false);
    expect(isFlashing(MSL.finalWarningMs, false)).toBe(true);
    expect(isFlashing(MSL.finalWarningMs, true)).toBe(false);
  });
});
