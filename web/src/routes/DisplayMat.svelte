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
  // What comes next, as the organizer chose for every mat screen (#110): the next match
  // along the bottom, the next few down the right, or nothing.
  const mode = $derived(live.view?.upcoming ?? 'bottom');
  const list = $derived(upcoming(here, 5));
</script>

<main class="mode-{mode}">
  <div class="board">
    <Scoreboard {mat} {match} {names} {elapsed} discipline={current?.disciplineName ?? ''} />
  </div>
  {#if mode === 'bottom'}
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
  {:else if mode === 'list'}
    <aside>
      <span class="label">{t('Next on mat {n}', { n: mat })}</span>
      {#if list.length === 0}
        <span class="none">{t('No more matches on mat {n}', { n: mat })}</span>
      {:else}
        <ol>
          {#each list as s (`${s.discipline}/${s.match.id}`)}
            <li>
              {#if s.disciplineName !== current?.disciplineName}<span class="disc">{s.disciplineName}</span>{/if}
              <span style="color: var(--bright-{s.match.options.red})">{s.red}</span>
              <span class="v">{t('v')}</span>
              <span style="color: var(--bright-{s.match.options.blue})">{s.blue}</span>
            </li>
          {/each}
        </ol>
      {/if}
    </aside>
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
  main.mode-none {
    grid-template-rows: 1fr;
  }
  /* The list down the right on a wide screen, under the board on an upright one. */
  main.mode-list {
    grid-template-columns: minmax(0, 1fr) minmax(12rem, 26%);
    grid-template-rows: minmax(0, 1fr);
  }
  @media (orientation: portrait) {
    main.mode-list {
      grid-template-columns: minmax(0, 1fr);
      grid-template-rows: minmax(0, 1fr) auto;
    }
  }
  aside {
    display: grid;
    align-content: start;
    gap: 0.6rem;
    padding: 0.9rem 1rem;
    background: var(--panel);
    border-radius: var(--radius);
    min-height: 0;
    overflow: hidden;
  }
  aside ol {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.7rem;
  }
  aside li {
    font-size: clamp(0.95rem, min(3vh, 2.2vw), 1.8rem);
    font-weight: 700;
    line-height: 1.25;
    overflow-wrap: anywhere;
  }
  aside .v {
    color: var(--ink-dim);
    font-weight: 400;
    margin: 0 0.25rem;
  }
  aside .disc {
    display: block;
    font-size: 0.6em;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--amber-bright);
  }
  aside .none {
    color: var(--ink-dim);
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
