/**
 * The live match clock.
 *
 * The log stores the elapsed time at each event, which is what keeps replay deterministic.
 * This adds the wall-clock time since the last event while the clock is running, so the
 * digits move without the derived state ever depending on when it was replayed.
 */
import { MSL, type State } from './match';

export class Clock {
  now = $state(Date.now());
  private timer: ReturnType<typeof setInterval> | null = null;

  start(): void {
    if (this.timer) return;
    // Ten times a second: enough for the seconds to turn over crisply and for the flash
    // at 02:50 to look deliberate, without being a busy loop.
    this.timer = setInterval(() => {
      this.now = Date.now();
    }, 100);
  }

  stop(): void {
    if (this.timer) clearInterval(this.timer);
    this.timer = null;
  }

  /** Elapsed match time, counting up from 00:00. It does not stop at 03:00. */
  elapsed(state: State, runningSince: number | null): number {
    if (!state.running || runningSince === null) return state.elapsedMs;
    return state.elapsedMs + (this.now - runningSince);
  }
}

export function formatClock(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  const m = Math.floor(total / 60);
  const s = total % 60;
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
}

export function isFlashing(ms: number, ended: boolean): boolean {
  return !ended && ms >= MSL.finalExchangeMs;
}

/**
 * Where the clock's anchor moves when a written event changes the base elapsed time.
 *
 * The live readout is `base + (now - anchor)`, and every event carries its own elapsedMs,
 * so writing one moves the base. Leaving the anchor where it was then counts the whole
 * stretch between anchor and now for a second time -- which is why the match timer jumped
 * forward by the elapsed time on every Confirm exchange.
 *
 * The correction is algebraic rather than a fresh clock reading: holding the displayed
 * time still across the write means `base + (now - anchor)` must come out the same before
 * and after, which gives `anchor += base_after - base_before` exactly. Re-reading the
 * clock instead would shed a fraction of a second on every exchange, and a match is
 * thirty of them.
 *
 * It runs backwards as readily as forwards: an undo drops the voided exchange's elapsedMs
 * out of the replay, so the base falls back to the event before it and the anchor follows.
 */
export function reanchor(anchor: number | null, before: number, after: number): number | null {
  if (anchor === null) return null;
  return anchor + (after - before);
}
