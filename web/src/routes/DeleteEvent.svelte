<script lang="ts">
  import { api } from '../api';
  import { hall } from '../lib/event.svelte';
  import { navigate } from '../router.svelte';
  import { t } from '../lib/i18n.svelte';
  import ConfirmDialog from './ConfirmDialog.svelte';

  /**
   * Deleting the event (#141), for the next one: every discipline, entry, result and the
   * day around them leave the hall, and an empty event takes their place, as on a first
   * start. Nothing is destroyed -- the files are moved under deleted/ in the event folder
   * -- but everything leaves every screen and tablet at once, so it asks first, and says
   * where the files go.
   */
  let { name = '', dir = '' }: { name?: string; dir?: string } = $props();

  let asking = $state(false);
  let busy = $state(false);
  let error = $state('');
  let deleted = $state('');

  async function remove() {
    asking = false;
    busy = true;
    error = '';
    try {
      deleted = (await api.deleteEvent()).deleted;
      navigate('/admin');
      // The new event has an identity of its own, so this page forgets the old one and
      // starts again (lib/forget.ts), as every tablet and screen in the hall does.
      await hall.refresh();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      busy = false;
    }
  }
</script>

<section>
  <h2>{t('Delete the whole event')}</h2>
  <p class="dim">
    {t('For the next event: every discipline, competitor and result, and the day around them, leave the hall, and an empty event takes their place. The files are kept, moved to the deleted folder inside')}
    <span class="mono">{dir}</span>.
  </p>
  <button class="danger" disabled={busy} onclick={() => (asking = true)}>{t('Delete the event…')}</button>
  {#if deleted}<p class="dim">{t('Moved to {dir}', { dir: deleted })}</p>{/if}
  {#if error}<p class="err">{error}</p>{/if}
</section>

{#if asking}
  <ConfirmDialog
    headline={name ? t('Delete {name}?', { name }) : t('Delete the event?')}
    detail={t('Every screen and tablet in the hall leaves it at once, and what they kept of it is forgotten. The files are kept in the deleted folder, from where they can be moved back by hand.')}
    confirmLabel={t('Yes, delete the event')}
    onConfirm={() => void remove()}
    onCancel={() => (asking = false)}
  />
{/if}

<style>
  section {
    background: var(--panel);
    border-radius: var(--radius);
    padding: 1.1rem 1.25rem;
    margin-bottom: 1rem;
    border-left: 3px solid var(--red);
  }
  h2 {
    margin: 0 0 0.5rem;
    font-size: 1.15rem;
  }
  .dim {
    margin: 0 0 0.8rem;
    color: var(--ink-dim);
    line-height: 1.5;
    font-size: 0.9rem;
    overflow-wrap: anywhere;
  }
  button.danger {
    border-color: var(--red);
  }
  .err {
    color: var(--amber-bright);
  }
</style>
