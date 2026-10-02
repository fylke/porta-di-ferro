<script lang="ts">
  import { onMount } from 'svelte';
  import { keepAwake } from '../lib/wakelock';
  import Scoreboard from './Scoreboard.svelte';
  import { Clock } from './lib-display.svelte';
  import { MatsLive, matOf, slotElapsed, upcoming } from '../lib/mats.svelte';
  import { t } from '../lib/i18n.svelte';

  let { mat }: { mat: number } = $props();

  // One physical mat of the event (phase 2), followed across disciplines: the scoreboard
  // says which discipline is on it.
  const live = new MatsLive();
  const clock = new Clock();

  onMount(() => {
    void live.start();
    clock.start();
    const release = keepAwake();
    return () => {
      clock.stop();
      live.stop();
      release();
    };
  });

  const here = $derived(matOf(live.view, mat));
  const current = $derived(here?.current ?? null);
  const match = $derived(current?.match ?? null);
  const names = $derived({ red: current?.red ?? '', blue: current?.blue ?? '' });
  const next = $derived(upcoming(here, 1)[0] ?? null);
  const elapsed = $derived(slotElapsed(current, live.receivedAt, clock.now));
</script>

<main>
  <div class="board">
    <Scoreboard {mat} {match} {names} {elapsed} discipline={current?.disciplineName ?? ''} />
  </div>
  {#if match?.state.ended || !match}
    <footer>
      {#if next}
        <span class="label">
          {t('Next on mat {n}', { n: mat })}{#if next.disciplineName !== current?.disciplineName} &middot; {next.disciplineName}{/if}
        </span>
        <span class="up">
          <span style="color: var(--bright-{next.match.options.red})">{next.red}</span>
          {t('v')}
          <span style="color: var(--bright-{next.match.options.blue})">{next.blue}</span>
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
  /* Capped by the width too, so two full names on an upright phone wrap onto a second
     line instead of a third and fourth. */
  .up {
    font-size: clamp(1rem, min(4vh, 6vw), 2.4rem);
    font-weight: 700;
    text-align: center;
  }
  .stale {
    position: absolute;
    top: 0.5rem;
    right: 0.75rem;
    font-size: 0.75rem;
    color: var(--amber-bright);
  }
</style>
