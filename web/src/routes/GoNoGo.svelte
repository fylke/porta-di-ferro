<script lang="ts">
  import { api, type GoNoGoView } from '../api';
  import { clockOf } from '../lib/clock-of-day';
  import { roleLabel } from '../lib/items';
  import { t } from '../lib/i18n.svelte';
  import { errorText } from '../lib/errors';

  /**
   * Go / no-go (#167): what could go wrong with this discipline, in one place, and the
   * tournament manager's sign-off. Something major changing afterwards -- its mats, its block
   * or day, its start, its pool sizes -- asks for a new one. It warns and never refuses:
   * the pools draw and the mats run without it.
   */
  // refresh is anything that changes when the summary might: it is asked for again then.
  let { slug, refresh }: { slug: string; refresh?: unknown } = $props();

  let view = $state<GoNoGoView | null>(null);
  let by = $state('');
  let error = $state('');
  let busy = $state(false);

  $effect(() => {
    void refresh;
    void api
      .goNoGo(slug)
      .then((v) => (view = v))
      .catch((e) => (error = errorText(e)));
  });

  async function signOff() {
    error = '';
    busy = true;
    try {
      view = await api.signOff(slug, by);
    } catch (e) {
      error = errorText(e);
    } finally {
      busy = false;
    }
  }

  function changedLabel(key: string): string {
    switch (key) {
      case 'mats':
        return t('the mats');
      case 'block':
        return t('the block of the day');
      case 'day':
        return t('the day');
      case 'start':
        return t('the start');
      case 'pools':
        return t('the pool sizes');
      default:
        return t('the mats for the eliminations');
    }
  }
</script>

<section class="gonogo" data-tour="gonogo">
  <h2>{t('Go / no-go')}</h2>
  {#if view}
    {#if view.warnings.length === 0}
      <p class="ok">{t('Nothing to warn about.')}</p>
    {:else}
      <ul class="warnings">
        {#each view.warnings as w, i (i)}
          <li>
            {#if w.kind === 'staff'}
              {w.noStaff
                ? t('Nobody is on the staff yet.')
                : t('Not enough staff: {roles} (short {n}).', {
                    n: w.short ?? 0,
                    roles: (w.roles ?? []).map(roleLabel).join(', ').toLocaleLowerCase(),
                  })}
            {:else if w.kind === 'overrun'}
              {t('Runs until {end}, past the venue closing at {close}.', { end: clockOf(w.end ?? ''), close: w.close ?? '' })}
            {:else}
              {t('Only {mats} of the {recommended} mats its numbers ask for: the pools wait for each other, or run larger.', {
                mats: w.mats ?? 0,
                recommended: w.recommended ?? 0,
              })}
            {/if}
          </li>
        {/each}
      </ul>
    {/if}

    {#if view.signoff && view.current}
      <p class="go">{t('Go: signed off by {by} at {time}.', { by: view.signoff.by, time: clockOf(view.signoff.at) })}</p>
    {:else}
      {#if view.signoff}
        <p class="warn">
          {t('Signed off by {by} at {time}, but {what} changed since: sign off again.', {
            by: view.signoff.by,
            time: clockOf(view.signoff.at),
            what: (view.changed ?? []).map(changedLabel).join(', '),
          })}
        </p>
      {:else}
        <p class="warn">{t('Not signed off yet.')}</p>
      {/if}
      <form onsubmit={(e) => (e.preventDefault(), void signOff())}>
        <input bind:value={by} maxlength="80" placeholder={t('Signed off by')} aria-label={t('Who signs off')} />
        <button type="submit" disabled={busy || !by.trim()}>{t('Go: sign off')}</button>
      </form>
    {/if}
  {/if}
  {#if error}<p class="err">{error}</p>{/if}
</section>

<style>
  .gonogo {
    background: var(--panel);
    border-radius: var(--radius);
    padding: 1.1rem 1.25rem;
    margin-bottom: 1rem;
  }
  h2 {
    margin: 0 0 0.6rem;
  }
  .warnings {
    margin: 0 0 0.6rem;
    padding-left: 1.1rem;
  }
  .warnings li {
    color: var(--amber-bright);
  }
  .go {
    color: var(--green, #3d9a5c);
    font-weight: 600;
  }
  form {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
  }
</style>
