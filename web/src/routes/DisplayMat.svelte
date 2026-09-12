<script lang="ts">
  import { onMount } from 'svelte';
  import { keepAwake } from '../lib/wakelock';
  import Scoreboard from './Scoreboard.svelte';
  import { Clock, Live, liveElapsed, matchOn, namesFor, nextOn } from './lib-display.svelte';
  import { t } from '../lib/i18n.svelte';

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
  const elapsed = $derived(liveElapsed(match, live, clock.now));
</script>

<main>
  <div class="board">
    <Scoreboard {mat} {match} {names} {elapsed} discipline={live.snapshot?.instance.name ?? ''} />
  </div>
  {#if match?.state.ended || !match}
    <footer>
      {#if upcoming}
        <span class="label">{t('Next on mat {n}', { n: mat })}</span>
        <span class="up">
          <span style="color: var(--bright-{upcoming.options.red})">{upcomingNames.red}</span>
          {t('v')}
          <span style="color: var(--bright-{upcoming.options.blue})">{upcomingNames.blue}</span>
        </span>
      {:else}
        <span class="label">{t('No more matches on mat {n}', { n: mat })}</span>
      {/if}
    </footer>
  {/if}
  {#if !live.connected}
    <div class="stale">{t('Reconnecting…')}</div>
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
  .stale {
    position: absolute;
    top: 0.5rem;
    right: 0.75rem;
    font-size: 0.75rem;
    color: var(--amber-bright);
  }
</style>
