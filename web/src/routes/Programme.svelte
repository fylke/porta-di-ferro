<script lang="ts">
  import type { ProgrammeRow } from '../api';
  import { clockOf } from '../lib/clock-of-day';
  import { t } from '../lib/i18n.svelte';

  /**
   * The day's programme, generated from the plan (#149): each discipline's gear check,
   * staff briefing, pools, eliminations and final, and the hall's breaks, about when and on
   * which mats. Nobody types it: it follows the plan before the day, and the day as it
   * actually goes, so nobody has to retype "pools at 10:40" when they run late.
   *
   * Shared by the landing pages, the info sheet and the planning step, so the printed sheet
   * and a phone never show two different days.
   */
  let { rows, names = true, empty = '' }: { rows: ProgrammeRow[]; names?: boolean; empty?: string } = $props();

  function stage(r: ProgrammeRow): string {
    switch (r.stage) {
      case 'gear-check':
        return t('Gear check');
      case 'briefing':
        return t('Staff briefing');
      case 'pools':
        return t('Pools');
      case 'eliminations':
        return t('Eliminations');
      case 'final':
        return t('Final');
      case 'break':
        return r.name === 'Lunch' ? t('Lunch') : r.name;
    }
  }
  function mats(r: ProgrammeRow): string {
    if (r.mats.length === 0) return '';
    if (r.mats.length === 1) return ` · ${t('mat {n}', { n: r.mats[0] })}`;
    return ` · ${t('mats {list}', { list: r.mats.join(', ') })}`;
  }
</script>

{#if rows.length === 0}
  <p class="dim">{empty || t('The programme has not been put up yet.')}</p>
{:else}
  <ol class="programme">
    {#each rows as r, i (i)}
      <li class:done={r.done} class:break={r.stage === 'break'}>
        <span class="when mono">{clockOf(r.start)}</span>
        <span class="what">
          {#if names && r.stage !== 'break'}<span class="disc">{r.name}</span>{/if}
          {stage(r)}
          <span class="dim">
            &middot; {r.done ? t('done') : t('about {from}–{to}', { from: clockOf(r.start), to: clockOf(r.end) })}{mats(r)}
          </span>
        </span>
      </li>
    {/each}
  </ol>
{/if}

<style>
  .programme {
    list-style: none;
    margin: 0.4rem 0 0;
    padding: 0;
    display: grid;
    gap: 0.35rem;
  }
  li {
    display: grid;
    grid-template-columns: 3.2rem minmax(0, 1fr);
    gap: 0.6rem;
    align-items: baseline;
    font-size: 0.92rem;
  }
  li.done {
    opacity: 0.55;
  }
  li.break .what {
    font-style: italic;
  }
  .when {
    color: var(--ink-dim);
    font-variant-numeric: tabular-nums;
  }
  .what {
    overflow-wrap: anywhere;
  }
  .disc {
    font-weight: 700;
    margin-right: 0.3rem;
  }
  .dim {
    color: var(--ink-dim);
    font-size: 0.85rem;
  }
</style>
