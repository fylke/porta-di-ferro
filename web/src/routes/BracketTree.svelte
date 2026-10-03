<script lang="ts">
  import type { MatchView } from '../api';
  import { t } from '../lib/i18n.svelte';
  import { roundLabel } from './lib-display.svelte';

  /**
   * The eliminations as a bracket is drawn on paper (#112): the rounds side by side, each
   * match between the two that feed it, joined by lines, so who meets whom next is seen
   * rather than worked out. The bronze match sits under the final.
   *
   * Read-only. A page that edits a match's log passes onedit.
   */
  let {
    matches,
    name,
    onedit,
  }: {
    matches: MatchView[];
    name: (id: string) => string;
    onedit?: (m: MatchView) => void;
  } = $props();

  const bySlot = (a: MatchView, b: MatchView) => (a.slot ?? 0) - (b.slot ?? 0);
  const columns = $derived(
    (['quarter', 'semi', 'final'] as const)
      .map((round) => ({ round, matches: matches.filter((m) => m.round === round).sort(bySlot) }))
      .filter((c) => c.matches.length > 0),
  );
  const bronze = $derived(matches.find((m) => m.round === 'bronze') ?? null);

  function title(round: string): string {
    return round === 'quarter' ? t('Quarter-finals') : round === 'semi' ? t('Semi-finals') : t('Final');
  }
  function won(m: MatchView, side: 'red' | 'blue'): boolean {
    return m.status === 'complete' && m.state.winner === side;
  }
  /** Who is in a slot, or -- not decided yet -- which match decides it. */
  function who(m: MatchView, side: 'red' | 'blue'): { text: string; known: boolean } {
    const id = side === 'red' ? m.red : m.blue;
    if (id) return { text: name(id), known: true };
    const feed = side === 'red' ? m.feedRed : m.feedBlue;
    const [how, from] = (feed ?? '').split(':');
    const feeder = matches.find((x) => x.id === from);
    if (!feeder) return { text: '—', known: false };
    const label = roundLabel(feeder).toLocaleLowerCase();
    return { text: how === 'loser' ? t('Loser of {match}', { match: label }) : t('Winner of {match}', { match: label }), known: false };
  }
  function score(m: MatchView, side: 'red' | 'blue'): string {
    return m.status === 'pending' ? '' : String(m.state[side].score);
  }
</script>

{#snippet box(m: MatchView)}
  <div class="match {m.status}">
    <div class="side red" class:won={won(m, 'red')} class:lost={won(m, 'blue')}>
      <span class="who" class:tbd={!who(m, 'red').known}>{who(m, 'red').text}</span>
      <span class="pts mono">{score(m, 'red')}</span>
    </div>
    <div class="side blue" class:won={won(m, 'blue')} class:lost={won(m, 'red')}>
      <span class="who" class:tbd={!who(m, 'blue').known}>{who(m, 'blue').text}</span>
      <span class="pts mono">{score(m, 'blue')}</span>
    </div>
    <div class="foot">
      <span>{t('mat {n}', { n: m.mat })}{#if m.status === 'running'} &middot; {t('under way')}{/if}</span>
      {#if onedit && m.red && m.blue}
        <button class="edit" title={t("Edit this match's log")} aria-label={t('Edit the log of {match}', { match: roundLabel(m) })} onclick={() => onedit?.(m)}>&#9998;</button>
      {/if}
    </div>
  </div>
{/snippet}

<div class="scroller">
  <div class="tree" style="--cols: {columns.length}">
    {#each columns as col, ci (col.round)}
      <div class="col">
        <h4>{title(col.round)}</h4>
        <div class="stack">
          {#each col.matches as m, i (m.id)}
            <div
              class="slot"
              class:top={i % 2 === 0}
              class:bottom={i % 2 === 1}
              class:out={ci < columns.length - 1}
              class:in={ci > 0}
            >
              {@render box(m)}
            </div>
          {/each}
        </div>
      </div>
    {/each}
  </div>
  {#if bronze}
    <div class="bronze">
      <h4>{t('Bronze match')}</h4>
      {@render box(bronze)}
    </div>
  {/if}
</div>

<style>
  .scroller {
    overflow-x: auto;
    padding-bottom: 0.3rem;
  }
  .tree {
    display: grid;
    grid-template-columns: repeat(var(--cols), minmax(11rem, 14rem));
    column-gap: 1.6rem;
    width: max-content;
  }
  .col {
    display: flex;
    flex-direction: column;
  }
  h4 {
    margin: 0 0 0.4rem;
    font-size: 0.72rem;
    font-weight: 800;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--ink-dim);
  }
  /* Every column as tall as the first, its matches spread evenly down it: a match lands
     half-way between the two that feed it. */
  .stack {
    flex: 1;
    display: flex;
    flex-direction: column;
    justify-content: space-around;
  }
  .slot {
    position: relative;
    flex: 1;
    display: flex;
    align-items: center;
    padding: 0.3rem 0;
  }
  /* The lines: out of a match to the right, then up or down to meet its pair half-way,
     and into the next match from the left. */
  .slot.out.top::after,
  .slot.out.bottom::after {
    content: '';
    position: absolute;
    right: -0.8rem;
    width: 0.8rem;
    height: 50%;
    border-right: 2px solid var(--line);
  }
  .slot.out.top::after {
    top: 50%;
    border-top: 2px solid var(--line);
  }
  .slot.out.bottom::after {
    bottom: 50%;
    border-bottom: 2px solid var(--line);
  }
  .slot.in::before {
    content: '';
    position: absolute;
    left: -0.8rem;
    width: 0.8rem;
    top: 50%;
    border-top: 2px solid var(--line);
  }
  .match {
    width: 100%;
    border: 1px solid var(--line);
    border-radius: 6px;
    background: var(--panel-2);
    overflow: hidden;
    font-size: 0.88rem;
  }
  .match.running {
    border-color: var(--amber-bright);
  }
  .side {
    display: flex;
    justify-content: space-between;
    gap: 0.5rem;
    padding: 0.3rem 0.5rem;
    border-left: 3px solid transparent;
  }
  .side.red {
    border-left-color: var(--red-bright);
    border-bottom: 1px solid var(--line);
  }
  .side.blue {
    border-left-color: var(--blue-bright);
  }
  .who {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .who.tbd {
    color: var(--ink-dim);
    font-style: italic;
    font-size: 0.82rem;
  }
  .side.won {
    font-weight: 800;
  }
  .side.lost {
    color: var(--ink-dim);
  }
  .pts {
    font-variant-numeric: tabular-nums;
  }
  .foot {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.1rem 0.5rem 0.2rem;
    font-size: 0.7rem;
    color: var(--ink-dim);
  }
  .edit {
    padding: 0 0.3rem;
    font-size: 0.8rem;
    background: none;
    border: none;
    color: var(--ink-dim);
  }
  .edit:hover {
    color: var(--ink);
  }
  .bronze {
    margin-top: 1rem;
    width: min(14rem, 100%);
  }
</style>
