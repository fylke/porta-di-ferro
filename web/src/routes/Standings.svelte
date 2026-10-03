<script lang="ts">
  import type { Snapshot } from '../api';
  import { nameLookup } from './lib-display.svelte';
  import BracketTree from './BracketTree.svelte';
  import FoldButton from './FoldButton.svelte';
  import { Folds } from '../lib/folds.svelte';
  import { t } from '../lib/i18n.svelte';

  /**
   * The rosters, the results and the bracket, for people who are not running the event
   * (issue #98).
   *
   * The same numbers the organizer sees, with every control taken off. Before a pool has
   * been fenced it is a roster -- who is in it and who meets whom -- and after it has
   * started the table comes up beside the matches. That is one component rather than two,
   * because a pool crosses from one to the other halfway through a morning and a page
   * that changed shape at that moment would look broken.
   */
  let { snapshot }: { snapshot: Snapshot | null } = $props();

  // What this viewer has folded away, per discipline (#110).
  // svelte-ignore state_referenced_locally
  const folds = new Folds(`${snapshot?.instance.slug ?? ''}/landing`);

  const name = $derived(nameLookup(snapshot));
  const fmt = (n: number) => (Number.isFinite(n) ? n.toFixed(2) : '0.00');
  const pools = $derived(snapshot?.pools ?? []);
  const bracket = $derived(snapshot?.bracket ?? null);
  const podium = $derived(bracket?.podium ?? null);

</script>

