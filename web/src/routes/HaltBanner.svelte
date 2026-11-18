<script lang="ts">
  import { clockOf } from '../lib/clock-of-day';
  import { t } from '../lib/i18n.svelte';

  /**
   * A global timeout in force (#157): every mat stops until the organizer lets the hall go
   * on -- an injury needs the physician, who is busy elsewhere. The same words on every
   * score keeper's tablet and every screen.
   */
  let { halt }: { halt?: { since: string; reason?: string } | null } = $props();
</script>

{#if halt}
  <div class="halt" role="alert">
    <strong>{t('Global timeout')}</strong>
    <span>{halt.reason || t('The physician is needed. Every mat waits.')}</span>
    <span class="since">{t('since {time}', { time: clockOf(halt.since) })}</span>
  </div>
{/if}

<style>
  .halt {
    display: flex;
    flex-wrap: wrap;
    gap: 0.3rem 0.8rem;
    align-items: baseline;
    justify-content: center;
    padding: 0.7rem 1rem;
    background: var(--amber-bright);
    color: #111;
    font-size: 1.15rem;
    text-align: center;
  }
  .since {
    opacity: 0.75;
    font-size: 0.95rem;
  }
</style>
