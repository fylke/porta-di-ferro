<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { Live } from '../lib/live.svelte';
  import { ScoreKeeperSession } from '../lib/scorekeeper.svelte';
  import { Clock, formatClock, isFlashing } from '../lib/clock.svelte';
  import { keepAwake } from '../lib/wakelock';
  import CompetitorPanel from './variants/CompetitorPanel.svelte';
  import EndDialog from './EndDialog.svelte';
  import ConfirmDialog from './ConfirmDialog.svelte';
  import OptionsSheet from './OptionsSheet.svelte';
  import { matchesOn } from './lib-display.svelte';
  import { MSL, replay, type Side } from '../lib/match';
  import * as db from '../lib/db';
  import { ended, penaltyLoss } from '../lib/outcome';
  import { summarise } from '../lib/drift';

  let { mat, variant = 'panels' }: { mat: number; variant?: string } = $props();

  const live = new Live();
  const clock = new Clock();
  let sk = $state<ScoreKeeperSession | null>(null);
  let loadedMatch = $state('');
  let askUndo = $state(false);
  let askReset = $state(false);
  let askForfeit = $state<'red' | 'blue' | null>(null);
  let menuOpen = $state(false);
  let optionsOpen = $state(false);
  // Which final-exchange dialog the head referee has already answered "continue" to.
  let dismissedFinal = $state(0);

  onMount(() => {
    live.start();
    clock.start();
    const release = keepAwake();
    return () => {
      clock.stop();
      live.stop();
      sk?.log.stop();
      release();
    };
  });

  // Every match on this mat, in running order. The server's own idea of which one is up
  // is the first of these that is not complete -- but this device does not follow that
  // blindly. A finished match stays on screen, with its result, until the score keeper
  // presses Next match: the server moving the mat on the instant the end was written
  // meant the final score was replaced by the next two names before anyone had read it.
  const queue = $derived(matchesOn(live.snapshot, mat));
  let matchId = $state('');
  // Every match on the mat is done and the score keeper has said so.
  let exhausted = $state(false);
  const rememberedKey = $derived(`porta.mat.${mat}.current`);

  // Matches this device has finished that the server may not know about yet -- the
  // whole point of running a pool offline. Found by replaying the logs on this device, so
  // they survive a reload, and kept up to date as matches end here.
  let localDone = $state(new Set<string>());
  const queueKey = $derived(queue.map((m) => m.id).join(','));
  $effect(() => {
    const ids = queueKey ? queueKey.split(',') : [];
    if (ids.length === 0) return;
    void (async () => {
      const done = new Set<string>();
      for (const id of ids) {
        const events = await db.read(id);
        if (events.length > 0 && replay(MSL, events).ended) done.add(id);
      }
      // Merged rather than replaced, so a match that ended here since the scan began is
      // not forgotten; untracked, so the merge does not re-run the scan.
      localDone = new Set([...untrack(() => localDone), ...done]);
    })();
  });

  /** Open means neither the server nor this device has seen it end. */
  function isOpen(m: { id: string; status: string }): boolean {
    return m.status !== 'complete' && !localDone.has(m.id);
  }

  function firstOpen(): string {
    return queue.find(isOpen)?.id ?? '';
  }

  $effect(() => {
    if (queue.length === 0) return;
    // Still on a match the mat still has: stay there, finished or not.
    if (matchId && queue.some((m) => m.id === matchId)) return;
    // Otherwise pick up where this device left off, or where the mat is.
    let remembered = '';
    try {
      remembered = localStorage.getItem(rememberedKey) ?? '';
    } catch {
      // No memory on this browser; the mat's own position is the fallback.
    }
    const open = queue.find((m) => m.id === remembered && isOpen(m));
    matchId = open?.id ?? firstOpen();
  });

  $effect(() => {
    if (!matchId) return;
    try {
      localStorage.setItem(rememberedKey, matchId);
    } catch {
      // Fine without it.
    }
  });

  /** The score keeper has read the result and is ready for the next two. */
  function nextMatch() {
    const i = queue.findIndex((m) => m.id === matchId);
    const after = queue.slice(i + 1).find(isOpen);
    const elsewhere = queue.find((m) => isOpen(m) && m.id !== matchId);
    matchId = after?.id ?? elsewhere?.id ?? '';
    if (!matchId) {
      exhausted = true;
      try {
        localStorage.removeItem(rememberedKey);
      } catch {
        // Fine without it.
      }
    }
  }

  const view = $derived(queue.find((m) => m.id === matchId) ?? null);

  // Which side of this screen each competitor is on. This device's own choice, kept per
  // mat, and independent of the displays' -- so a score keeper who sits facing the mat
  // from the far side can mirror their screen without turning every scoreboard round.
  const swapKey = $derived(`porta.mat.${mat}.swap`);
  let swapHere = $state(false);
  $effect(() => {
    try {
      swapHere = localStorage.getItem(swapKey) === '1';
    } catch {
      swapHere = false;
    }
  });
  function setSwapHere(swap: boolean) {
    swapHere = swap;
    try {
      localStorage.setItem(swapKey, swap ? '1' : '0');
    } catch {
      // Fine without it.
    }
  }
  const order = $derived<[Side, Side]>(swapHere ? ['blue', 'red'] : ['red', 'blue']);
  const options = $derived(sk ? sk.options : { red: 'red', blue: 'blue', swapDisplay: false });
  const names = $derived.by(() => {
    const byId = new Map((live.snapshot?.competitors ?? []).map((c) => [c.id, c.name]));
    return {
      red: view ? (byId.get(view.red) ?? 'Red') : 'Red',
      blue: view ? (byId.get(view.blue) ?? 'Blue') : 'Blue',
    };
  });

  $effect(() => {
    if (matchId && matchId !== loadedMatch) {
      loadedMatch = matchId;
      dismissedFinal = 0;
      const next = new ScoreKeeperSession(matchId);
      sk?.log.stop();
      sk = next;
      void next.load();
    }
  });

  const matchState = $derived(sk ? sk.state : null);
  $effect(() => {
    if (matchState?.ended && matchId && !localDone.has(matchId)) {
      localDone = new Set([...localDone, matchId]);
    }
  });
  const elapsed = $derived(sk && matchState ? clock.elapsed(matchState, sk.runningSince) : 0);
  const flashing = $derived(matchState ? isFlashing(elapsed, matchState.ended) : false);

  const showEndDialog = $derived(
    !!matchState &&
      !matchState.ended &&
      matchState.pending !== 'none' &&
      !(matchState.pending === 'final_exchange' && dismissedFinal === matchState.lastSeq),
  );

  // The end dialog's wording. A penalty loss names the loser and why, because that is
  // the one result a head referee will be asked to justify; the others name the winner.
  const capText = $derived.by((): { headline: string; detail: string } => {
    if (!matchState || !sk) return { headline: '', detail: '' };
    if (matchState.pending === 'final_exchange') {
      return { headline: 'Was that the final exchange?', detail: '' };
    }
    if (matchState.pending === 'penalty_cap') {
      return penaltyLoss(MSL, matchState, names, sk.log.events);
    }
    if (matchState.red.score === matchState.blue.score) {
      return { headline: `Draw ${matchState.red.score}–${matchState.blue.score}`, detail: '' };
    }
    const leader = matchState.red.score > matchState.blue.score ? names.red : names.blue;
    const high = Math.max(matchState.red.score, matchState.blue.score);
    const low = Math.min(matchState.red.score, matchState.blue.score);
    return { headline: `${leader} wins ${high}–${low}`, detail: '' };
  });

  async function endMatch() {
    if (!matchState) return;
    const reason =
      matchState.pending === 'penalty_cap' ? 'penalty' : matchState.pending === 'point_cap' ? 'point_cap' : 'time';
    await sk?.end(elapsed, reason);
  }

  async function secondAction() {
    if (!matchState) return;
    if (matchState.pending === 'final_exchange') {
      // Play continues, and the dialog comes back after the next confirmation. Nothing is
      // written: "we carried on" is not an event, and a record of it would only be noise.
      dismissedFinal = matchState.lastSeq;
      return;
    }
    await sk?.undo(elapsed);
  }

  function askToForfeit(side: 'red' | 'blue') {
    menuOpen = false;
    askForfeit = side;
  }

  function escalate(side: 'red' | 'blue', levels: 2 | 3) {
    menuOpen = false;
    sk?.escalate(side, levels);
  }

  function openOptions() {
    menuOpen = false;
    optionsOpen = true;
  }

  function forfeit(side: 'red' | 'blue') {
    askForfeit = null;
    void sk?.forfeit(side);
  }

  function nameOf(side: 'red' | 'blue'): string {
    return side === 'red' ? names.red : names.blue;
  }

  // What the centre column says once the match is over, in place of the clock controls.
  const result = $derived(
    matchState?.ended && sk ? ended(MSL, matchState, names, sk.log.events) : null,
  );
