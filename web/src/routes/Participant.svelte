<script lang="ts">
  import { onMount } from 'svelte';
  import { Clock, Live, liveElapsed, matchOn, nameLookup, upcomingOn } from './lib-display.svelte';
  import { formatClock } from '../lib/clock.svelte';
  import { t } from '../lib/i18n.svelte';
  import LangToggle from './LangToggle.svelte';
  import Schedule from './Schedule.svelte';
  import Standings from './Standings.svelte';

  /**
   * What a competitor or a spectator gets when they scan the code on the door (issue #98).
   *
   * Read-only, deliberately and completely. This is the one page in the application whose
   * audience is not running the event, and the address reaches it from every phone in the
   * hall — so there is no control on it that changes anything, not even a withdrawal.
   * The admin view is where the tournament is edited, and it is a different route.
   *
   * The order is the issue's: welcome, schedule, participants, mats, results. On a phone
   * that is the order you scroll. On anything wider the schedule and the mats move to a
   * side column, which is the sketch on the issue, and the reading order stays the same
   * because the source order does.
   */
  const live = new Live();
  const clock = new Clock();

  onMount(() => {
    live.start();
    clock.start();
    return () => {
      clock.stop();
      live.stop();
    };
  });

  const snapshot = $derived(live.snapshot);
  const event = $derived(snapshot?.tournament.event ?? {});
  const name = $derived(nameLookup(snapshot));
  const mats = $derived(
    Array.from({ length: snapshot?.tournament.mats ?? 0 }, (_, i) => i + 1),
  );

  // Sorted by name rather than by entry order: this is a list people look themselves up
  // in, and the order competitors were typed in is meaningless to them.
  const roster = $derived(
    [...(snapshot?.competitors ?? [])].sort((a, b) => a.name.localeCompare(b.name)),
  );

  const started = $derived(
    (snapshot?.pools ?? []).some((p) => p.matches.some((m) => m.status !== 'pending')),
  );
</script>

