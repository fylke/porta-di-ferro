<script lang="ts">
  import { onMount } from 'svelte';
  import { navigate } from '../router.svelte';
  import { Live } from '../lib/live.svelte';
  import { matchOn, namesFor } from './lib-display.svelte';
  import { t } from '../lib/i18n.svelte';
  import LangToggle from './LangToggle.svelte';

  /**
   * The screen a score keeper lands on after scanning the QR code: pick a mat, once.
   *
   * It is deliberately the whole page. The code used to point at the organizer view, so a
   * tablet arrived on the competitor register and the setup form with the mat picker
   * somewhere further down -- an organizer's screen, on the organizer's PC, none of it any
   * use at a mat. What a score keeper needs here is one decision, in buttons big enough to
   * hit without looking.
   */
  const live = new Live();

  // Started and stopped with the component, like every other route. Opening the stream at
  // module evaluation left an EventSource open after navigating on to /score/:mat, so the
  // device carried a dead subscription for the rest of the event.
  onMount(() => {
    live.start();
    return () => live.stop();
  });

  const mats = $derived(
    live.snapshot ? Array.from({ length: live.snapshot.tournament.mats }, (_, i) => i + 1) : [],
  );

  // What is on each mat right now. A score keeper standing at a mat knows the two people
  // in front of them long before they know which number the organizer gave the mat, so
  // the names are the label that actually identifies the button.
  function upNext(mat: number) {
    const match = matchOn(live.snapshot, mat);
    if (!match) return null;
    const names = namesFor(live.snapshot, match);
    return { pool: match.pool, ...names };
  }
</script>

<main>
  <div class="top"><LangToggle /></div>
  <h1>{t('Which mat?')}</h1>
  {#if live.snapshot?.instance.name}
    <p class="discipline">{live.snapshot.instance.name}</p>
  {/if}
  {#if live.error && !live.snapshot}
    <p class="err">{t('Cannot reach the server:')} {live.error}</p>
  {:else if live.stale}
    <p class="err">{t('The server is out of reach. This is the schedule from the last time it was seen; scoring still works.')}</p>
  {/if}
  <div class="mats">
    {#each mats as mat (mat)}
      {@const up = upNext(mat)}
      <button onclick={() => navigate(`/score/${mat}`)}>
        <span class="n">{t('Mat {n}', { n: mat })}</span>
        <span class="up">
          {#if up}
            {t('Pool {n}', { n: up.pool })} &middot; {up.red} {t('v')} {up.blue}
          {:else}
            {t('Nothing up yet')}
          {/if}
        </span>
      </button>
    {/each}
  </div>
  {#if mats.length === 0 && !live.error}
    <p class="hint">{t('Waiting for the organizer to set up the mats.')}</p>
  {/if}
  <p class="hint">{t('Pick the mat this device is sitting at. It stays on that mat for the whole event, and follows whichever match is up next there.')}</p>
</main>

<style>
  main {
    max-width: 32rem;
    margin: 0 auto;
    padding: 3rem 1.5rem;
  }
  .top {
    display: flex;
    justify-content: flex-end;
    margin-bottom: 0.5rem;
  }
  h1 {
    font-size: 2rem;
    margin: 0 0 1.5rem;
  }
  .discipline {
    margin: -1rem 0 1.5rem;
    font-size: 1.1rem;
    font-weight: 700;
    color: var(--amber-bright);
  }
  .mats {
    display: grid;
    gap: 1rem;
  }
  button {
    display: grid;
    gap: 0.4rem;
    padding: 1.6rem;
    text-align: left;
    background: var(--panel-2);
    border: 2px solid var(--line);
  }
  .n {
    font-size: 1.6rem;
    font-weight: 700;
  }
  .up {
    font-size: 1rem;
    color: var(--ink-dim);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .hint {
    color: var(--ink-dim);
    line-height: 1.6;
    margin-top: 2rem;
  }
  .err {
    color: var(--amber-bright);
  }
</style>
