/**
 * The score keeper's working state: what is selected right now, and what happens when
 * Confirm exchange is pressed.
 *
 * Nothing here takes effect until that press. Points and warnings alike are only
 * selections until then -- the score does not move, the warning is not counted, and
 * nothing is written to the log (design §4).
 */
import { MSL, optionsOf, replay, type Event, type Options, type Side, type State } from './match';
import { reanchor } from './clock.svelte';
import { MatchLog } from './sync.svelte';

export interface Selection {
  /** The selected point value, or 0 for none. The two point buttons are exclusive. */
  value: number;
  /**
   * Penalty levels to apply: 0 none, 1 a warning, 2 straight to a point deduction, 3
   * straight to a match loss. The warning button toggles between 0 and 1; the severe
   * levels are chosen from the overflow menu and show on the same button.
   */
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

  /**
   * Derived from the log every time it is read, never stored -- unless the score keeper
   * has chosen the server's state after a disagreement, in which case the server's answer
   * for the same log is shown instead. Once this device writes past what the server has
   * answered for, its own replay is the only state there is until the next push.
   */
  get state(): State {
    const local = replay(MSL, this.log.events);
    const server = this.log.serverState;
    if (this.log.aligned && server && server.lastSeq === local.lastSeq) return server;
    return local;
  }

  /** How the match is shown -- colours and display sides -- also from the log. */
  get options(): Options {
    return optionsOf(this.log.events);
  }

  /**
   * Changes how the match is shown. An options record rides the log like everything else,
   * which is what gets it to the displays through the same path as the score and lets it
   * work with no server to talk to.
   */
  async setOptions(patch: Partial<Options>, elapsedMs: number): Promise<void> {
    const next = { ...this.options, ...patch };
    await this.commit(this.event('options', elapsedMs, { options: next }));
  }

  selection(side: Side): Selection {
    return side === 'red' ? this.red : this.blue;
  }

  /**
   * The point buttons are mutually exclusive, and pressing an already-selected one
   * deselects it -- so any mis-tap is undone by tapping it again.
   *
   * Selecting a point also starts the clock if it is not running. A point being awarded
   * means fencing has been happening, and a score keeper who forgot to press play is the
   * most common way a match clock ends up wrong at a competition. Deselecting does not
   * stop it again: the clock is now right, and a mis-tap on the point is not a time-out.
   */
  togglePoint(side: Side, value: number): void {
    const sel = this.selection(side);
    const selecting = sel.value !== value;
    sel.value = selecting ? value : 0;
    if (selecting && !this.state.running && !this.state.ended) void this.startClock();
  }

  /**
   * The warning toggles independently of the points, and it is also how a severe warning
   * is cancelled: tapping a selected DOUBLE!! or TRIPLE!!! clears it back to nothing, and
   * the next tap is an ordinary warning again. Cancelling a mis-picked escalation never
   * means going back into the menu (design §4).
   */
  toggleWarning(side: Side): void {
    const sel = this.selection(side);
    sel.penalty = sel.penalty > 0 ? 0 : 1;
  }

  /**
   * Immediate escalation: the head referee has judged a violation severe enough to skip
   * the ladder. It is a pending selection like any other and commits with Confirm
   * exchange -- reaching into a buried menu is already deliberate, and the normal confirm
   * is the second gate. What the engine does with it is the same as with an ordinary
   * warning, applied two or three levels at once.
   */
  escalate(side: Side, levels: 2 | 3): void {
    this.selection(side).penalty = levels;
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
    if (!this.state.running) {
      await this.startClock();
      return;
    }
    await this.commit(this.event('timer', elapsedMs, { timer: { action: 'stop' } }));
    this.runningSince = null;
  }

  /**
   * Starts a stopped clock: a first start from zero, or a resume after a time-out. The
   * one place the anchor is set outright rather than moved, because starting is exactly
   * the moment the base becomes now.
   */
  private async startClock(): Promise<void> {
    const base = this.state.elapsedMs;
    const action = base > 0 ? 'resume' : 'start';
    await this.commit(this.event('timer', base, { timer: { action } }));
    this.runningSince = Date.now();
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
