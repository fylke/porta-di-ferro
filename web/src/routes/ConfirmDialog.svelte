<script lang="ts">
  /** Undo is rare and destructive, so it asks before it applies (design §4). */
  let {
    headline,
    detail = '',
    confirmLabel = 'Yes, undo it',
    onConfirm,
    onCancel,
  }: {
    headline: string;
    detail?: string;
    confirmLabel?: string;
    onConfirm: () => void;
    onCancel: () => void;
  } = $props();
</script>

<div class="scrim" role="dialog" aria-modal="true" aria-label={headline}>
  <div class="card">
    <p class="headline">{headline}</p>
    {#if detail}<p class="detail">{detail}</p>{/if}
    <button class="confirm" onclick={onConfirm}>{confirmLabel}</button>
    <button onclick={onCancel}>Cancel</button>
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
    z-index: 60;
  }
  .card {
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: 14px;
    padding: 1.75rem;
    width: min(30rem, 100%);
    display: grid;
    gap: 0.75rem;
  }
  .headline {
    margin: 0;
    font-size: 1.4rem;
    font-weight: 700;
    text-align: center;
  }
  .detail {
    margin: 0 0 0.5rem;
    color: var(--ink-dim);
    text-align: center;
    line-height: 1.5;
  }
  button {
    padding: 1rem;
    font-size: 1.05rem;
    font-weight: 700;
    border: 2px solid var(--line);
    background: var(--panel-2);
  }
  .confirm {
    background: var(--amber);
    border-color: var(--amber);
    color: #1a1200;
  }
</style>
