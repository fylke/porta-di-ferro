<script lang="ts">
  import { t } from '../lib/i18n.svelte';

  /** The fold toggle at the start of a section's heading (#110). */
  let { open, label, ontoggle }: { open: boolean; label: string; ontoggle: () => void } = $props();
</script>

<button
  class="fold"
  aria-expanded={open}
  aria-label={open ? t('Fold away {what}', { what: label }) : t('Show {what}', { what: label })}
  title={open ? t('Fold away') : t('Show')}
  onclick={ontoggle}
>
  <span class="chev" class:closed={!open} aria-hidden="true">&#9662;</span>
</button>

<style>
  .fold {
    display: inline-grid;
    place-items: center;
    width: 1.6rem;
    height: 1.6rem;
    padding: 0;
    margin-right: 0.2rem;
    border: none;
    background: none;
    color: var(--ink-dim);
    font-size: 0.9rem;
    vertical-align: middle;
    flex: none;
  }
  .fold:hover {
    color: var(--ink);
  }
  .chev {
    transition: transform 0.15s;
  }
  .chev.closed {
    transform: rotate(-90deg);
  }
</style>
