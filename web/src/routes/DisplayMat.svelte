<script lang="ts">
  import { onMount } from 'svelte';
  import { keepAwake } from '../lib/wakelock';
  import Scoreboard from './Scoreboard.svelte';
  import { Clock, Live, matchOn, namesFor, nextOn } from './lib-display.svelte';

  let { mat }: { mat: number } = $props();

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

  const match = $derived(matchOn(live.snapshot, mat));
  const names = $derived(namesFor(live.snapshot, match));
  const upcoming = $derived(nextOn(live.snapshot, mat));
  const upcomingNames = $derived(namesFor(live.snapshot, upcoming));
  // A display has no writer of its own, so it anchors the clock to the moment it saw the
  // match start running. Close enough for a scoreboard, and never wrong in a way anyone
  // can see from across a hall.
  let startedAt = $state(Date.now());
  let wasRunning = $state(false);
  $effect(() => {
    const running = !!match?.state.running;
    if (running && !wasRunning) startedAt = Date.now();
    wasRunning = running;
  });
  const elapsed = $derived(
    match?.state.running
      ? match.state.elapsedMs + (clock.now - startedAt)
      : (match?.state.elapsedMs ?? 0),
  );
</script>

<main>
  <div class="board">
    <Scoreboard {mat} {match} {names} {elapsed} />
  </div>
  {#if match?.state.ended || !match}
    <footer>
      {#if upcoming}
        <span class="label">Next on mat {mat}</span>
        <span class="up"><span class="red">{upcomingNames.red}</span> v <span class="blue">{upcomingNames.blue}</span></span>
      {:else}
        <span class="label">No more matches on mat {mat}</span>
      {/if}
    </footer>
  {/if}
  {#if !live.connected}
    <div class="stale">Reconnecting&hellip;</div>
  {/if}
</main>

<style>
  main {
    height: 100dvh;
    display: grid;
    grid-template-rows: 1fr auto;
    gap: 0.75rem;
    padding: 0.75rem;
    position: relative;
  }
  .board {
    min-height: 0;
  }
  footer {
    display: grid;
    justify-items: center;
    gap: 0.3rem;
    padding: 0.75rem;
    background: var(--panel);
    border-radius: var(--radius);
  }
  .label {
    font-size: clamp(0.7rem, 1.8vh, 1.1rem);
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: var(--ink-dim);
  }
  .up {
    font-size: clamp(1rem, 4vh, 2.4rem);
    font-weight: 700;
  }
  .red {
    color: var(--red-bright);
  }
  .blue {
    color: var(--blue-bright);
  }
  .stale {
    position: absolute;
    top: 0.5rem;
    right: 0.75rem;
    font-size: 0.75rem;
    color: var(--amber-bright);
  }
</style>