{#if pools.length === 0}
  <p class="dim">{t('The pools have not been drawn yet.')}</p>
{/if}

{#if podium?.first}
  <!-- Top of the section once it exists: it is the thing everyone came to find out. -->
  <div class="podium">
    <span class="gold"><strong>1</strong> {name(podium.first)}</span>
    <span><strong>2</strong> {name(podium.second)}</span>
    {#if podium.third}<span><strong>3</strong> {name(podium.third)}</span>{/if}
  </div>
{/if}

{#if bracket}
  <!-- The bracket as it is drawn on paper (#112). -->
  <div class="bracket">
    <h3><FoldButton open={folds.open('bracket')} label={t('Eliminations')} ontoggle={() => folds.toggle('bracket')} />{t('Eliminations')}</h3>
    {#if folds.open('bracket')}<BracketTree matches={bracket.matches} {name} />{/if}
  </div>
{/if}

<div class="pools">
  {#each pools as pool (pool.number)}
    {@const started = pool.matches.some((m) => m.status !== 'pending')}
    <section>
      <h3>
        <FoldButton open={folds.open(`pool-${pool.number}`)} label={t('Pool {n}', { n: pool.number })} ontoggle={() => folds.toggle(`pool-${pool.number}`)} />
        {t('Pool {n}', { n: pool.number })}
        <span class="meta">{t('mat {n}', { n: pool.mat })}</span>
        <span class="meta">{pool.complete ? t('complete') : started ? t('in progress') : t('not started')}</span>
      </h3>

      {#if folds.open(`pool-${pool.number}`)}
      {#if started}
        <div class="scroller">
        <table>
          <thead>
            <tr>
              <th></th>
              <th class="l">{t('Competitor')}</th>
              <th title={t('Matches completed')}>M</th>
              <th title={t('Match point index')}>MPI</th>
              <th class="wide" title={t('Victory index')}>VI</th>
              <th class="wide" title={t('Score index')}>SI</th>
              <th class="wide" title={t('Reception index, lowest wins')}>RI</th>
            </tr>
          </thead>
          <tbody>
            {#each pool.standings as s (s.competitor)}
              <tr>
                <td class="rank mono">{s.rank}</td>
                <td class="l">{s.name}<span class="club">{s.club}</span></td>
                <td class="mono">{s.completed}</td>
                <td class="mono strong">{fmt(s.matchPointIndex)}</td>
                <td class="mono wide">{fmt(s.victoryIndex)}</td>
                <td class="mono wide">{fmt(s.scoreIndex)}</td>
                <td class="mono wide">{fmt(s.receptionIndex)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
        </div>
      {/if}

      <ol class="matches">
        {#each pool.matches as m (m.id)}
          <li class={m.status}>
            <span class="n mono">{m.order}</span>
            <span class="red">{name(m.red)}</span>
            <span class="score mono">
              {#if m.status === 'pending'}{t('v')}{:else}{m.state.red.score}–{m.state.blue.score}{/if}
            </span>
            <span class="blue">{name(m.blue)}</span>
          </li>
        {/each}
      </ol>
      {/if}
    </section>
  {/each}
</div>

<style>
  .dim {
    color: var(--ink-dim);
    margin: 0;
  }
  .pools {
    display: grid;
    gap: 1rem;
    min-width: 0;
  }
  .pools > * {
    min-width: 0;
  }
  section {
    padding: 0;
    background: none;
  }
  h3 {
    margin: 0 0 0.5rem;
    font-size: 0.95rem;
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    align-items: baseline;
  }
  .meta {
    font-size: 0.78rem;
    font-weight: 400;
    color: var(--ink-dim);
  }

  .podium {
    display: flex;
    flex-wrap: wrap;
    gap: 0.6rem 1.4rem;
    padding: 0.7rem 0.9rem;
    margin-bottom: 1rem;
    border-radius: var(--radius);
    background: var(--panel-2);
    font-weight: 700;
    overflow-wrap: anywhere;
  }
  .podium strong {
    color: var(--amber-bright);
    margin-right: 0.3rem;
  }
  .gold {
    font-size: 1.1rem;
  }

  .bracket {
    margin-bottom: 1.2rem;
    min-width: 0;
  }
  .bracket h3 {
    font-size: 0.72rem;
    font-weight: 800;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--ink-dim);
  }

  .scroller {
    overflow-x: auto;
    margin-bottom: 0.7rem;
    /* -webkit-overflow-scrolling is on by default in current mobile browsers. */
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.85rem;
    min-width: 17rem;
  }
  th,
  td {
    padding: 0.28rem 0.3rem;
    text-align: right;
    border-bottom: 1px solid var(--line);
  }
  th {
    color: var(--ink-dim);
    font-weight: 500;
    font-size: 0.72rem;
  }
  .l {
    text-align: left;
  }
  .rank {
    color: var(--ink-dim);
  }
  .strong {
    font-weight: 700;
  }
  .club {
    display: block;
    font-size: 0.72rem;
    color: var(--ink-dim);
  }

  .matches {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.12rem;
  }
  .matches li {
    display: grid;
    grid-template-columns: 1.3rem minmax(0, 1fr) auto minmax(0, 1fr);
    gap: 0.45rem;
    align-items: baseline;
    padding: 0.28rem 0.35rem;
    border-radius: 5px;
    font-size: 0.88rem;
  }
  .matches li.running {
    background: var(--panel-2);
    outline: 1px solid var(--ok);
  }
  .matches li.complete,
  .bmatch.done {
    color: var(--ink-dim);
  }
  .n {
    color: var(--ink-dim);
    font-size: 0.75rem;
  }
  .red {
    color: var(--red-bright);
    text-align: right;
    overflow-wrap: anywhere;
  }
  .blue {
    color: var(--blue-bright);
    overflow-wrap: anywhere;
  }
  .matches li.complete .red,
  .matches li.complete .blue,
  .bmatch.done .red,
  .bmatch.done .blue {
    color: var(--ink-dim);
  }
  .score {
    font-weight: 700;
    white-space: nowrap;
  }

  /* The four indices are what an organizer reads; a phone gets the one that orders the
     table and the matches played. The rest is detail nobody squints at on a 360px screen. */
  @media (max-width: 34rem) {
    .wide {
      display: none;
    }
  }
</style>
