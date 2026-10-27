<script lang="ts">
  import { t } from '../lib/i18n.svelte';

  /**
   * The one save button (#149). It says Save, and once pressed Saved ✓ -- greyed, where the
   * eye already is -- until the next edit. A green "Saved" further down the panel was too
   * easy to miss. With nothing edited it is a Save that cannot be pressed.
   */
  let {
    dirty,
    busy = false,
    label = '',
    type = 'button',
    onclick,
  }: {
    /** Something on the screen is not saved yet. */
    dirty: boolean;
    busy?: boolean;
    /** What it says while there is something to save; Save if blank. */
    label?: string;
    type?: 'button' | 'submit';
    onclick?: () => void;
  } = $props();

  // Saved once something was edited and then stored; an edit takes it back.
  let saved = $state(false);
  let was = false;
  $effect(() => {
    if (was && !dirty) saved = true;
    if (dirty) saved = false;
    was = dirty;
  });
</script>

<button {type} class:saved disabled={busy || !dirty} {onclick}>
  {#if saved}{t('Saved')} &check;{:else}{label || t('Save')}{/if}
</button>

<style>
  /* Greyed rather than faded: it is a state to read, not a control that went away. */
  .saved:disabled {
    opacity: 1;
    background: var(--panel-2);
    border-color: var(--line);
    color: var(--ink-dim);
  }
</style>
