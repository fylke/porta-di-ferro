<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type Snapshot } from '../api';
  import { discipline, dhref } from '../router.svelte';
  import { t } from '../lib/i18n.svelte';
  import LangToggle from './LangToggle.svelte';

  /**
   * Setting up one discipline (#149), at /d/{discipline}/setup: just what it has to have
   * before the day can be planned, on one page, and a button that says when it is done. The
   * planning step opens it in a tab of its own, and shows the discipline as set up the
   * moment Finish is pressed here.
   *
   * Everything else about a discipline -- its competitors, its draw, its eliminations -- is
   * on its admin page, which this links to once it is done.
   */
  let snapshot = $state<Snapshot | null>(null);
  let error = $state('');
  let busy = $state(false);
  let done = $state(false);

  let mats = $state(2);
  let minPoolSize = $state(4);
  let maxPoolSize = $state(7);
  let expected = $state<number | null>(null);
  let capacity = $state<number | null>(null);

  const slug = discipline();

  onMount(() => {
    void (async () => {
      try {
        const [snap, forecast] = await Promise.all([api.state(), api.forecast()]);
        snapshot = snap;
        mats = snap.tournament.mats;
        minPoolSize = snap.tournament.minPoolSize;
        maxPoolSize = snap.tournament.maxPoolSize;
        capacity = snap.tournament.capacity || null;
        expected = forecast.expected?.[slug] || null;
        done = !!snap.tournament.setUp;
      } catch (e) {
        error = e instanceof Error ? e.message : String(e);
      }
    })();
  });

  const entered = $derived(snapshot?.competitors.filter((c) => !c.withdrawn).length ?? 0);
  const valid = $derived(minPoolSize >= 2 && maxPoolSize <= 7 && minPoolSize <= maxPoolSize && (expected ?? 0) >= 0);

  async function finish(e: SubmitEvent) {
    e.preventDefault();
    if (!valid) return;
    error = '';
    busy = true;
    try {
      await api.saveTournament(mats, minPoolSize, maxPoolSize, undefined, Math.max(0, Math.round(capacity ?? 0)), true);
      await api.setExpected({ [slug]: Math.max(0, Math.round(expected ?? 0)) });
      done = true;
    } catch (err) {
      error = err instanceof Error ? err.message : String(err);
    } finally {
      busy = false;
    }
  }
</script>

<main class="admin">
  <header>
    <h1>
      {t('Set up')}
      {#if snapshot?.instance.name}<span class="disc">{snapshot.instance.name}</span>{/if}
    </h1>
    <LangToggle />
  </header>

  {#if !snapshot}
    <p class="dim">{error || t('Loading…')}</p>
  {:else}
    <form onsubmit={finish}>
      <p class="dim">
        {t('What this discipline needs before the day can be planned. Everything here can be changed later on its admin page.')}
      </p>

      <fieldset>
        <legend>{t('Mats')}</legend>
        <label>
          {t('How many mats its pools use')}
          <select bind:value={mats} onchange={() => (done = false)}>
            {#each [1, 2, 3, 4] as n (n)}<option value={n}>{n}</option>{/each}
          </select>
        </label>
      </fieldset>

      <fieldset>
        <legend>{t('Pools')}</legend>
        <label>
          {t('Smallest pool')}
          <input type="number" min="2" max="7" bind:value={minPoolSize} oninput={() => (done = false)} required />
        </label>
        <label>
          {t('Largest pool')}
          <input type="number" min="2" max="7" bind:value={maxPoolSize} oninput={() => (done = false)} required />
        </label>
      </fieldset>

      <fieldset>
        <legend>{t('Entries')}</legend>
        <label>
          {t('How many you expect')}
          <input type="number" min="0" max="200" bind:value={expected} oninput={() => (done = false)} placeholder={entered ? String(entered) : ''} />
        </label>
        <label>
          {t('Places (optional)')}
          <input type="number" min="0" bind:value={capacity} oninput={() => (done = false)} placeholder={t('no limit')} />
        </label>
        <p class="hint">{t('The day is planned for the number you expect until that many are entered and the pools drawn.')}</p>
      </fieldset>

      {#if !valid}<p class="err">{t('A pool has between 2 and 7, and the smallest no bigger than the largest.')}</p>{/if}
      {#if error}<p class="err">{error}</p>{/if}

      <div class="actions">
        <button type="submit" class:finished={done} disabled={busy || !valid || done}>
          {#if done}{t('Set up')} &check;{:else}{t('Finish setting up')}{/if}
        </button>
        {#if done}
          <span class="ok">{t('Done. You can close this tab, or go on to its admin page.')}</span>
          <a href={dhref('/admin')}>{t('Its admin page')}</a>
        {/if}
      </div>
    </form>
  {/if}
</main>

<style>
  main {
    max-width: 40rem;
    margin: 0 auto;
    padding: 1.25rem 16px 4rem;
  }
  header {
    display: flex;
    flex-wrap: wrap;
    gap: 1rem;
    justify-content: space-between;
    align-items: baseline;
    margin-bottom: 1rem;
  }
  h1 {
    margin: 0;
    font-size: 1.5rem;
    display: flex;
    flex-wrap: wrap;
    gap: 0.6rem;
    align-items: baseline;
  }
  .disc {
    color: var(--amber-bright);
    font-size: 1.1rem;
  }
  form {
    background: var(--panel);
    border-radius: var(--radius);
    padding: 1.1rem 1.25rem;
    display: grid;
    gap: 0.9rem;
  }
  fieldset {
    border: 1px solid var(--line);
    border-radius: var(--radius);
    padding: 0.7rem 0.9rem 0.9rem;
    margin: 0;
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
    gap: 0.6rem;
  }
  legend {
    font-size: 0.72rem;
    font-weight: 800;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--ink-dim);
    padding: 0 0.3rem;
  }
  label {
    display: grid;
    gap: 0.3rem;
    font-size: 0.85rem;
    color: var(--ink-dim);
  }
  input,
  select {
    min-width: 0;
    padding: 0.4rem 0.5rem;
  }
  .hint {
    grid-column: 1 / -1;
    margin: 0;
    font-size: 0.82rem;
    color: var(--ink-dim);
  }
  .dim {
    margin: 0;
    color: var(--ink-dim);
    line-height: 1.5;
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.6rem 1rem;
  }
  .finished:disabled {
    opacity: 1;
    background: var(--panel-2);
    border-color: var(--line);
    color: var(--ink-dim);
  }
  .ok {
    color: var(--ink-dim);
    font-size: 0.9rem;
  }
  .err {
    margin: 0;
    color: var(--amber-bright);
  }
</style>
