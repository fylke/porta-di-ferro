<script lang="ts">
  import { onMount } from 'svelte';
  import { Clock, Live, liveElapsed, nameLookup, roundLabel } from './lib-display.svelte';
  import { formatClock } from '../lib/clock.svelte';
  import { t } from '../lib/i18n.svelte';
  import LangToggle from './LangToggle.svelte';

  /**
   * One person's day (issue #98): every match they are in, in the order the hall runs
   * them, with what happened in the ones that are done.
   *
   * Reached by tapping a name on the landing page, and worth its own address: this is the
   * link a competitor sends their club, and the one a spectator keeps open to catch
   * somebody they came to watch.
   *
   * Read-only like the page it came from.
   */
  let { id }: { id: string } = $props();

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
  const name = $derived(nameLookup(snapshot));
  const person = $derived((snapshot?.competitors ?? []).find((c) => c.id === id) ?? null);

  /**
   * Their matches, in running order: the pools as the mats queue them, then the bracket.
   * A bracket match whose competitors are not decided yet cannot be theirs, so it is not
   * here -- the page says what is known, and "you might meet the winner of…" is not.
   */
  const fixtures = $derived.by(() => {
    if (!snapshot) return [];
    const rows = [];
    for (const pool of snapshot.pools) {
      for (const m of pool.matches) {
        if (m.red !== id && m.blue !== id) continue;
        rows.push({ match: m, where: t('Pool {n}', { n: pool.number }) });
      }
    }
    for (const m of snapshot.bracket?.matches ?? []) {
      if (m.red !== id && m.blue !== id) continue;
      rows.push({ match: m, where: roundLabel(m) });
    }
    return rows;
  });

  const standing = $derived.by(() => {
    for (const pool of snapshot?.pools ?? []) {
      const row = pool.standings.find((s) => s.competitor === id);
      if (row) return { row, pool: pool.number };
    }
    return null;
  });

  /** Which side they are on, so the row can say who they are actually fencing. */
  function opponent(m: { red: string; blue: string }): string {
    return name(m.red === id ? m.blue : m.red);
  }
  function theirScore(m: { red: string; state: { red: { score: number }; blue: { score: number } } }): string {
    const mine = m.red === id ? m.state.red.score : m.state.blue.score;
    const theirs = m.red === id ? m.state.blue.score : m.state.red.score;
    return `${mine}–${theirs}`;
  }
  function outcome(m: {
    red: string;
    status: string;
    state: { ended: boolean; winner: string };
  }): 'won' | 'lost' | 'drew' | '' {
    if (!m.state.ended) return '';
    if (!m.state.winner) return 'drew';
    const mine = m.red === id ? 'red' : 'blue';
    return m.state.winner === mine ? 'won' : 'lost';
  }
</script>

