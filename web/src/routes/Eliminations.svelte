<script lang="ts">
  import { api, type Snapshot } from '../api';
  import { nameLookup, roundLabel } from './lib-display.svelte';

  /**
   * The cut and the bracket (design §7 item 3). Drawn once every pool match is in, from
   * the overall ranking -- everyone across the pools, by the same chain the pool tables
   * use -- and then filled round by round from results, so nothing here is entered by
   * hand and a corrected quarter-final corrects the semi-final on its own.
   */
  let { snapshot, onchange }: { snapshot: Snapshot; onchange: () => void } = $props();

  const name = $derived(nameLookup(snapshot));
  const bracket = $derived(snapshot.bracket ?? null);
  const fmt = (n: number) => (Number.isFinite(n) ? n.toFixed(2) : '0.00');
  const toGo = $derived(
    snapshot.pools.flatMap((p) => p.matches).filter((m) => m.status !== 'complete').length,
  );
  const cut = $derived(snapshot.overall.length >= 8 ? 8 : snapshot.overall.length >= 4 ? 4 : 2);
  const seeds = $derived(snapshot.overall.slice(0, cut));
  const scored = $derived(!!bracket && bracket.matches.some((m) => m.status !== 'pending'));

  let error = $state('');
  let busy = $state(false);
  async function draw() {
    error = '';
    busy = true;
    try {
      await api.drawBracket();
      onchange();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      busy = false;
    }
  }

  const rounds = $derived.by(() => {
    if (!bracket) return [];
    const order = ['quarter', 'semi', 'bronze', 'final'] as const;
    return order
      .map((r) => ({ round: r, matches: bracket.matches.filter((m) => m.round === r) }))
      .filter((g) => g.matches.length > 0);
  });
  const heading = (round: string) =>
    round === 'quarter' ? 'Quarter-finals' : round === 'semi' ? 'Semi-finals' : round === 'bronze' ? 'Bronze match' : 'Final';
</script>

{#if snapshot.pools.length > 0}
  <section>
    <h2>Eliminations <span class="meta">top {cut}, single elimination, sudden death</span></h2>

    {#if !snapshot.poolsComplete && !bracket}
      <p class="dim">
        Drawn from finished pools. {toGo} pool match{toGo === 1 ? '' : 'es'} still to score.
      </p>
    {:else}
      {#if !bracket || !scored}
        <div class="seeds">
          <h3>Seeding &middot; overall ranking across the pools</h3>
          <table>
            <thead>
              <tr>
                <th></th>
                <th class="l">Competitor</th>
                <th title="Match point index">MPI</th>
                <th title="Victory index">VI</th>
                <th title="Score index">SI</th>
                <th title="Reception index, lowest wins">RI</th>
              </tr>
            </thead>
            <tbody>
              {#each seeds as s (s.competitor)}
                <tr>
                  <td class="rank mono">{s.rank}</td>
                  <td class="l">{s.name}<span class="club">{s.club}</span></td>
                  <td class="mono strong">{fmt(s.matchPointIndex)}</td>
                  <td class="mono">{fmt(s.victoryIndex)}</td>
                  <td class="mono">{fmt(s.scoreIndex)}</td>
                  <td class="mono">{fmt(s.receptionIndex)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}

      <button class="draw" disabled={busy || !snapshot.poolsComplete} onclick={draw}>
        {bracket ? 'Draw the eliminations again' : 'Draw the eliminations'}
      </button>
      {#if scored}
        <p class="warn">
          Bracket matches have been scored. Drawing again replaces the bracket and the
          results stop lining up with it.
        </p>
      {/if}
      {#if error}<p class="err">{error}</p>{/if}
    {/if}

    {#if bracket}
      <div class="rounds">
        {#each rounds as g (g.round)}
          <div class="round">
            <h3>{heading(g.round)}</h3>
            <ol>
              {#each g.matches as m (m.id)}
                <li class={m.status}>
                  <span class="n">{roundLabel(m)} &middot; mat {m.mat}</span>
                  <span class="red">{m.red ? name(m.red) : '—'}</span>
                  <span class="score mono">
                    {#if m.status === 'pending'}v{:else}{m.state.red.score}–{m.state.blue.score}{/if}
                  </span>
                  <span class="blue">{m.blue ? name(m.blue) : '—'}</span>
                </li>
              {/each}
            </ol>
          </div>
        {/each}
      </div>
      {#if bracket.podium.first}
        <p class="podium">
          <span><strong>1.</strong> {name(bracket.podium.first)}</span>
          <span><strong>2.</strong> {name(bracket.podium.second)}</span>
          {#if bracket.podium.third}<span><strong>3.</strong> {name(bracket.podium.third)}</span>{/if}
        </p>
      {/if}
    {/if}
  </section>
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
  h3 {
    margin: 0.6rem 0 0.4rem;
    font-size: 0.75rem;
    font-weight: 800;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--ink-dim);
  }
  .meta {
    font-size: 0.85rem;
    font-weight: 400;
    color: var(--ink-dim);
  }
  .dim {
    color: var(--ink-dim);
    margin: 0;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.88rem;
    margin-bottom: 0.8rem;
  }
  th,
  td {
    padding: 0.3rem 0.45rem;
    text-align: right;
    border-bottom: 1px solid var(--line);
  }
  th {
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--ink-dim);
    font-weight: 600;
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
    font-size: 0.75rem;
    color: var(--ink-dim);
  }
  .draw {
    width: 100%;
    padding: 0.85rem;
    font-weight: 700;
    background: var(--panel-2);
    border: 2px solid var(--line);
  }
  .draw:disabled {
    opacity: 0.5;
  }
  .warn,
  .err {
    margin: 0.8rem 0 0;
    font-size: 0.88rem;
    line-height: 1.5;
    color: var(--amber-bright);
  }
  .rounds {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(16rem, 1fr));
    gap: 1rem;
    margin-top: 0.8rem;
  }
  ol {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.35rem;
  }
  li {
    display: grid;
    grid-template-columns: 1fr auto 1fr;
    grid-template-rows: auto auto;
    gap: 0.1rem 0.5rem;
    align-items: baseline;
    padding: 0.4rem 0.5rem;
    border-radius: 6px;
    background: var(--panel-2);
    font-size: 0.92rem;
  }
  li .n {
    grid-column: 1 / -1;
    font-size: 0.7rem;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--ink-dim);
  }
  .red {
    color: var(--red-bright);
    font-weight: 700;
    text-align: right;
  }
  .blue {
    color: var(--blue-bright);
    font-weight: 700;
  }
  .score {
    color: var(--ink-dim);
  }
  li.complete .red,
  li.complete .blue {
    color: var(--ink);
  }
  li.running .score {
    color: var(--amber-bright);
  }
  .podium {
    margin: 0.9rem 0 0;
    display: flex;
    flex-wrap: wrap;
    gap: 1.2rem;
    font-size: 1.05rem;
    font-weight: 700;
  }
  .podium strong {
    color: var(--amber-bright);
  }
</style>
