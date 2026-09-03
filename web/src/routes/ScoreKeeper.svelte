<script lang="ts">
  import { onMount } from 'svelte';
  import { Live } from '../lib/live.svelte';
  import { ScoreKeeperSession } from '../lib/scorekeeper.svelte';
  import { Clock, formatClock, isFlashing } from '../lib/clock.svelte';
  import { keepAwake } from '../lib/wakelock';
  import CompetitorPanel from './variants/CompetitorPanel.svelte';
  import EndDialog from './EndDialog.svelte';
  import ConfirmDialog from './ConfirmDialog.svelte';

  let { mat, variant = 'panels' }: { mat: number; variant?: string } = $props();

  const live = new Live();
  const clock = new Clock();
  let sk = $state<ScoreKeeperSession | null>(null);
  let loadedMatch = $state('');
  let askUndo = $state(false);
  let menuOpen = $state(false);
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

  // The mat follows whichever match is up next there: when one finishes, the next appears.
  const matchId = $derived(live.snapshot?.mats?.[String(mat)] ?? '');
  const view = $derived(
    live.snapshot?.pools.flatMap((p) => p.matches).find((m) => m.id === matchId) ?? null,
  );
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
  const elapsed = $derived(sk && matchState ? clock.elapsed(matchState, sk.runningSince) : 0);
  const flashing = $derived(matchState ? isFlashing(elapsed, matchState.ended) : false);

  const showEndDialog = $derived(
    !!matchState &&
      !matchState.ended &&
      matchState.pending !== 'none' &&
      !(matchState.pending === 'final_exchange' && dismissedFinal === matchState.lastSeq),
  );

  const capHeadline = $derived.by(() => {
    if (!matchState) return '';
    if (matchState.pending === 'final_exchange') return 'Time is up. Was that the final exchange?';
    if (matchState.red.score === matchState.blue.score) return `Draw ${matchState.red.score}–${matchState.blue.score}`;
    const leader = matchState.red.score > matchState.blue.score ? names.red : names.blue;
    const high = Math.max(matchState.red.score, matchState.blue.score);
    const low = Math.min(matchState.red.score, matchState.blue.score);
    return `${leader} wins ${high}–${low}`;
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

  function forfeit(side: 'red' | 'blue') {
    menuOpen = false;
    void sk?.forfeit(side);
  }
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
      <!-- The home for rare per-match controls. It holds forfeits now; Milestone 2 adds
           immediate penalty escalation and the colour and side options. Establishing the
           slot now avoids reopening a deliberately full grid to make room later. -->
      <div class="menu" role="menu">
        <button role="menuitem" onclick={() => forfeit('red')}>{names.red} forfeits</button>
        <button role="menuitem" onclick={() => forfeit('blue')}>{names.blue} forfeits</button>
        <a role="menuitem" href="/score/{mat}?variant={variant === 'panels' ? 'edge' : 'panels'}">
          Try the other layout
        </a>
      </div>
    {/if}
  </div>

  {#if view && sk && matchState}
    <div class="grid">
      <CompetitorPanel
        side="red"
        name={names.red}
        score={matchState.red.score}
        warnings={matchState.red.penalty}
        selection={sk.red}
        {variant}
        disabled={matchState.ended}
        onPoint={(v) => sk?.togglePoint('red', v)}
        onWarning={() => sk?.toggleWarning('red')}
      />

      <div class="centre" class:flashing>
        <div class="time mono">{formatClock(elapsed)}</div>
        <button class="clock" disabled={matchState.ended} onclick={() => void sk?.toggleClock(elapsed)}>
          {matchState.running ? 'PAUSE' : 'PLAY'}
        </button>
        <div class="sync" class:offline={sk.log.sync === 'offline'}>
          {#if sk.log.sync === 'offline'}
            Offline &middot; {sk.log.pendingCount} to send
          {:else}
            Mat {mat} &middot; pool {view.pool}
          {/if}
        </div>
      </div>

      <CompetitorPanel
        side="blue"
        name={names.blue}
        score={matchState.blue.score}
        warnings={matchState.blue.penalty}
        selection={sk.blue}
        {variant}
        disabled={matchState.ended}
        onPoint={(v) => sk?.togglePoint('blue', v)}
        onWarning={() => sk?.toggleWarning('blue')}
      />
    </div>

    <button class="confirm" disabled={matchState.ended} onclick={() => void sk?.confirm(elapsed)}>
      {matchState.ended ? 'MATCH OVER' : 'CONFIRM EXCHANGE'}
    </button>
  {:else}
    <div class="waiting">
      <p>{live.error ? live.error : `Waiting for a match on mat ${mat}.`}</p>
      <p class="dim">This screen follows the mat. It fills in when a match is up.</p>
    </div>
  {/if}

  {#if showEndDialog && matchState}
    <EndDialog
      pending={matchState.pending}
      headline={capHeadline}
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
</main>

<style>
  .sk {
    height: 100dvh;
    display: grid;
    grid-template-rows: 1fr auto;
    position: relative;
    overflow: hidden;
  }

  /* Red stays on the left and blue on the right in every layout. That mapping mirrors the
     mat and must never move, whatever the screen size: swapping sides is a deliberate
     action, not something a device rotation does. */
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
  /* Among the largest controls on the screen: the only one that must be hit fast. */
  .clock {
    align-self: stretch;
    font-size: clamp(0.9rem, 2.6vh, 1.3rem);
    font-weight: 800;
    letter-spacing: 0.06em;
    background: var(--panel-2);
    border: 2px solid var(--line);
  }
  .clock:active {
    filter: brightness(1.35);
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
  .confirm:active {
    filter: brightness(1.3);
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
    min-width: 14rem;
    overflow: hidden;
  }
  .menu button,
  .menu a {
    padding: 0.85rem 1rem;
    text-align: left;
    background: none;
    border: none;
    border-radius: 0;
    color: var(--ink);
    text-decoration: none;
    font-size: 0.95rem;
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
  }
</style>
