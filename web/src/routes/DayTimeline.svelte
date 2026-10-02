<script lang="ts">
  import type { ForecastView, ItemView } from '../api';
  import { clockOf } from '../lib/clock-of-day';
  import { t } from '../lib/i18n.svelte';

  /**
   * The day as the proposal draws it (§10, the mat board): mats down the side, time across,
   * every work item a block where the forecast has it. Behind each block a faint ghost is
   * where the plan had it, so a mat running late shows without a number. Read-only: the
   * cards below are what moves.
   */
  let {
    forecast,
    items,
    mats,
    name,
  }: {
    forecast: ForecastView;
    items: ItemView[];
    mats: number[];
    name: (id: string) => string;
  } = $props();

  const statusOf = $derived(new Map(items.map((it) => [it.id, it])));

  const range = $derived.by(() => {
    let from = Infinity;
    let to = -Infinity;
    let first = '';
    for (const x of forecast.items) {
      for (const s of [x.start, x.plannedStart]) {
        const v = Date.parse(s);
        if (!Number.isNaN(v) && v < from) {
          from = v;
          first = s;
        }
      }
      for (const s of [x.end, x.plannedEnd]) {
        const v = Date.parse(s);
        if (!Number.isNaN(v) && v > to) to = v;
      }
    }
    return Number.isFinite(from) && to > from ? { from, to, first } : null;
  });

  function pct(stamp: string): number {
    if (!range) return 0;
    return ((Date.parse(stamp) - range.from) / (range.to - range.from)) * 100;
  }
  function width(a: string, b: string): number {
    return Math.max(0.6, pct(b) - pct(a));
  }

  /** Whole hours across the day, labelled in the hall's clock. */
  const ticks = $derived.by(() => {
    if (!range) return [];
    const [h, m] = clockOf(range.first).split(':').map(Number);
    const out: { at: number; label: string }[] = [];
    let hour = h + 1;
    for (let at = range.from + (60 - m) * 60000; at < range.to; at += 3600000, hour++) {
      out.push({ at: ((at - range.from) / (range.to - range.from)) * 100, label: `${String(hour % 24).padStart(2, '0')}:00` });
    }
    return out;
  });

  const now = $derived(range && forecast.live ? ((Date.now() - range.from) / (range.to - range.from)) * 100 : -1);
</script>

{#if range}
  <div class="timeline" aria-label={t('The day, mat by mat')}>
    <div class="ticks">
      {#each ticks as tick (tick.label)}
        <span class="tick" style="left: {tick.at}%">{tick.label}</span>
      {/each}
    </div>
    {#each mats as mat (mat)}
      <div class="row">
        <span class="mat">{t('Mat {n}', { n: mat })}</span>
        <div class="lane">
          {#each forecast.items.filter((x) => x.mat === mat) as x (x.id)}
            {#if x.plannedStart && x.plannedStart !== x.start}
              <span class="ghost" style="left: {pct(x.plannedStart)}%; width: {width(x.plannedStart, x.plannedEnd)}%"></span>
            {/if}
            <span
              class="block {statusOf.get(x.id)?.status ?? ''}"
              style="left: {pct(x.start)}%; width: {width(x.start, x.end)}%"
              title="{name(x.id)}: {clockOf(x.start)}–{clockOf(x.end)}"
            >
              <span class="label">{name(x.id)}</span>
            </span>
          {/each}
          {#if now >= 0 && now <= 100}<span class="now" style="left: {now}%"></span>{/if}
        </div>
      </div>
    {/each}
  </div>
{/if}

<style>
  .timeline {
    margin: 0.4rem 0 0.9rem;
    overflow-x: auto;
  }
  .ticks,
  .lane {
    position: relative;
    min-width: 36rem;
  }
  .ticks {
    height: 1.1rem;
    margin-left: 3.6rem;
  }
  .tick {
    position: absolute;
    transform: translateX(-50%);
    font-size: 0.68rem;
    color: var(--ink-dim);
    font-variant-numeric: tabular-nums;
  }
  .row {
    display: grid;
    grid-template-columns: 3.6rem minmax(0, 1fr);
    align-items: center;
    margin-bottom: 0.25rem;
  }
  .mat {
    font-size: 0.72rem;
    font-weight: 800;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--ink-dim);
  }
  .lane {
    height: 1.7rem;
    background: var(--panel-2);
    border-radius: 6px;
  }
  .block,
  .ghost {
    position: absolute;
    top: 0.2rem;
    bottom: 0.2rem;
    border-radius: 4px;
  }
  .ghost {
    border: 1px dashed var(--ink-dim);
    opacity: 0.5;
  }
  .block {
    background: var(--blue-bright);
    opacity: 0.85;
    overflow: hidden;
  }
  .block.running {
    background: var(--amber-bright);
  }
  .block.done {
    opacity: 0.35;
  }
  .block.planned {
    background: transparent;
    border: 1px dashed var(--blue-bright);
  }
  .label {
    display: block;
    padding: 0 0.3rem;
    font-size: 0.68rem;
    line-height: 1.3rem;
    color: #0d0f14;
    font-weight: 700;
    white-space: nowrap;
  }
  .block.planned .label {
    color: var(--ink);
  }
  .now {
    position: absolute;
    top: -0.1rem;
    bottom: -0.1rem;
    width: 2px;
    background: var(--ok);
  }
</style>
