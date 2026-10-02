<script lang="ts">
  import type { ProgrammeRow } from '../api';
  import { clockOf } from '../lib/clock-of-day';
  import { t } from '../lib/i18n.svelte';

  /**
   * The fencing part of the day, derived from the forecast (phase 4): each discipline's
   * pools, eliminations and final, about when and on which mats. The organizer types the
   * rest of the programme -- gear check, lunch, the prize giving -- and this follows the
   * day as it actually goes, so nobody has to retype "pools at 10:40" when they run late.
   */
  let { rows, names = true }: { rows: ProgrammeRow[]; names?: boolean } = $props();

  function stage(r: ProgrammeRow): string {
    switch (r.stage) {
      case 'pools':
        return t('Pools');
      case 'eliminations':
        return t('Eliminations');
      case 'final':
        return t('Final');
    }
  }
  function mats(r: ProgrammeRow): string {
    if (r.mats.length === 1) return t('mat {n}', { n: r.mats[0] });
    return t('mats {list}', { list: r.mats.join(', ') });
  }
</script>

{#if rows.length > 0}
  <ol class="fencing">
    {#each rows as r (`${r.discipline}/${r.stage}`)}
      <li class:done={r.done}>
        <span class="when mono">{clockOf(r.start)}</span>
        <span class="what">
          {#if names}<span class="disc">{r.name}</span>{/if}
          {stage(r)}
          <span class="dim">
            &middot; {r.done ? t('done') : t('about {from}–{to}', { from: clockOf(r.start), to: clockOf(r.end) })} &middot; {mats(r)}
          </span>
        </span>
      </li>
    {/each}
  </ol>
{/if}

<style>
  .fencing {
    list-style: none;
    margin: 0.6rem 0 0;
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
