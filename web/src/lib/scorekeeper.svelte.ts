/**
 * The score keeper's working state: what is selected right now, and what happens when
 * Confirm exchange is pressed.
 *
 * Nothing here takes effect until that press. Points and warnings alike are only
 * selections until then -- the score does not move, the warning is not counted, and
 * nothing is written to the log (design §4).
 */
import { MSL, replay, type Event, type Side, type State } from './match';
import { MatchLog } from './sync.svelte';

export interface Selection {
  /** The selected point value, or 0 for none. The two point buttons are exclusive. */
  value: number;
  /** Penalty levels to apply. 0 or 1 in the MVP; escalation is Milestone 2. */
  penalty: number;
}

export class ScoreKeeperSession {
  log: MatchLog;
  red = $state<Selection>({ value: 0, penalty: 0 });
  blue = $state<Selection>({ value: 0, penalty: 0 });
  /** When the clock was last set running, for the live readout. */
  runningSince = $state<number | null>(null);

  constructor(matchId: string) {
    this.log = new MatchLog(matchId);
  }

  async load(): Promise<void> {
    await this.log.load();
    // A match that was already running when this device joined keeps counting from now;
    // the elapsed time in the log is the floor.
    this.runningSince = this.state.running ? Date.now() : null;
  }

  /** Derived from the log every time it is read, never stored. */
  get state(): State {
    return replay(MSL, this.log.events);
  }

  selection(side: Side): Selection {
    return side === 'red' ? this.red : this.blue;
  }

  /**
   * The point buttons are mutually exclusive, and pressing an already-selected one
   * deselects it -- so any mis-tap is undone by tapping it again.
   */
  togglePoint(side: Side, value: number): void {
    const sel = this.selection(side);
    sel.value = sel.value === value ? 0 : value;
  }

  /** The warning toggles independently of the points. */
  toggleWarning(side: Side): void {
    const sel = this.selection(side);
    sel.penalty = sel.penalty > 0 ? 0 : 1;
  }

  get anythingSelected(): boolean {
    return (
      this.red.value > 0 || this.blue.value > 0 || this.red.penalty > 0 || this.blue.penalty > 0
    );
  }

  private clearSelection(): void {
    this.red = { value: 0, penalty: 0 };
    this.blue = { value: 0, penalty: 0 };
  }

  private event(type: Event['type'], elapsedMs: number, extra: Partial<Event>): Event {
    return {
      seq: this.log.nextSeq(),
      type,
      at: new Date().toISOString(),
      elapsedMs: Math.round(elapsedMs),
      ...extra,
    };
  }

  /**
   * Commits the exchange. Confirming with nothing selected records a no-score exchange,
   * which is a real event and is logged as such.
   */
  async confirm(elapsedMs: number): Promise<void> {
    if (this.state.ended) return;
    const event = this.event('exchange', elapsedMs, {
      exchange: {
        red: { value: this.red.value, penalty: this.red.penalty },
        blue: { value: this.blue.value, penalty: this.blue.penalty },
      },
    });
    this.clearSelection();
    await this.log.write([event]);
  }

  async toggleClock(elapsedMs: number): Promise<void> {
    if (this.state.ended) return;
    const running = this.state.running;
    const action = running ? 'stop' : this.state.elapsedMs > 0 ? 'resume' : 'start';
    await this.log.write([this.event('timer', elapsedMs, { timer: { action } })]);
    this.runningSince = running ? null : Date.now();
  }

  /** Undo of the last confirmed exchange. Appends a correction; never mutates history. */
  async undo(elapsedMs: number): Promise<void> {
    const target = this.state.undoableSeq;
    if (target === 0) return;
    await this.log.write([this.event('undo', elapsedMs, { undo: { seq: target } })]);
  }

  async end(elapsedMs: number, reason: State['endReason']): Promise<void> {
    await this.log.write([this.event('end', elapsedMs, { end: { reason } })]);
    this.runningSince = null;
  }

  /** A match conceded before it starts. Recorded 0-8. */
  async forfeit(side: Side): Promise<void> {
    await this.log.write([this.event('end', 0, { end: { reason: 'forfeit', forfeiter: side } })]);
    this.runningSince = null;
  }
}
