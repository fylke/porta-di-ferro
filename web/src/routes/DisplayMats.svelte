<script lang="ts">
  import { onMount } from 'svelte';
  import { keepAwake } from '../lib/wakelock';
  import Scoreboard from './Scoreboard.svelte';
  import { Clock, Live, matchOn, namesFor } from './lib-display.svelte';

  /**
   * Every active mat on a single screen, or a chosen subset via ?ids=1,2.
   *
   * This is what keeps the MVP hardware-cheap: if the organizers can find one spot
   * visible from both mats, the whole event needs a single extra screen on a single video
   * output, which almost any laptop already has.
   *
   * The layout adapts to how many mats it is asked to show rather than shrinking one
   * design: one full scoreboard, two side by side, and three or more as compact rows.
   * A rotating carousel would keep type large but guarantees the mat someone cares about
   * is off-screen exactly when they look.
   */
  let { ids = '' }: { ids?: string } = $props();

  const live = new Live();
  const clock = new Clock();

  onMount(() => {
    live.start();
    clock.start();
    const release = keepAwake();
    return () => {
      clock.stop();
      live.stop();
      release();
    };
  });

  const mats = $derived.by(() => {
    const requested = ids
      .split(',')
      .map((s) => Number(s.trim()))
      .filter((n) => Number.isFinite(n) && n > 0);
    if (requested.length > 0) return requested;
    const count = live.snapshot?.tournament.mats ?? 0;
    return Array.from({ length: count }, (_, i) => i + 1);
  });

  const compact = $derived(mats.length >= 3);
  const started = new Map<number, number>();

  function elapsedFor(mat: number): number {
    const match = matchOn(live.snapshot, mat);
    if (!match) return 0;
    if (!match.state.running) {
      started.delete(mat);
      return match.state.elapsedMs;
    }
    if (!started.has(mat)) started.set(mat, Date.now());
    return match.state.elapsedMs + (clock.now - (started.get(mat) ?? Date.now()));
  }
</script>

<main class:compact style="--rows: {mats.length}">
  {#each mats as mat (mat)}
    {@const match = matchOn(live.snapshot, mat)}
    <Scoreboard {mat} {match} names={namesFor(live.snapshot, match)} elapsed={elapsedFor(mat)} {compact} />
  {/each}
  {#if mats.length === 0}
    <p class="empty">No mats are set up yet.</p>
  {/if}
</main>

<style>
  main {
    height: 100dvh;
    display: grid;
    gap: 0.75rem;
    padding: 0.75rem;
    grid-auto-rows: 1fr;
  }
  /* Two mats: side by side, both full size. */
  main:not(.compact) {
    grid-template-columns: repeat(auto-fit, minmax(20rem, 1fr));
  }
  /* Three or more: one row each, wide and short. */
  main.compact {
    grid-template-columns: 1fr;
  }
  .empty {
    align-self: center;
    justify-self: center;
    color: var(--ink-dim);
  }
</style>
