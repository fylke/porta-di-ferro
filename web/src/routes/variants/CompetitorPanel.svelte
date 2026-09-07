<script lang="ts">
  import type { Side } from '../../lib/match';
  import type { Selection } from '../../lib/scorekeeper.svelte';

  /**
   * One competitor's half of the score keeper view.
   *
   * `variant` is the prototype knob (design §4): the authors disagree about bordered
   * buttons versus edge-to-edge clickable panels, which is a good reason to build both
   * and put them in front of real competitors on a club night.
   */
  let {
    side,
    name,
    score,
    warnings,
    selection,
    variant,
    disabled = false,
    onPoint,
    onWarning,
  }: {
    side: Side;
    name: string;
    score: number;
    warnings: number;
    selection: Selection;
    variant: string;
    disabled?: boolean;
    onPoint: (value: number) => void;
    onWarning: () => void;
  } = $props();
</script>

<section class="panel {side} v-{variant}" aria-label="{side} competitor">
  <header>
    <span class="label">{side.toUpperCase()}</span>
    <span class="name">{name}</span>
  </header>

  <div class="score mono" aria-label="score">
    {score}
    {#if warnings > 0}
      <span class="warnings" aria-label="{warnings} warnings">
        {#each { length: warnings } as _, i (i)}<span class="triangle">&#9650;</span>{/each}
      </span>
    {/if}
  </div>

  <div class="points">
    {#each [2, 1] as value (value)}
      <button
        class="point"
        class:selected={selection.value === value}
        {disabled}
        aria-pressed={selection.value === value}
        onclick={() => onPoint(value)}>{value}</button
      >
    {/each}
  </div>

  <button
    class="warning"
    class:selected={selection.penalty > 0}
    {disabled}
    aria-pressed={selection.penalty > 0}
    onclick={onWarning}>WARNING!</button
  >
</section>

<style>
  /* Each competitor's half is tinted with their colour, matching the wristbands, so the
     score keeper's eye lands on the right half without reading anything. */
  .panel {
    display: grid;
    grid-template-rows: auto auto 1fr auto;
    gap: 0.5rem;
    padding: 0.75rem;
    min-height: 0;
  }
  .panel.red {
    background: var(--red-tint);
  }
  .panel.blue {
    background: var(--blue-tint);
  }

  header {
    display: flex;
    align-items: baseline;
    gap: 0.6rem;
    min-width: 0;
  }
  .label {
    font-weight: 800;
    letter-spacing: 0.08em;
    font-size: 0.85rem;
  }
  .red .label {
    color: var(--red-bright);
  }
  .blue .label {
    color: var(--blue-bright);
  }
  .name {
    font-size: 1rem;
    color: var(--ink);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .score {
    font-size: clamp(2.5rem, 9vh, 5rem);
    font-weight: 800;
    line-height: 1;
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  /* Amber, distinct from both competitor colours and conventional for the meaning. One
     triangle per warning: the count is what matters, because the score keeper needs to see
     whether the next one costs a point or ends the match. */
  .warnings {
    display: inline-flex;
    gap: 0.15rem;
    font-size: 0.4em;
    color: var(--amber-bright);
  }

  .points {
    display: grid;
    grid-template-rows: 1fr 1fr;
    gap: 0.5rem;
    min-height: 0;
  }
  .point {
    font-size: clamp(1.8rem, 6vh, 3rem);
    font-weight: 800;
    background: var(--panel-2);
    border: 2px solid var(--line);
  }

  .warning {
    padding: 0.7rem;
    font-size: clamp(0.85rem, 2.2vh, 1.05rem);
    font-weight: 800;
    letter-spacing: 0.04em;
    background: var(--panel-2);
    border: 2px solid var(--line);
    color: var(--amber-bright);
    /* Shrunk deliberately: rarely used, and it was competing with the scoring controls
       for area. */
    max-height: 4rem;
  }

  /* Selection is shown by fill and border weight, never by hue -- the same treatment on
     both sides, so a selected button looks selected regardless of whose it is. */
  .point.selected,
  .warning.selected {
    background: var(--ink);
    color: #0d0f14;
    border-color: var(--ink);
    box-shadow: inset 0 0 0 4px var(--bg);
  }
  .warning.selected {
    background: var(--amber-bright);
    border-color: var(--amber-bright);
    color: #1a1200;
  }

  button:active {
    filter: brightness(1.35);
  }
  button:disabled {
    opacity: 0.4;
  }

  /* The edge-to-edge variant: no dead space, every region clickable, and the buttons
     lose their borders in favour of filling their whole area. */
  .v-edge {
    gap: 2px;
    padding: 0;
  }
  .v-edge header,
  .v-edge .score {
    padding: 0.5rem 0.75rem;
  }
  .v-edge .points {
    gap: 2px;
  }
  .v-edge .point,
  .v-edge .warning {
    border: none;
    border-radius: 0;
    background: rgba(255, 255, 255, 0.06);
  }
  .v-edge .point.selected,
  .v-edge .warning.selected {
    box-shadow: inset 0 0 0 6px var(--bg);
  }
</style>
