<script lang="ts">
  import type { Snapshot } from '../api';
  import { nameLookup } from './lib-display.svelte';

  /** The organizer's screen for a running tournament: matches, status, live standings. */
  let { snapshot }: { snapshot: Snapshot } = $props();

  const name = $derived(nameLookup(snapshot));
  const fmt = (n: number) => (Number.isFinite(n) ? n.toFixed(2) : '0.00');
</script>

{#each snapshot.pools as pool (pool.number)}
  <section>
    <h2>
      Pool {pool.number}
      <span class="meta">Mat {pool.mat} &middot; {pool.complete ? 'complete' : 'in progress'}</span>
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
            {#if m.state.endReason === 'forfeit'}<span class="tag">forfeit</span>{/if}
            {#if m.state.endReason === 'penalty'}<span class="tag">penalty</span>{/if}
          </li>
        {/each}
      </ol>

      <table>
        <thead>
          <tr>
            <th></th>
            <th class="l">Competitor</th>
            <th title="Matches completed">M</th>
            <th title="Match point index">MPI</th>
            <th title="Victory index">VI</th>
            <th title="Score index">SI</th>
            <th title="Reception index, lowest wins">RI</th>
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
  <p class="empty">Draw the pools to get started.</p>
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
    justify-content: space-between;
    align-items: baseline;
  }
  .meta {
    font-size: 0.85rem;
    font-weight: 400;
    color: var(--ink-dim);
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
