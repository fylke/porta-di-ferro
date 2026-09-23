<script lang="ts">
  import type { ScheduleItem } from '../api';
  import { t } from '../lib/i18n.svelte';

  /**
   * The day's agenda (issue #98): gear check, the pools, lunch, the eliminations.
   *
   * Shared by the landing page and the info sheet, because they show the same list and a
   * spectator comparing the printed sheet with their phone should not find two different
   * ones.
   *
   * The times are the organizer's text, not parsed and not sorted. A schedule that says
   * "after the pools" is a real schedule, and one that reorders itself under an organizer
   * who typed the rows in the order they happen would be worse than useless.
   */
  let { items, compact = false }: { items: ScheduleItem[]; compact?: boolean } = $props();
</script>

{#if items.length === 0}
  <p class="dim">{t('The programme has not been put up yet.')}</p>
{:else}
  <ol class:compact>
    {#each items as item, i (i)}
      <li class={item.kind ?? ''}>
        <span class="at mono">{item.at ?? ''}</span>
        <span class="label">{item.label}</span>
      </li>
    {/each}
  </ol>
{/if}

<style>
  ol {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.1rem;
  }
  li {
    display: grid;
    grid-template-columns: minmax(3.2rem, auto) 1fr;
    gap: 0.7rem;
    align-items: baseline;
    padding: 0.35rem 0;
    line-height: 1.4;
  }
  .at {
    color: var(--ink-dim);
    font-size: 0.85em;
    white-space: nowrap;
  }
  .label {
    overflow-wrap: anywhere;
  }
  /* A discipline is the thing people came for; a break is not. Weight rather than colour,
     because colour means identity everywhere else in this application. */
  li.discipline .label {
    font-weight: 700;
  }
  li.break .label,
  li.break .at {
    color: var(--ink-dim);
  }
  .compact li {
    padding: 0.2rem 0;
    font-size: 0.92rem;
  }
  .dim {
    color: var(--ink-dim);
    margin: 0;
  }
</style>
