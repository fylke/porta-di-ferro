<script lang="ts">
  import { onMount } from 'svelte';
  import { navigate } from '../router.svelte';
  import { Live } from '../lib/live.svelte';

  // The screen a score keeper lands on after scanning the QR code: pick a mat, once.
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
</script>

<main>
  <h1>Which mat?</h1>
  {#if live.error}
    <p class="err">Cannot reach the server: {live.error}</p>
  {/if}
  <div class="mats">
    {#each mats as mat (mat)}
      <button onclick={() => navigate(`/score/${mat}`)}>Mat {mat}</button>
    {/each}
  </div>
  <p class="hint">
    Pick the mat this device is sitting at. It stays on that mat for the whole event, and
    follows whichever match is up next there.
  </p>
</main>

<style>
  main {
    max-width: 32rem;
    margin: 0 auto;
    padding: 3rem 1.5rem;
  }
  h1 {
    font-size: 2rem;
    margin: 0 0 1.5rem;
  }
  .mats {
    display: grid;
    gap: 1rem;
  }
  button {
    padding: 2rem;
    font-size: 1.6rem;
    font-weight: 700;
    background: var(--panel-2);
    border: 2px solid var(--line);
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
