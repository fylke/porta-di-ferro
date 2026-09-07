<script lang="ts">
  import type { MatchView } from '../api';
  import { formatClock, isFlashing } from '../lib/clock.svelte';

  /**
   * One mat's scoreboard, in the same colour language as the score keeper view so a
   * spectator glancing between screens does not have to relearn it.
   *
   * During a match: names and colours, scores, warning triangles, the match time, and the
   * winner with final scores once decided. Between matches: the next match on that mat --
   * not pool standings, because there is never enough time between matches for anyone to
   * read them.
   */
  let {
    mat,
    match,
    names,
    elapsed,
    compact = false,
  }: {
    mat: number;
    match: MatchView | null;
    names: { red: string; blue: string };
    elapsed: number;
    compact?: boolean;
  } = $props();

  const board = $derived(match?.state ?? null);
  const flashing = $derived(board ? isFlashing(elapsed, board.ended) : false);
  const decided = $derived(!!board?.ended);
  const winnerName = $derived(
    board?.winner === 'red' ? names.red : board?.winner === 'blue' ? names.blue : '',
  );
</script>

<section class="board" class:compact class:flashing>
  {#if !match || !board}
    <div class="idle">
      <span class="mat">Mat {mat}</span>
      <span class="dim">No match up yet</span>
    </div>
  {:else}
    <div class="side red">
      <div class="name">{names.red}</div>
      <div class="score mono">{board.red.score}</div>
      <div class="warns">
        {#each { length: board.red.penalty } as _, i (i)}<span>&#9650;</span>{/each}
      </div>
    </div>

    <div class="centre">
      <div class="mat">Mat {mat}</div>
      {#if decided}
        <div class="result">{winnerName ? `${winnerName} wins` : 'Draw'}</div>
      {:else}
        <div class="time mono">{formatClock(elapsed)}</div>
      {/if}
    </div>

    <div class="side blue">
      <div class="name">{names.blue}</div>
      <div class="score mono">{board.blue.score}</div>
      <div class="warns">
        {#each { length: board.blue.penalty } as _, i (i)}<span>&#9650;</span>{/each}
      </div>
    </div>
  {/if}
</section>

<style>
  .board {
    display: grid;
    grid-template-columns: 1fr auto 1fr;
    align-items: center;
    gap: 1rem;
    padding: clamp(0.5rem, 2vh, 2rem);
    background: var(--panel);
    border-radius: var(--radius);
    min-height: 0;
    height: 100%;
  }
  .board.flashing {
    animation: boardflash 1s steps(1, end) infinite;
  }
  @keyframes boardflash {
    0%,
    49% {
      background: var(--red);
    }
    50%,
    100% {
      background: var(--panel);
    }
  }

  .side {
    display: grid;
    justify-items: center;
    gap: 0.25rem;
    padding: clamp(0.4rem, 1.5vh, 1.5rem);
    border-radius: var(--radius);
    min-width: 0;
  }
  .side.red {
    background: var(--red-tint);
  }
  .side.blue {
    background: var(--blue-tint);
  }
  .name {
    font-size: clamp(1rem, 4vh, 3rem);
    font-weight: 700;
    text-align: center;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .score {
    font-size: clamp(3rem, 22vh, 14rem);
    font-weight: 800;
    line-height: 0.95;
  }
  .warns {
    display: flex;
    gap: 0.2rem;
    color: var(--amber-bright);
    font-size: clamp(0.9rem, 3vh, 2rem);
    min-height: 1em;
  }

  .centre {
    display: grid;
    justify-items: center;
    gap: 0.4rem;
    text-align: center;
  }
  .mat {
    font-size: clamp(0.7rem, 2vh, 1.2rem);
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: var(--ink-dim);
  }
  .time {
    font-size: clamp(1.6rem, 9vh, 6rem);
    font-weight: 800;
    line-height: 1;
  }
  .result {
    font-size: clamp(1.1rem, 5vh, 3rem);
    font-weight: 800;
  }

  .idle {
    grid-column: 1 / -1;
    display: grid;
    justify-items: center;
    gap: 0.5rem;
  }
  .dim {
    color: var(--ink-dim);
    font-size: clamp(0.9rem, 3vh, 1.8rem);
  }

  /* Three mats or more: one compact row each, like a departures board. Wide short rows
     stay legible at a distance in a way shrunken scoreboards do not. */
  .board.compact {
    grid-template-columns: 1fr auto 1fr;
    padding: 0.6rem 1rem;
    gap: 0.75rem;
  }
  .compact .side {
    grid-template-columns: 1fr auto auto;
    justify-items: start;
    align-items: center;
    gap: 0.75rem;
    padding: 0.4rem 0.75rem;
  }
  .compact .name {
    font-size: clamp(0.9rem, 3.5vh, 2rem);
    text-align: left;
  }
  .compact .score {
    font-size: clamp(1.4rem, 5vh, 3rem);
  }
  .compact .warns {
    font-size: 0.8rem;
  }
  .compact .time,
  .compact .result {
    font-size: clamp(1rem, 4vh, 2.2rem);
  }
</style>
