<script lang="ts">
  import type { Pending } from '../lib/match';

  /**
   * The three match-ending dialogs. One component parameterised on its second action --
   * not three components, and not one identical dialog (design §4).
   *
   * After time expires, continuing is a legitimate call and the head referee decides.
   * After either cap the match is over by rule, and the second option exists only as a
   * safety net against a mis-tap.
   */
  let {
    pending,
    headline,
    onEnd,
    onSecond,
  }: {
    pending: Pending;
    headline: string;
    onEnd: () => void;
    onSecond: () => void;
  } = $props();

  const secondLabel = $derived(
    pending === 'final_exchange' ? 'Continue one more exchange' : 'Undo last exchange',
  );
</script>

<div class="scrim" role="dialog" aria-modal="true" aria-label={headline}>
  <div class="card">
    <p class="headline">{headline}</p>
    <button class="end" onclick={onEnd}>End match</button>
    <button class="second" onclick={onSecond}>{secondLabel}</button>
  </div>
</div>

<style>
  .scrim {
    position: fixed;
    inset: 0;
    background: rgba(6, 8, 12, 0.86);
    display: grid;
    place-items: center;
    padding: 1.5rem;
    z-index: 50;
  }
  .card {
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: 14px;
    padding: 1.75rem;
    width: min(34rem, 100%);
    display: grid;
    gap: 0.9rem;
  }
  .headline {
    margin: 0 0 0.5rem;
    font-size: clamp(1.4rem, 4vw, 2rem);
    font-weight: 700;
    text-align: center;
    line-height: 1.3;
  }
  button {
    padding: 1.2rem;
    font-size: 1.15rem;
    font-weight: 700;
    border: 2px solid var(--line);
    background: var(--panel-2);
  }
  .end {
    background: var(--ok);
    border-color: var(--ok);
  }
</style>
