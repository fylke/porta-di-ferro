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
  return !ended && ms >= MSL.finalWarningMs;
}
