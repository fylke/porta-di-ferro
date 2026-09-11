/**
 * The score keeper's working state: what is selected right now, and what happens when
 * Confirm exchange is pressed.
 *
 * Nothing here takes effect until that press. Points and warnings alike are only
 * selections until then -- the score does not move, the warning is not counted, and
 * nothing is written to the log (design §4).
 */
import { MSL, replay, type Event, type Side, type State } from './match';
import { reanchor } from './clock.svelte';
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
    // A match that was already running when this device joined carries on from where the
    // log left it, not from the moment the page opened. Anchoring to now instead rewound
    // the clock by however long the device had been away, so a score keeper who refreshed
    // mid-match no longer agreed with the mat display -- the other half of issue #58.
    this.runningSince = this.state.running ? Date.now() - this.sinceLastEvent() : null;
  }

  /**
   * How long ago the last event in the log happened, by this device's clock.
   *
   * One device keeps score for a match, so those timestamps were written by the very
   * clock now reading them and the answer is exact -- including after a stretch offline,
   * when the server's own idea of when it last saw an event is the one that is wrong.
   * A timestamp that will not parse falls back to zero, which is the behaviour this
   * replaces and so never worse than it.
   */
  private sinceLastEvent(): number {
    const last = this.log.events[this.log.events.length - 1];
    const at = last?.at ? Date.parse(last.at) : NaN;
    if (Number.isNaN(at)) return 0;
    return Math.max(0, Date.now() - at);
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

  /**
   * Appends an event and moves the clock's anchor with it, so the readout carries on from
   * the number that was on screen rather than jumping.
   *
   * Every write goes through here. An event carries the elapsed time at the moment it
   * happened, and replaying it moves the base the live clock counts up from -- so the
   * anchor has to move by the same amount or the interval between them is counted twice.
   */
  private async commit(event: Event): Promise<void> {
    const before = this.state.elapsedMs;
    await this.log.write([event]);
    this.runningSince = reanchor(this.runningSince, before, this.state.elapsedMs);
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
    await this.commit(event);
  }

  async toggleClock(elapsedMs: number): Promise<void> {
    if (this.state.ended) return;
    const running = this.state.running;
    const action = running ? 'stop' : this.state.elapsedMs > 0 ? 'resume' : 'start';
    await this.commit(this.event('timer', elapsedMs, { timer: { action } }));
    // The clock control is the one place the anchor is set outright rather than moved:
    // starting or resuming is exactly the moment the base becomes now.
    this.runningSince = running ? null : Date.now();
  }

  /**
   * Puts the clock back to 00:00, stopped. For a clock started by mistake -- and a
   * correction appended like any other, so the log still shows that it happened.
   */
  async resetClock(elapsedMs: number): Promise<void> {
    if (this.state.ended) return;
    await this.commit(this.event('timer', elapsedMs, { timer: { action: 'reset' } }));
    this.runningSince = null;
  }

  /** Undo of the last confirmed exchange. Appends a correction; never mutates history. */
  async undo(elapsedMs: number): Promise<void> {
    const target = this.state.undoableSeq;
    if (target === 0) return;
    await this.commit(this.event('undo', elapsedMs, { undo: { seq: target } }));
  }

  async end(elapsedMs: number, reason: State['endReason']): Promise<void> {
    await this.commit(this.event('end', elapsedMs, { end: { reason } }));
    this.runningSince = null;
  }

  /** A match conceded before it starts. Recorded 0-8. */
  async forfeit(side: Side): Promise<void> {
    await this.commit(this.event('end', 0, { end: { reason: 'forfeit', forfeiter: side } }));
    this.runningSince = null;
  }
}
