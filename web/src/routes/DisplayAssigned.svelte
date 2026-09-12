<script lang="ts">
  import { onMount } from 'svelte';
  import { Heartbeat, deviceName } from '../lib/presence.svelte';
  import DisplayMat from './DisplayMat.svelte';
  import DisplayMats from './DisplayMats.svelte';
  import DisplayRoster from './DisplayRoster.svelte';
  import Audience from './Audience.svelte';
  import LangToggle from './LangToggle.svelte';
  import { t } from '../lib/i18n.svelte';

  /**
   * A server-assigned display (design §7 item 4). Open /display on any screen: it
   * registers itself, says hello every few seconds, and renders whatever the organizer
   * has told it to show -- reassigned on the fly, without anyone touching the screen.
   *
   * Added alongside URL addressing, never instead of it. Pointing a browser at
   * /display/mat/1 stays the fastest way to set a screen up, and this is for the hall
   * where the screens are already up and the organizer is at the desk.
   */
  const beat = new Heartbeat('display');
  onMount(() => {
    beat.start();
    return () => beat.stop();
  });

  // The target is a short path the organizer picked from a list: what to render and
  // for which mat. Anything unknown is treated as unassigned rather than guessed at.
  const target = $derived(beat.me?.target ?? '');
  const shown = $derived.by((): { kind: string; mat: number; ids: string } => {
    const [kind, arg = ''] = target.split('/');
    if (kind === 'mat' || kind === 'audience') {
      const mat = Number(arg);
      return Number.isInteger(mat) && mat > 0 ? { kind, mat, ids: '' } : { kind: '', mat: 0, ids: '' };
    }
    if (kind === 'mats') return { kind, mat: 0, ids: arg };
    if (kind === 'roster') return { kind, mat: 0, ids: '' };
    return { kind: '', mat: 0, ids: '' };
  });
</script>

{#if shown.kind === 'mat'}
  <DisplayMat mat={shown.mat} />
{:else if shown.kind === 'audience'}
  <Audience mat={shown.mat} />
{:else if shown.kind === 'mats'}
  <DisplayMats ids={shown.ids} />
{:else if shown.kind === 'roster'}
  <DisplayRoster />
{:else}
  <main class="waiting">
    <p class="name">{deviceName()}</p>
    <h1>{t('Waiting for the organizer')}</h1>
    <p class="dim">
      {#if beat.online}
        {t('This screen is registered. The organizer picks what it shows from the organizer page, under Screens.')}
      {:else}
        {t('Cannot reach the server. This screen will register itself as soon as it can.')}
      {/if}
    </p>
    <!-- The one control a waiting screen has: set once, before it is assigned and left. -->
    <LangToggle />
  </main>
{/if}

<style>
  .waiting {
    height: 100dvh;
    display: grid;
    align-content: center;
    justify-items: center;
    gap: 0.75rem;
    padding: 2rem;
    text-align: center;
  }
  .name {
    margin: 0;
    font-size: clamp(2rem, 8vh, 5rem);
    font-weight: 800;
    letter-spacing: 0.04em;
  }
  h1 {
    margin: 0;
    font-size: clamp(1.1rem, 3.5vh, 2rem);
    font-weight: 600;
  }
  .dim {
    margin: 0;
    max-width: 34rem;
    color: var(--ink-dim);
    line-height: 1.6;
  }
</style>
