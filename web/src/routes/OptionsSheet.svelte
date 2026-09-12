<script lang="ts">
  import { COLOURS, type Options, type Side } from '../lib/match';
  import { t } from '../lib/i18n.svelte';

  /**
   * Colours and sides, reached from the overflow menu (design §7 item 6).
   *
   * Colours come first and sides last, on purpose. Swapping sides risks a screen that
   * disagrees with the corners of the mat, so a different colour is usually the better
   * answer to the same problem, and the layout makes it the easier one to reach. The
   * side swap on this screen is this device's alone; the display swap is the match's.
   */
  let {
    options,
    names,
    swapHere,
    onColour,
    onSwapHere,
    onSwapDisplay,
    onClose,
  }: {
    options: Options;
    names: { red: string; blue: string };
    swapHere: boolean;
    onColour: (side: Side, colour: string) => void;
    onSwapHere: (swap: boolean) => void;
    onSwapDisplay: (swap: boolean) => void;
    onClose: () => void;
  } = $props();

  const otherOf = (side: Side): Side => (side === 'red' ? 'blue' : 'red');
</script>

<div class="scrim" role="dialog" aria-modal="true" aria-label={t('Colours and sides')}>
  <div class="card">
    <h2>{t('Colours and sides')}</h2>

    {#each ['red', 'blue'] as const as side (side)}
      <section>
        <h3>{names[side]} <span class="dim">&middot; {t('the {side} side', { side: t(side) })}</span></h3>
        <!-- Selection is fill and border, never hue: a chosen swatch has a ring of ink,
             so a non-default colour cannot start reading as a selected control. -->
        <div class="swatches">
          {#each COLOURS as colour (colour)}
            <button
              class="swatch"
              class:selected={options[side] === colour}
              style="--tint: var(--tint-{colour}); --bright: var(--bright-{colour})"
              aria-pressed={options[side] === colour}
              disabled={options[otherOf(side)] === colour}
              onclick={() => onColour(side, colour)}>{t(colour)}</button
            >
          {/each}
        </div>
      </section>
    {/each}

    <section>
      <h3>{t('Sides')}</h3>
      <p class="dim">{t('Swapping sides risks a screen that disagrees with the corners of the mat. A different colour is usually the better fix.')}</p>
      <label>
        <input type="checkbox" checked={swapHere} onchange={(e) => onSwapHere(e.currentTarget.checked)} />
        {t('Swap sides on this screen')}
      </label>
      <label>
        <input
          type="checkbox"
          checked={options.swapDisplay}
          onchange={(e) => onSwapDisplay(e.currentTarget.checked)}
        />
        {t('Swap sides on the displays')}
      </label>
    </section>

    <button class="done" onclick={onClose}>{t('Done')}</button>
  </div>
</div>

<style>
  .scrim {
    position: fixed;
    inset: 0;
    background: rgba(6, 8, 12, 0.86);
    display: grid;
    place-items: center;
    padding: 1rem;
    z-index: 50;
  }
  .card {
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: 14px;
    padding: 1.25rem 1.5rem;
    width: min(34rem, 100%);
    max-height: 100%;
    overflow: auto;
    display: grid;
    gap: 1rem;
  }
  h2 {
    margin: 0;
    font-size: 1.25rem;
  }
  h3 {
    margin: 0 0 0.5rem;
    font-size: 1rem;
  }
  .dim {
    color: var(--ink-dim);
    font-weight: 400;
    font-size: 0.9rem;
    line-height: 1.5;
    margin: 0 0 0.5rem;
  }
  .swatches {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
  }
  .swatch {
    padding: 0.7rem 0.9rem;
    min-width: 5.2rem;
    font-size: 0.8rem;
    font-weight: 800;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    background: var(--tint);
    color: var(--bright);
    border: 2px solid var(--bright);
  }
  .swatch.selected {
    box-shadow:
      inset 0 0 0 3px var(--panel),
      inset 0 0 0 5px var(--ink);
  }
  .swatch:disabled {
    opacity: 0.3;
  }
  label {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding: 0.5rem 0;
    font-size: 1rem;
  }
  input[type='checkbox'] {
    width: 1.3rem;
    height: 1.3rem;
    padding: 0;
  }
  .done {
    padding: 1rem;
    font-size: 1.05rem;
    font-weight: 700;
    background: var(--ink);
    color: #0d0f14;
    border: none;
  }
</style>
