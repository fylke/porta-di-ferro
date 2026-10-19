<script lang="ts">
  import type { ForecastView, ItemTimes, ItemView } from '../api';
  import { clockOf, minutesBetween } from '../lib/clock-of-day';
  import { t } from '../lib/i18n.svelte';

  /**
   * The day as the proposal draws it (§10, the mat board): mats down the side, time across,
   * every work item a block where the forecast has it. Behind each block a faint ghost is
   * where the plan had it, so a mat running late shows without a number. Read-only: the
   * cards below are what moves.
   *
   * Pointing at a block, or tabbing to it, says what it is (#142): when it runs against
   * when it was planned, how far it has got, and who it is short of. A block short of
   * staff is orange, and opens the staff.
   */
  let {
    forecast,
    items,
    mats,
    matNames = {},
    name,
    short = new Map(),
    onshort,
  }: {
    forecast: ForecastView;
    items: ItemView[];
    mats: number[];
    matNames?: Record<number, string>;
    name: (id: string) => string;
    /** The roles each item is short of, by item. */
    short?: Map<string, string[]>;
    onshort?: () => void;
  } = $props();

  let shown = $state<{ x: ItemTimes; left: number; top: number } | null>(null);
  function show(e: Event, x: ItemTimes) {
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
    shown = { x, left: Math.min(r.left, window.innerWidth - 260), top: r.bottom + 6 };
  }
  function statusText(id: string): string {
    const it = statusOf.get(id);
    if (!it) return '';
    switch (it.status) {
      case 'running':
        return t('under way, {done} of {total} matches', { done: it.done, total: it.total });
      case 'done':
        return t('done');
      case 'planned':
        return t('not drawn yet');
      default:
        return t('{done} of {total} matches', { done: it.done, total: it.total });
    }
  }
  function isShort(id: string): boolean {
    return short.has(id) && statusOf.get(id)?.status !== 'done';
  }

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
        <span class="mat" title={matNames[mat] ?? ''}>{matNames[mat] || t('Mat {n}', { n: mat })}</span>
        <div class="lane">
          {#each forecast.items.filter((x) => x.mat === mat) as x (x.id)}
            {#if x.plannedStart && x.plannedStart !== x.start}
              <span class="ghost" style="left: {pct(x.plannedStart)}%; width: {width(x.plannedStart, x.plannedEnd)}%"></span>
            {/if}
            {#if isShort(x.id) && onshort}
              <!-- Short of staff: a way to them. -->
              <button
                class="block short {statusOf.get(x.id)?.status ?? ''}"
                style="left: {pct(x.start)}%; width: {width(x.start, x.end)}%"
                aria-label="{name(x.id)}: {clockOf(x.start)}–{clockOf(x.end)}"
                onpointerenter={(e) => show(e, x)}
                onpointerleave={() => (shown = null)}
                onfocus={(e) => show(e, x)}
                onblur={() => (shown = null)}
                onclick={onshort}
              >
                <span class="label">{name(x.id)}</span>
              </button>
            {:else}
              <span
                class="block {statusOf.get(x.id)?.status ?? ''}"
                class:short={isShort(x.id)}
                style="left: {pct(x.start)}%; width: {width(x.start, x.end)}%"
                role="img"
                aria-label="{name(x.id)}: {clockOf(x.start)}–{clockOf(x.end)}"
                onpointerenter={(e) => show(e, x)}
                onpointerleave={() => (shown = null)}
              >
                <span class="label">{name(x.id)}</span>
              </span>
            {/if}
          {/each}
          {#if now >= 0 && now <= 100}<span class="now" style="left: {now}%"></span>{/if}
        </div>
      </div>
    {/each}
  </div>
  {#if shown}
    {@const x = shown.x}
    {@const late = minutesBetween(x.plannedStart, x.start)}
    <div class="tip" role="tooltip" style="left: {shown.left}px; top: {shown.top}px">
      <strong>{name(x.id)}</strong>
      <span>{matNames[x.mat] || t('Mat {n}', { n: x.mat })} · {clockOf(x.start)}–{clockOf(x.end)}</span>
      {#if x.plannedStart && late !== 0}
        <span class="dim">{t('planned {from}–{to}', { from: clockOf(x.plannedStart), to: clockOf(x.plannedEnd) })}{late > 0 ? ` · ${t('+{n} min', { n: late })}` : ''}</span>
      {/if}
      {#if statusText(x.id)}<span class="dim">{statusText(x.id)}</span>{/if}
      {#if isShort(x.id)}
        <span class="warn">{t('Short of {roles}', { roles: short.get(x.id)!.join(', ').toLocaleLowerCase() })}{onshort ? ` · ${t('click to see the staff')}` : ''}</span>
      {/if}
    </div>
  {/if}
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
  button.block {
    padding: 0;
    border: none;
    border-radius: 4px;
    text-align: left;
    font: inherit;
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
  .block:focus-visible {
    outline: 2px solid var(--ink);
    outline-offset: 1px;
  }
  /* Short of staff (#142): orange hatching over whatever the block is, so it never reads
     as amber's own meaning here, which is under way. */
  .block.short {
    background-image: repeating-linear-gradient(135deg, rgba(242, 177, 52, 0.75) 0 4px, transparent 4px 9px);
    box-shadow: inset 0 0 0 2px var(--amber-bright);
    opacity: 1;
    cursor: pointer;
  }
  .tip {
    position: fixed;
    z-index: 50;
    display: grid;
    gap: 0.15rem;
    max-width: 16rem;
    padding: 0.5rem 0.65rem;
    border-radius: 8px;
    background: var(--panel-2);
    border: 1px solid var(--line);
    box-shadow: 0 6px 18px rgba(0, 0, 0, 0.45);
    font-size: 0.8rem;
    line-height: 1.35;
    pointer-events: none;
  }
  .tip .dim {
    color: var(--ink-dim);
  }
  .tip .warn {
    color: var(--amber-bright);
    font-weight: 700;
  }
  .now {
    position: absolute;
    top: -0.1rem;
    bottom: -0.1rem;
    width: 2px;
    background: var(--ok);
  }
</style>
