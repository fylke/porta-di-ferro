<script lang="ts">
  import { api, type MatchView, type Snapshot } from '../api';
  import { nameLookup } from './lib-display.svelte';
  import MatchEditor from './MatchEditor.svelte';
  import { t } from '../lib/i18n.svelte';

  /**
   * The organizer's screen for a running tournament: matches, status, live standings --
   * and the override of the mat assignment. Pools arrive in run order, by mat and then
   * by queue, so the list reads the way the hall runs.
   */
  let { snapshot, onchange }: { snapshot: Snapshot; onchange: () => void } = $props();

  const name = $derived(nameLookup(snapshot));
  const fmt = (n: number) => (Number.isFinite(n) ? n.toFixed(2) : '0.00');
  const mats = $derived(Array.from({ length: snapshot.tournament.mats }, (_, i) => i + 1));

  // The match whose log is open in the editor (design §7 item 1).
  let editing = $state<MatchView | null>(null);

  let error = $state('');
  async function override(run: () => Promise<unknown>) {
    error = '';
    try {
      await run();
      onchange();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }
</script>

{#if error}<p class="err">{error}</p>{/if}

{#each snapshot.pools as pool, i (pool.number)}
  {#if i === 0 || snapshot.pools[i - 1].mat !== pool.mat}
    <h3 class="mat-head">{t('Mat {n}', { n: pool.mat })}</h3>
  {/if}
  <section>
    <h2>
      {t('Pool {n}', { n: pool.number })}
      <span class="meta">{pool.complete ? t('complete') : t('in progress')}</span>
      {#if pool.overridden}
        <!-- Visible as an override, so nobody has to wonder why mat 2 is running pool 3. -->
        <span class="tag override" title={t('Moved by the organizer from where the draw put it')}>{t('moved')}</span>
      {/if}
      <span class="controls">
        <label>
          {t('Mat')}
          <select value={pool.mat} onchange={(e) => void override(() => api.movePool(pool.number, Number(e.currentTarget.value)))}>
            {#each mats as m (m)}<option value={m}>{m}</option>{/each}
          </select>
        </label>
        <button title={t('Run this pool earlier on its mat')} aria-label={t('Move pool {n} up', { n: pool.number })} onclick={() => void override(() => api.reorderPool(pool.number, 'up'))}>&uarr;</button>
        <button title={t('Run this pool later on its mat')} aria-label={t('Move pool {n} down', { n: pool.number })} onclick={() => void override(() => api.reorderPool(pool.number, 'down'))}>&darr;</button>
      </span>
    </h2>

    <div class="split">
      <ol class="matches">
        {#each pool.matches as m (m.id)}
          <li class={m.status}>
            <span class="n mono">{m.order}</span>
            <span class="red">{name(m.red)}</span>
            <span class="score mono">
              {#if m.status === 'pending'}v{:else}{m.state.red.score}–{m.state.blue.score}{/if}
            </span>
            <span class="blue">{name(m.blue)}</span>
            {#if m.state.endReason === 'forfeit'}<span class="tag">{t('forfeit')}</span>{/if}
            {#if m.state.endReason === 'penalty'}<span class="tag">{t('penalty')}</span>{/if}
            <button class="edit" title={t("Edit this match's log")} aria-label={t('Edit the log of match {n}', { n: m.order })} onclick={() => (editing = m)}>&#9998;</button>
          </li>
        {/each}
      </ol>

      <table>
        <thead>
          <tr>
            <th></th>
            <th class="l">{t('Competitor')}</th>
            <th title={t('Matches completed')}>M</th>
            <th title={t('Match point index')}>MPI</th>
            <th title={t('Victory index')}>VI</th>
            <th title={t('Score index')}>SI</th>
            <th title={t('Reception index, lowest wins')}>RI</th>
          </tr>
        </thead>
        <tbody>
          {#each pool.standings as s (s.competitor)}
            <tr>
              <td class="rank mono">{s.rank}</td>
              <td class="l">{s.name}<span class="club">{s.club}</span></td>
              <td class="mono">{s.completed}</td>
              <td class="mono strong">{fmt(s.matchPointIndex)}</td>
              <td class="mono">{fmt(s.victoryIndex)}</td>
              <td class="mono">{fmt(s.scoreIndex)}</td>
              <td class="mono">{fmt(s.receptionIndex)}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  </section>
{/each}

{#if snapshot.pools.length === 0}
  <p class="empty">{t('Draw the pools to get started.')}</p>
{/if}

{#if editing}
  <MatchEditor
    match={editing}
    names={{ red: name(editing.red), blue: name(editing.blue) }}
    onClose={() => (editing = null)}
    onSaved={onchange}
  />
{/if}

<style>
  section {
    background: var(--panel);
    border-radius: var(--radius);
    padding: 1.1rem 1.25rem;
    margin-bottom: 1rem;
  }
  h2 {
    margin: 0 0 0.8rem;
    font-size: 1.1rem;
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem 0.8rem;
    align-items: baseline;
  }
  .meta {
    font-size: 0.85rem;
    font-weight: 400;
    color: var(--ink-dim);
  }
  .mat-head {
    margin: 1.4rem 0 0.5rem;
    font-size: 0.8rem;
    font-weight: 800;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--ink-dim);
  }
  .tag.override {
    background: var(--amber);
    color: #1a1200;
  }
  /* The override lives on the pool's own header, small and to the right: it is an escape
     hatch for the day, not a setting anyone should be reaching for. */
  .controls {
    margin-left: auto;
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.8rem;
    font-weight: 400;
    color: var(--ink-dim);
  }
  .controls label {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
  }
  .controls select {
    padding: 0.25rem 0.4rem;
    font-size: 0.85rem;
  }
  .controls button {
    padding: 0.25rem 0.55rem;
    font-size: 0.9rem;
    background: var(--panel-2);
    border: 1px solid var(--line);
    color: var(--ink);
  }
  .err {
    color: var(--amber-bright);
    margin: 0 0 0.8rem;
  }
  /* Present on every match, quiet until hovered: an error is often noticed after the
     match has left the mat, and the fix should be one click from the table. */
  .edit {
    margin-left: auto;
    padding: 0.1rem 0.45rem;
    font-size: 0.85rem;
    background: none;
    border: 1px solid transparent;
    color: var(--ink-dim);
    opacity: 0.6;
  }
  .edit:hover {
    opacity: 1;
    border-color: var(--line);
  }
  .split {
    display: grid;
    grid-template-columns: minmax(16rem, 1fr) minmax(18rem, 1.1fr);
    gap: 1.25rem;
    align-items: start;
  }
  @media (max-width: 780px) {
    .split {
      grid-template-columns: 1fr;
    }
  }
  ol {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.15rem;
  }
  li {
    display: grid;
    grid-template-columns: 1.4rem 1fr auto 1fr auto;
    gap: 0.5rem;
    align-items: baseline;
    padding: 0.3rem 0.4rem;
    border-radius: 6px;
    font-size: 0.9rem;
  }
  li.running {
    background: var(--panel-2);
    outline: 2px solid var(--ok);
  }
  li.complete {
    color: var(--ink-dim);
  }
  .n {
    color: var(--ink-dim);
    font-size: 0.75rem;
  }
  .red {
    color: var(--red-bright);
    text-align: right;
  }
  .blue {
    color: var(--blue-bright);
  }
  li.complete .red,
  li.complete .blue {
    color: var(--ink-dim);
  }
  .score {
    font-weight: 700;
    min-width: 2.8rem;
    text-align: center;
  }
  .tag {
    font-size: 0.7rem;
    color: var(--amber-bright);
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.85rem;
  }
  th,
  td {
    padding: 0.3rem 0.35rem;
    text-align: right;
    border-bottom: 1px solid var(--line);
  }
  th {
    color: var(--ink-dim);
    font-weight: 500;
    font-size: 0.75rem;
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
  .empty {
    color: var(--ink-dim);
  }
</style>
