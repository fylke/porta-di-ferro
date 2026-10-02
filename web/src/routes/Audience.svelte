<script lang="ts">
  import { onMount } from 'svelte';
  import { keepAwake } from '../lib/wakelock';
  import Scoreboard from './Scoreboard.svelte';
  import { Clock } from './lib-display.svelte';
  import { MatsLive, matOf, slotElapsed, upcoming } from '../lib/mats.svelte';
  import { t } from '../lib/i18n.svelte';

  /**
   * The audience display (design §7 item 5): the mat's scoreboard, the winner and final
   * scores held prominently once a match is decided, the next match in a smaller but
   * still legible face, and an on-deck panel down the side of the three to five matches
   * after that -- which is what tells a competitor whether there is time to refill a
   * water bottle or take a jacket off.
   *
   * The result stays up because the mat follows its score keeper: the server points the
   * mat at whatever match the live score keeper is holding, finished or not, until Next
   * match is pressed. This screen renders that and adds nothing of its own.
   */
  let { mat }: { mat: number } = $props();

  // One physical mat of the event, across disciplines (phase 2): the queue down the side
  // may run from one discipline into the next, and each entry says whose it is.
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
  const elapsed = $derived(slotElapsed(current, live.receivedAt, clock.now));
  // The next match, then the ones after it: enough to read the queue, few enough to
  // keep the type large.
  const queue = $derived(upcoming(here, 6));
  const next = $derived(queue[0] ?? null);
  const deck = $derived(queue.slice(1, 6));
</script>

<main>
  <section class="stage">
    <div class="board">
      <Scoreboard {mat} {match} {names} {elapsed} discipline={current?.disciplineName ?? ''} />
    </div>
    <div class="next">
      {#if next}
        <span class="label">
          {t('Next on mat {n}', { n: mat })}{#if next.disciplineName !== current?.disciplineName} &middot; {next.disciplineName}{/if}
        </span>
        <span class="pair">
          <span class="who" style="background: var(--tint-{next.match.options.red}); color: var(--bright-{next.match.options.red})">{next.red}</span>
          <span class="v">{t('v')}</span>
          <span class="who" style="background: var(--tint-{next.match.options.blue}); color: var(--bright-{next.match.options.blue})">{next.blue}</span>
        </span>
      {:else if match}
        <span class="label">{t('Last match on mat {n}', { n: mat })}</span>
      {:else}
        <span class="label">{t('Mat {n}', { n: mat })}</span>
      {/if}
    </div>
  </section>

  <aside class="deck">
    <h2>{t('On deck')}</h2>
    {#if deck.length === 0}
      <p class="dim">{next ? t('Nothing after the next match.') : t('Nothing more on this mat.')}</p>
    {:else}
      <ol>
        {#each deck as s (`${s.discipline}/${s.match.id}`)}
          <li>
            <span class="n mono">{s.match.order}</span>
            <span class="who" style="background: var(--tint-{s.match.options.red}); color: var(--bright-{s.match.options.red})">{s.red}</span>
            <span class="who" style="background: var(--tint-{s.match.options.blue}); color: var(--bright-{s.match.options.blue})">{s.blue}</span>
            {#if s.disciplineName !== current?.disciplineName}<span class="in">{s.disciplineName}</span>{/if}
          </li>
        {/each}
      </ol>
    {/if}
  </aside>

  {#if !live.connected}
    <div class="stale">{t('Reconnecting…')}</div>
  {/if}
</main>

<style>
  main {
    height: 100dvh;
    display: grid;
    grid-template-columns: 1fr clamp(16rem, 26vw, 24rem);
    gap: 0.75rem;
    padding: 0.75rem;
    position: relative;
  }
  .stage {
    display: grid;
    grid-template-rows: 1fr auto;
    gap: 0.75rem;
    min-height: 0;
    min-width: 0;
  }
  .board {
    min-height: 0;
  }
  .next {
    display: grid;
    justify-items: center;
    gap: 0.4rem;
    padding: 0.8rem 1rem;
    background: var(--panel);
    border-radius: var(--radius);
  }
  .label {
    font-size: clamp(0.7rem, 1.8vh, 1.1rem);
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: var(--ink-dim);
  }
  /* Wraps rather than pushing the page sideways: two full names do not fit across a
     phone (issue #107). */
  .pair {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    align-items: center;
    gap: 0.6rem;
    max-width: 100%;
    font-size: clamp(1rem, 3.6vh, 2.2rem);
    font-weight: 700;
  }
  .v {
    color: var(--ink-dim);
    font-weight: 400;
  }
  /* Names sit on their competitor's colour, so the queue reads by colour from across
     the hall before it reads by name. */
  .who {
    padding: 0.2em 0.6em;
    border-radius: 8px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }

  .deck {
    display: grid;
    grid-template-rows: auto 1fr;
    gap: 0.5rem;
    padding: 0.8rem 1rem;
    background: var(--panel);
    border-radius: var(--radius);
    min-height: 0;
  }
  h2 {
    margin: 0;
    font-size: clamp(0.7rem, 1.8vh, 1.1rem);
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: var(--ink-dim);
  }
  ol {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    align-content: start;
    gap: 0.5rem;
    overflow: hidden;
  }
  li {
    display: grid;
    grid-template-columns: auto 1fr;
    grid-template-rows: auto auto;
    gap: 0.25rem 0.6rem;
    align-items: center;
    font-size: clamp(0.9rem, 2.4vh, 1.5rem);
    font-weight: 700;
  }
  li .in {
    grid-column: 2;
    font-size: 0.7em;
    font-weight: 600;
    color: var(--ink-dim);
  }
  li .n {
    grid-row: 1 / span 2;
    color: var(--ink-dim);
    font-weight: 400;
  }
  .dim {
    color: var(--ink-dim);
    margin: 0;
  }
  .stale {
    position: absolute;
    top: 0.5rem;
    right: 0.75rem;
    font-size: 0.75rem;
    color: var(--amber-bright);
  }

  @media (orientation: portrait), (max-width: 800px) {
    main {
      grid-template-columns: minmax(0, 1fr);
      grid-template-rows: 1fr auto;
    }
    .deck {
      max-height: 40dvh;
    }
  }
  /* A phone is held, not hung on a wall, so it can scroll: the board takes most of the
     screen and the queue follows below it, instead of all three squeezed into one
     screen with the names clipped off the board (issue #107). */
  @media (max-width: 34rem) {
    main {
      height: auto;
      min-height: 100dvh;
    }
    .stage {
      grid-template-rows: auto auto;
    }
    .board {
      height: 64dvh;
    }
    .deck {
      max-height: none;
    }
  }
</style>