<main>
  <header>
    <a class="back" href="/">&larr; {t('Everyone')}</a>
    <LangToggle />
  </header>

  {#if !snapshot}
    <p class="dim">{live.error || t('Loading…')}</p>
  {:else if !person}
    <h1>{t('No such competitor')}</h1>
    <p class="dim">{t('That name is not in this tournament. It may be in another discipline.')}</p>
  {:else}
    <h1 class:out={person.withdrawn}>{person.name}</h1>
    <p class="club">
      {person.club}
      {#if person.withdrawn}<span class="tag">{t('withdrawn')}</span>{/if}
    </p>

    {#if person.withdrawn}
      <p class="warn">{t('This competitor has withdrawn. Their results are voided and do not count towards anyone else’s standing.')}</p>
    {/if}

    {#if standing}
      <section class="summary">
        <h2>{t('Pool {n}', { n: standing.pool })}</h2>
        <dl>
          <div><dt>{t('Rank')}</dt><dd class="mono big">{standing.row.rank}</dd></div>
          <div><dt>{t('Matches')}</dt><dd class="mono">{standing.row.completed}</dd></div>
          <div><dt>{t('W-D-L')}</dt><dd class="mono">{standing.row.wins}-{standing.row.draws}-{standing.row.losses}</dd></div>
          <div><dt title={t('Match point index')}>MPI</dt><dd class="mono">{standing.row.matchPointIndex.toFixed(2)}</dd></div>
        </dl>
      </section>
    {/if}

    <section>
      <h2>{t('Matches')}</h2>
      {#if fixtures.length === 0}
        <p class="dim">{t('No matches yet. They appear here once the pools are drawn.')}</p>
      {:else}
        <ol class="fixtures">
          {#each fixtures as f (f.match.id)}
            <li class={f.match.status}>
              <span class="where">
                {f.where}
                <span class="dim">&middot; {t('mat {n}', { n: f.match.mat })}</span>
              </span>
              <span class="versus">
                <span class="dim">{t('v')}</span>
                {opponent(f.match)}
              </span>
              <span class="result">
                {#if f.match.status === 'pending'}
                  <span class="dim">{t('to come')}</span>
                {:else}
                  <span class="mono score {outcome(f.match)}">{theirScore(f.match)}</span>
                  {#if f.match.state.running}
                    <span class="mono clock">{formatClock(liveElapsed(f.match, live, clock.now))}</span>
                  {:else if outcome(f.match) === 'won'}
                    <span class="verdict won">{t('won')}</span>
                  {:else if outcome(f.match) === 'lost'}
                    <span class="verdict">{t('lost')}</span>
                  {:else if outcome(f.match) === 'drew'}
                    <span class="verdict">{t('drew')}</span>
                  {/if}
                {/if}
              </span>
            </li>
          {/each}
        </ol>
      {/if}
    </section>
  {/if}
</main>

<style>
  main {
    max-width: 44rem;
    margin: 0 auto;
    padding: 1rem 1rem 3rem;
  }
  header {
    display: flex;
    align-items: center;
    gap: 1rem;
    margin-bottom: 0.8rem;
  }
  .back {
    flex: 1;
    color: var(--ink-dim);
    text-decoration: none;
    font-size: 0.9rem;
  }
  .back:hover {
    color: var(--ink);
  }
  h1 {
    margin: 0;
    font-size: clamp(1.4rem, 5vw, 2rem);
    overflow-wrap: anywhere;
  }
  h1.out {
    text-decoration: line-through;
    opacity: 0.6;
  }
  .club {
    margin: 0.2rem 0 1rem;
    color: var(--ink-dim);
  }
  .tag {
    margin-left: 0.5rem;
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--amber-bright);
  }
  .warn {
    margin: 0 0 1rem;
    color: var(--amber-bright);
    line-height: 1.5;
    font-size: 0.9rem;
  }
  section {
    background: var(--panel);
    border-radius: var(--radius);
    padding: 1rem 1.1rem;
    margin-bottom: 0.8rem;
  }
  h2 {
    margin: 0 0 0.7rem;
    font-size: 0.78rem;
    font-weight: 800;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--ink-dim);
  }
  .dim {
    color: var(--ink-dim);
  }

  dl {
    margin: 0;
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(5rem, 1fr));
    gap: 0.8rem;
  }
  dl div {
    display: grid;
    gap: 0.15rem;
  }
  dt {
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--ink-dim);
  }
  dd {
    margin: 0;
    font-weight: 700;
  }
  .big {
    font-size: 1.6rem;
    line-height: 1;
  }

  .fixtures {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.3rem;
  }
  .fixtures li {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 0.2rem 0.8rem;
    padding: 0.55rem 0.6rem;
    border-radius: 6px;
    background: var(--panel-2);
    align-items: baseline;
  }
  .fixtures li.running {
    outline: 2px solid var(--ok);
  }
  .where {
    font-size: 0.75rem;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--ink-dim);
    grid-column: 1;
  }
  .versus {
    grid-column: 1;
    font-weight: 600;
    overflow-wrap: anywhere;
  }
  .result {
    grid-column: 2;
    grid-row: 1 / span 2;
    display: grid;
    justify-items: end;
    gap: 0.15rem;
    align-content: center;
    white-space: nowrap;
  }
  .score {
    font-size: 1.15rem;
    font-weight: 800;
  }
  .score.won {
    color: var(--ok);
  }
  .score.lost {
    color: var(--ink-dim);
  }
  .clock {
    color: var(--amber-bright);
    font-weight: 700;
  }
  .verdict {
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--ink-dim);
  }
  .verdict.won {
    color: var(--ok);
  }
</style>