</script>

<main class="sk">
  <div class="corner left">
    <button
      class="corner-btn"
      disabled={!matchState || matchState.undoableSeq === 0}
      onclick={() => (askUndo = true)}>UNDO</button
    >
  </div>
  <div class="corner right">
    <button class="corner-btn" aria-haspopup="menu" onclick={() => (menuOpen = !menuOpen)}
      >&hellip;</button
    >
    {#if menuOpen}
      <!-- The home for rare per-match controls: immediate penalty escalation and forfeits,
           grouped by competitor so the name is read before the consequence. An escalation
           is a pending selection that commits with Confirm exchange and has no dialog of
           its own; a forfeit ends the match on the spot, so it asks. -->
      <div class="menu" role="menu">
        {#each ['red', 'blue'] as const as side (side)}
          <div class="menu-head {side}">{nameOf(side)}</div>
          <button
            role="menuitem"
            disabled={!matchState || matchState.ended}
            onclick={() => escalate(side, 2)}
          >
            <span>Double warning</span><span class="why">loses a point</span>
          </button>
          <button
            role="menuitem"
            disabled={!matchState || matchState.ended}
            onclick={() => escalate(side, 3)}
          >
            <span>Triple warning</span><span class="why">loses the match</span>
          </button>
          <button
            role="menuitem"
            disabled={!matchState || matchState.ended}
            onclick={() => askToForfeit(side)}
          >
            <span>Forfeits</span><span class="why">recorded 0&ndash;8</span>
          </button>
        {/each}
        <div class="menu-head">Match</div>
        <button role="menuitem" disabled={!matchState} onclick={openOptions}>
          <span>Colours and sides&hellip;</span>
        </button>
        <div class="menu-head">This screen</div>
        <a role="menuitem" href="/score/{mat}?variant={variant === 'panels' ? 'edge' : 'panels'}">
          Try the other layout
        </a>
      </div>
    {/if}
  </div>

  {#if view && sk && matchState}
    <div class="grid">
      {@render panel(order[0])}

      <div class="centre" class:flashing>
        <div class="time mono">{formatClock(elapsed)}</div>
        {#if matchState.ended}
          <!-- The result holds the centre until Next match is pressed, so it can actually be
               read, and read back to the head referee, before the next two names appear. -->
          <div class="result" aria-live="polite">
            <span class="outcome">{result?.headline}</span>
            <span class="final mono">{result?.detail}</span>
          </div>
        {:else}
          <div class="clock-row">
            <button class="clock" onclick={() => void sk?.toggleClock(elapsed)}>
              {matchState.running ? 'PAUSE' : 'PLAY'}
            </button>
            <!-- For a clock started by mistake. Small, because it is rare; confirmed, because
                 it is a correction to the record rather than a pause. -->
            <button
              class="reset"
              aria-label="Reset the clock to zero"
              title="Reset the clock to zero"
              disabled={elapsed === 0 && !matchState.running}
              onclick={() => (askReset = true)}>&#8634;</button
            >
          </div>
        {/if}
        <div class="sync" class:offline={sk.log.sync === 'offline' || live.stale}>
          {#if sk.log.sync === 'offline'}
            Offline &middot; {sk.log.pendingCount} to send
          {:else if sk.log.aligned}
            Mat {mat} &middot; showing the server&rsquo;s scoring
          {:else if live.stale}
            Offline &middot; schedule from {new Date(live.cachedAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
          {:else}
            Mat {mat} &middot; pool {view.pool}
          {/if}
        </div>
      </div>

      {@render panel(order[1])}
    </div>

    {#if sk.log.drift}
      <!-- The two engines disagree about the same log. Non-blocking: the match goes on,
           and the score keeper decides whose numbers it goes on under. Either way the
           console has the full report. -->
      <div class="drift" role="alert">
        <span>
          The server scores this match differently ({sk.log.drift.fields.join(', ')}):
          {summarise(sk.log.drift.local, sk.log.drift.server)}.
        </span>
        <span class="drift-actions">
          <button onclick={() => sk?.log.alignToServer()}>Use the server's</button>
          <button onclick={() => sk?.log.dismissDrift()}>Keep this one</button>
        </span>
      </div>
    {/if}
    {#if matchState.ended}
      <button class="confirm next" onclick={nextMatch}>NEXT MATCH</button>
    {:else}
      <button class="confirm" onclick={() => void sk?.confirm(elapsed)}>CONFIRM EXCHANGE</button>
    {/if}
  {:else}
    <div class="waiting">
      {#if live.error && queue.length === 0}
        <p>{live.error}</p>
      {:else if exhausted || (queue.length > 0 && !firstOpen())}
        <p>Every match on mat {mat} is done.</p>
        <p class="dim">Nothing more is scheduled here. Check with the organizer.</p>
      {:else}
        <p>Waiting for a match on mat {mat}.</p>
        <p class="dim">This screen follows the mat. It fills in when a match is up.</p>
      {/if}
    </div>
  {/if}

  {#if optionsOpen && sk}
    <OptionsSheet
      {options}
      {names}
      {swapHere}
      onColour={(side, colour) => void sk?.setOptions({ [side]: colour }, elapsed)}
      onSwapHere={setSwapHere}
      onSwapDisplay={(swap) => void sk?.setOptions({ swapDisplay: swap }, elapsed)}
      onClose={() => (optionsOpen = false)}
    />
  {/if}

  {#if showEndDialog && matchState}
    <EndDialog
      pending={matchState.pending}
      headline={capText.headline}
      detail={capText.detail}
      onEnd={() => void endMatch()}
      onSecond={() => void secondAction()}
    />
  {/if}

  {#if askUndo}
    <ConfirmDialog
      headline="Undo the last exchange?"
      detail="It is recorded as a correction, so nothing is lost from the log."
      onConfirm={() => {
        askUndo = false;
        void sk?.undo(elapsed);
      }}
      onCancel={() => (askUndo = false)}
    />
  {/if}

  {#if askForfeit}
    <ConfirmDialog
      headline="{nameOf(askForfeit)} forfeits?"
      detail="Recorded 0–8. {nameOf(askForfeit === 'red' ? 'blue' : 'red')} takes the win and the match points; {nameOf(askForfeit)} earns none."
      confirmLabel="Yes, {nameOf(askForfeit)} forfeits"
      onConfirm={() => forfeit(askForfeit!)}
      onCancel={() => (askForfeit = null)}
    />
  {/if}

  {#if askReset}
    <ConfirmDialog
      headline="Reset the clock to 00:00?"
      detail="For a clock that was started by mistake. The scores stay as they are, and the reset is recorded in the log."
      confirmLabel="Yes, reset it"
      onConfirm={() => {
        askReset = false;
        void sk?.resetClock(elapsed);
      }}
      onCancel={() => (askReset = false)}
    />
  {/if}
</main>

{#snippet panel(side: Side)}
  {#if sk && matchState}
    <CompetitorPanel
      {side}
      colour={options[side]}
      name={names[side]}
      score={matchState[side].score}
      warnings={matchState[side].penalty}
      selection={sk.selection(side)}
      {variant}
      disabled={matchState.ended}
      onPoint={(v) => sk?.togglePoint(side, v)}
      onWarning={() => sk?.toggleWarning(side)}
    />
  {/if}
{/snippet}

<style>
  .sk {
    height: 100dvh;
    display: grid;
    grid-template-rows: 1fr auto;
    position: relative;
    overflow: hidden;
  }

  /* Red stays on the left and blue on the right in every layout unless the score keeper
     swaps them from the menu. That mapping mirrors the mat and must never move by itself,
     whatever the screen size: swapping sides is a deliberate action, not something a
     device rotation does. */
  .grid {
    display: grid;
    grid-template-columns: 1fr minmax(7rem, 0.55fr) 1fr;
    min-height: 0;
    padding-top: 2.6rem;
  }

  .centre {
    display: grid;
    grid-template-rows: auto 1fr auto;
    gap: 0.5rem;
    align-content: center;
    padding: 0.75rem 0.5rem;
    text-align: center;
    background: var(--panel);
  }
  /* The one place red does not mean the red competitor. It works because the timer sits in
     the neutral centre column and the whole area floods at once, which reads as an alarm
     rather than as identity -- so keep it a full-area change, not coloured digits. */
  .centre.flashing {
    animation: flash 1s steps(1, end) infinite;
  }
  @keyframes flash {
    0%,
    49% {
      background: var(--red);
    }
    50%,
    100% {
      background: var(--panel);
    }
  }

  /* Large, so the clock can be read at a glance rather than looked at directly -- but not
     oppressively so, because giving the number half the screen starves the scoring
     controls, which matter just as much. */
  .time {
    font-size: clamp(1.8rem, 6.5vh, 3.4rem);
    font-weight: 800;
    line-height: 1;
  }
  /* Play/pause is among the largest controls on the screen: the only one that must be hit
     fast. It cedes a narrow strip on its right to reset, which is rare and confirmed, so the
     two cannot be confused by size alone. */
  .clock-row {
    align-self: stretch;
    display: grid;
    grid-template-columns: 1fr auto;
    gap: 0.4rem;
    min-height: 0;
  }
  .clock {
    font-size: clamp(0.9rem, 2.6vh, 1.3rem);
    font-weight: 800;
    letter-spacing: 0.06em;
    background: var(--panel-2);
    border: 2px solid var(--line);
  }
  .reset {
    width: clamp(2.6rem, 6vh, 3.4rem);
    font-size: clamp(1.1rem, 3vh, 1.6rem);
    line-height: 1;
    background: var(--panel-2);
    border: 2px solid var(--line);
    color: var(--ink-dim);
  }
  .reset:disabled {
    opacity: 0.35;
  }
  .clock:active,
  .reset:active {
    filter: brightness(1.35);
  }
  .result {
    align-self: stretch;
    display: grid;
    align-content: center;
    gap: 0.3rem;
    padding: 0.6rem 0.4rem;
    border: 2px solid var(--line);
    border-radius: var(--radius);
    background: var(--panel-2);
  }
  .outcome {
    font-size: clamp(0.9rem, 2.4vh, 1.2rem);
    font-weight: 800;
    letter-spacing: 0.04em;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .final {
    font-size: clamp(1.4rem, 5vh, 2.6rem);
    font-weight: 800;
    line-height: 1;
  }
  .sync {
    font-size: 0.75rem;
    color: var(--ink-dim);
  }
  .sync.offline {
    color: var(--amber-bright);
    font-weight: 700;
  }

  /* Pressed every single exchange, so it takes the bottom of the screen, full width,
     where the thumb already is. */
  .confirm {
    padding: clamp(1rem, 3.5vh, 1.8rem);
    font-size: clamp(1.1rem, 3vh, 1.6rem);
    font-weight: 800;
    letter-spacing: 0.06em;
    border: none;
    border-radius: 0;
    background: var(--ok);
    color: #07120b;
  }
  .confirm:disabled {
    background: var(--panel-2);
    color: var(--ink-dim);
  }
  /* The same slot, a different job: the one press that moves the mat on. Neutral rather
     than green, so a thumb that has been hitting Confirm all match notices the change. */
  .confirm.next {
    background: var(--ink);
    color: #0d0f14;
  }
  .confirm:active {
    filter: brightness(1.3);
  }

  .drift {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem 1rem;
    align-items: center;
    justify-content: space-between;
    padding: 0.6rem 0.9rem;
    background: var(--amber);
    color: #1a1200;
    font-size: 0.9rem;
    font-weight: 600;
  }
  .drift-actions {
    display: flex;
    gap: 0.5rem;
  }
  .drift button {
    padding: 0.45rem 0.8rem;
    font-size: 0.85rem;
    font-weight: 700;
    background: #1a1200;
    color: var(--amber-bright);
    border: none;
  }

  /* Rare and destructive, so they sit outside the main grid rather than competing for
     space with the per-exchange controls. */
  .corner {
    position: absolute;
    top: 0.4rem;
    z-index: 20;
  }
  .corner.left {
    left: 0.5rem;
  }
  .corner.right {
    right: 0.5rem;
  }
  .corner-btn {
    padding: 0.4rem 0.9rem;
    font-size: 0.85rem;
    font-weight: 800;
    letter-spacing: 0.06em;
    background: var(--panel-2);
    border: 1px solid var(--line);
    color: var(--ink-dim);
  }
  .corner-btn:disabled {
    opacity: 0.35;
  }
  .menu {
    position: absolute;
    right: 0;
    top: 2.4rem;
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: var(--radius);
    display: grid;
    min-width: 17rem;
    max-height: calc(100dvh - 3.5rem);
    overflow: auto;
  }
  .menu button,
  .menu a {
    padding: 0.75rem 1rem;
    text-align: left;
    background: none;
    border: none;
    border-radius: 0;
    color: var(--ink);
    text-decoration: none;
    font-size: 0.95rem;
    display: flex;
    justify-content: space-between;
    gap: 1rem;
  }
  .menu button:disabled {
    opacity: 0.4;
  }
  .menu .why {
    color: var(--ink-dim);
    font-size: 0.8rem;
  }
  .menu-head {
    padding: 0.55rem 1rem 0.2rem;
    font-size: 0.7rem;
    font-weight: 800;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--ink-dim);
    border-top: 1px solid var(--line);
  }
  .menu-head:first-child {
    border-top: none;
  }
  .menu-head.red {
    color: var(--red-bright);
  }
  .menu-head.blue {
    color: var(--blue-bright);
  }
  .menu button:active,
  .menu a:active {
    background: var(--panel-2);
  }

  .waiting {
    display: grid;
    align-content: center;
    justify-items: center;
    gap: 0.5rem;
    text-align: center;
    padding: 2rem;
  }
  .dim {
    color: var(--ink-dim);
  }

  /* On a phone the timer and its controls move to the top and the two competitors sit
     closer together beneath, rather than shrinking the landscape design onto a smaller
     screen. */
  @media (orientation: portrait) and (max-width: 760px) {
    .grid {
      grid-template-columns: 1fr 1fr;
      grid-template-rows: auto 1fr;
    }
    .centre {
      grid-column: 1 / -1;
      grid-row: 1;
      grid-template-rows: auto auto auto;
    }
    .clock {
      padding: 0.9rem;
    }
    .reset {
      width: 3.2rem;
    }
  }
</style>