<main>
  <header>
    <h1>
      {snapshot?.instance.name || 'Porta di Ferro'}
    </h1>
    <LangToggle />
  </header>

  {#if !snapshot}
    <p class="loading">{live.error || t('Loading…')}</p>
  {:else}
    <div class="layout">
      <section class="welcome" class:empty={!event.welcome}>
        {#if event.welcome}
          <!-- The organizer's own words, from the admin view. Line breaks are theirs. -->
          <p class="intro">{event.welcome}</p>
        {:else}
          <p class="intro dim">{t('Welcome. The schedule and the results are on this page, and they update themselves.')}</p>
        {/if}
      </section>

      <section class="schedule">
        <h2>{t('Programme')}</h2>
        <Schedule items={event.schedule ?? []} />
      </section>

      <section class="people">
        <h2>{t('Competitors')} <span class="count">{roster.length}</span></h2>
        <p class="dim hint">{t('Tap a name for that person’s matches.')}</p>
        <ul class="roster">
          {#each roster as c (c.id)}
            <li class:out={c.withdrawn}>
              <a href="/who/{c.id}">
                <span class="who">{c.name}</span>
                <span class="club">{c.club}</span>
                {#if c.withdrawn}<span class="tag">{t('withdrawn')}</span>{/if}
              </a>
            </li>
          {/each}
          {#if roster.length === 0}
            <li class="dim">{t('Nobody is entered yet.')}</li>
          {/if}
        </ul>
      </section>

      <section class="mats">
        <h2>{t('Mats')}</h2>
        <ul class="matlist">
          {#each mats as mat (mat)}
            {@const current = matchOn(snapshot, mat)}
            {@const next = upcomingOn(snapshot, mat, 1)[0] ?? null}
            <li>
              <a class="mat" href="/display/mat/{mat}">
                <span class="matname">{t('Mat {n}', { n: mat })}</span>
                {#if current}
                  <span class="now">
                    <span class="red">{name(current.red)}</span>
                    <span class="score mono">
                      {#if current.status === 'pending'}
                        {t('up next')}
                      {:else}
                        {current.state.red.score}–{current.state.blue.score}
                      {/if}
                    </span>
                    <span class="blue">{name(current.blue)}</span>
                  </span>
                  {#if current.state.running}
                    <span class="clock mono">{formatClock(liveElapsed(current, live, clock.now))}</span>
                  {/if}
                {:else}
                  <span class="dim">{t('Nothing on this mat just now.')}</span>
                {/if}
                {#if next}
                  <span class="deck dim">
                    {t('On deck')}: {name(next.red)} {t('v')} {name(next.blue)}
                  </span>
                {/if}
              </a>
            </li>
          {/each}
        </ul>
      </section>

      <section class="results">
        <h2>{started ? t('Results') : t('Pools')}</h2>
        <Standings {snapshot} />
      </section>
    </div>
  {/if}
</main>

<style>
  main {
    max-width: 78rem;
    margin: 0 auto;
    padding: 1rem 1rem 3rem;
  }
  header {
    display: flex;
    align-items: center;
    gap: 1rem;
    margin-bottom: 1rem;
  }
  h1 {
    flex: 1;
    margin: 0;
    font-size: clamp(1.25rem, 4vw, 1.8rem);
    min-width: 0;
    overflow-wrap: anywhere;
  }
  h2 {
    margin: 0 0 0.6rem;
    font-size: 0.78rem;
    font-weight: 800;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--ink-dim);
    display: flex;
    align-items: baseline;
    gap: 0.5rem;
  }
  .count {
    font-size: 0.85rem;
    letter-spacing: 0;
    text-transform: none;
    font-weight: 600;
  }
  section {
    background: var(--panel);
    border-radius: var(--radius);
    padding: 1rem 1.1rem;
  }
  .loading {
    color: var(--ink-dim);
  }
  .dim {
    color: var(--ink-dim);
  }
  .hint {
    margin: -0.3rem 0 0.6rem;
    font-size: 0.82rem;
  }
  .intro {
    margin: 0;
    line-height: 1.55;
    /* The organizer typed it into a textarea; their line breaks are meant. */
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  /* One column on a phone, in the order the issue asks for. The source order is that
     order, so nothing below re-sorts the page for a screen reader -- the wider layouts
     only move boxes into a second column. */
  .layout {
    display: grid;
    gap: 0.8rem;
    grid-template-columns: minmax(0, 1fr);
  }
  /* Without this a grid item refuses to be narrower than its widest child, and one wide
     table takes the whole page off the side of a phone. */
  .layout > * {
    min-width: 0;
  }

  .roster {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 1px;
  }
  .roster a {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 0.5rem;
    padding: 0.55rem 0.5rem;
    border-radius: 6px;
    color: var(--ink);
    text-decoration: none;
  }
  .roster a:hover {
    background: var(--panel-2);
  }
  .who {
    font-weight: 600;
  }
  .club {
    color: var(--ink-dim);
    font-size: 0.85rem;
    margin-left: auto;
    text-align: right;
  }
  /* A competitor who has dropped out stays on the list. People look for the name they
     were told, and a name that has vanished reads as a mistake rather than a withdrawal. */
  .out a {
    opacity: 0.45;
  }
  .out .who {
    text-decoration: line-through;
  }
  .tag {
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--amber-bright);
    flex-basis: 100%;
  }

  .matlist {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.5rem;
  }
  .mat {
    display: grid;
    gap: 0.3rem;
    padding: 0.7rem 0.8rem;
    border-radius: var(--radius);
    background: var(--panel-2);
    color: var(--ink);
    text-decoration: none;
  }
  .mat:hover {
    outline: 1px solid var(--line);
  }
  .matname {
    font-size: 0.72rem;
    font-weight: 800;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--ink-dim);
  }
  .now {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
    gap: 0.5rem;
    align-items: baseline;
    font-weight: 700;
  }
  .now .red {
    color: var(--red-bright);
    text-align: right;
    overflow-wrap: anywhere;
  }
  .now .blue {
    color: var(--blue-bright);
    overflow-wrap: anywhere;
  }
  .score {
    font-size: 1.1rem;
    white-space: nowrap;
  }
  .clock {
    font-size: 1.5rem;
    font-weight: 800;
    line-height: 1;
    text-align: center;
    color: var(--amber-bright);
  }
  .deck {
    font-size: 0.8rem;
    overflow-wrap: anywhere;
  }

  /* Tablet portrait and up: the sketch's two columns. The schedule and the mats go to
     the side, the reading matter stays on the left. */
  @media (min-width: 46rem) {
    .layout {
      grid-template-columns: minmax(0, 1.6fr) minmax(0, 1fr);
      align-items: start;
    }
    .welcome,
    .people,
    .results {
      grid-column: 1;
    }
    .schedule,
    .mats {
      grid-column: 2;
    }
    .welcome {
      grid-row: 1;
    }
    .schedule {
      grid-row: 1;
    }
    .people {
      grid-row: 2;
    }
    .mats {
      grid-row: 2 / span 2;
      align-self: start;
    }
    .results {
      grid-row: 3;
    }
  }

  /* A laptop and up has room for the competitor list in two columns rather than one
     very long one. */
  @media (min-width: 64rem) {
    .roster {
      grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
      gap: 1px 1rem;
    }
  }
</style>
