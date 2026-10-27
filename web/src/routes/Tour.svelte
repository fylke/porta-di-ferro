<script lang="ts">
  import { tick } from 'svelte';
  import { tour, steps } from '../lib/tour.svelte';
  import { navigate } from '../router.svelte';
  import { t } from '../lib/i18n.svelte';

  /**
   * The guided tour's card, over the page it has got to (#149), with the panel it is about
   * lit and the rest of the page dimmed. The panel is found by its data-tour mark once the
   * page has drawn it; a page that draws it late -- after a fetch -- is given a moment, and
   * a step whose panel is not there stands on its own.
   */
  let box = $state<{ top: number; left: number; width: number; height: number } | null>(null);
  let target: Element | null = null;

  function measure() {
    if (!target || !target.isConnected) {
      box = null;
      return;
    }
    const r = target.getBoundingClientRect();
    box = { top: r.top - 6, left: r.left - 6, width: r.width + 12, height: r.height + 12 };
  }

  async function find(mark: string | undefined) {
    target = null;
    box = null;
    if (!mark) return;
    await tick();
    for (let i = 0; i < 20 && tour.active && !target; i++) {
      target = document.querySelector(`[data-tour="${mark}"]`);
      if (!target) await new Promise((r) => setTimeout(r, 100));
    }
    if (!target) return;
    target.scrollIntoView({ block: 'center' });
    measure();
  }

  $effect(() => {
    void find(tour.step.target);
  });
  // Pages draw in stages -- a panel fills in after its fetch and pushes the next one down
  // -- so the light follows its panel while the tour is open.
  $effect(() => {
    const every = setInterval(measure, 250);
    return () => clearInterval(every);
  });

  function plan() {
    tour.stop();
    navigate('/plan?step=1');
  }
</script>

<svelte:window onscroll={measure} onresize={measure} onkeydown={(e) => e.key === 'Escape' && tour.stop()} />

{#if box}
  <div class="spot" style="top:{box.top}px;left:{box.left}px;width:{box.width}px;height:{box.height}px"></div>
{:else}
  <div class="dim"></div>
{/if}

<div class="card admin" role="dialog" aria-modal="false" aria-labelledby="tour-title">
  <p class="count">{t('Step {n} of {total}', { n: tour.index + 1, total: steps.length })}</p>
  <h2 id="tour-title">{tour.step.title()}</h2>
  <p class="text">{tour.step.text()}</p>
  <div class="actions">
    <button class="secondary" onclick={() => tour.stop()}>{t('End the tour')}</button>
    <span class="grow"></span>
    {#if tour.index > 0}<button class="secondary" onclick={() => tour.back()}>{t('Back')}</button>{/if}
    {#if tour.last}
      <button onclick={plan}>{t('Plan an event')}</button>
    {:else}
      <button onclick={() => tour.next()}>{t('Next')}</button>
    {/if}
  </div>
</div>

<style>
  /* The page dimmed around the panel: one box whose shadow is the dimming. */
  .spot {
    position: fixed;
    z-index: 80;
    border-radius: 12px;
    border: 2px solid var(--amber-bright);
    box-shadow: 0 0 0 200vmax rgba(6, 8, 12, 0.7);
    pointer-events: none;
    transition:
      top 0.15s,
      left 0.15s,
      width 0.15s,
      height 0.15s;
  }
  .dim {
    position: fixed;
    inset: 0;
    z-index: 80;
    background: rgba(6, 8, 12, 0.7);
    pointer-events: none;
  }
  .card {
    position: fixed;
    z-index: 81;
    left: 16px;
    right: 16px;
    bottom: 16px;
    max-width: 26rem;
    margin-left: auto;
    background: var(--panel);
    border: 1px solid var(--amber-bright);
    border-radius: 14px;
    padding: 1rem 1.1rem;
    box-shadow: 0 10px 30px rgba(0, 0, 0, 0.5);
  }
  .count {
    margin: 0;
    font-size: 0.75rem;
    font-weight: 800;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--amber-bright);
  }
  h2 {
    margin: 0.3rem 0 0.4rem;
    font-size: 1.15rem;
  }
  .text {
    margin: 0 0 0.9rem;
    line-height: 1.5;
    color: var(--ink-dim);
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
  }
  .grow {
    flex: 1;
  }
</style>
