<!-- REQ: NFR_SEC_01 -->
<script lang="ts">
  import { onOrganizerPC } from '../lib/organizer';
  import { t } from '../lib/i18n.svelte';

  /**
   * Said at the top of an organizer page opened anywhere but the organizer's own PC (#165):
   * the page can be looked at from the hall, but the server takes changes from that PC
   * alone, so every button here would answer with a refusal.
   */
  const away = onOrganizerPC() === false;
</script>

{#if away}
  <p class="look-only" role="note">
    {t('You can look at this page here, but changes are made on the computer running Porta di Ferro.')}
  </p>
{/if}

<style>
  .look-only {
    margin: 0 0 1rem;
    padding: 0.6rem 0.9rem;
    border-radius: var(--radius);
    background: var(--panel);
    border: 1px solid var(--amber-bright);
    color: var(--ink);
  }
</style>
