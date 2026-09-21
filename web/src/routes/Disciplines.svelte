<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type Instance } from '../api';
  import { t } from '../lib/i18n.svelte';

  /**
   * Concurrent disciplines (design §7 item 9). Several at once are several runs of the
   * application at once, each with its own port and data directory; this is the organizer
   * starting the second one from here rather than from a terminal, and seeing them all.
   */
  let { self }: { self: Instance } = $props();

  let instances = $state<Instance[]>([]);
  let name = $state('');
  let error = $state('');
  let busy = $state(false);

  async function refresh() {
    try {
      instances = await api.instances();
    } catch {
      // Fine: the list is a convenience, and it refreshes on the next action.
    }
  }

  onMount(() => {
    void refresh();
    const poll = setInterval(() => {
      if (document.visibilityState === 'visible') void refresh();
    }, 5000);
    return () => clearInterval(poll);
  });

  async function start() {
    error = '';
    busy = true;
    try {
      const inst = await api.startInstance(name);
      name = '';
      await refresh();
      // It takes a moment to come up; open it once it answers rather than to a blank tab.
      const opened = window.open('', '_blank');
      const until = Date.now() + 10000;
      const tryOpen = async () => {
        try {
          const res = await fetch(`${inst.url}api/state`, { mode: 'no-cors' });
          if (res) {
            if (opened) opened.location.href = inst.url;
            return;
          }
        } catch {
          // Not up yet.
        }
        if (Date.now() < until) setTimeout(() => void tryOpen(), 500);
        else if (opened) opened.location.href = inst.url;
      };
      void tryOpen();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      busy = false;
    }
  }

  async function stop(port: number) {
    error = '';
    try {
      await api.stopInstance(port);
      await refresh();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }
</script>

<section>
  <h2>{t('Disciplines')}</h2>
  <p class="dim">{t('Each discipline is its own run of the application, on its own port with its own data folder. Score keepers and screens join one discipline by its address; the name shows on every page so nobody has to guess which one they are on.')}</p>

  <ul>
    {#each instances as i (i.port)}
      <li class:self={i.self}>
        <span class="who">{i.name || t('Unnamed')}</span>
        {#if i.self}
          <span class="meta">{t('this one · port {n}', { n: i.port })}</span>
        {:else}
          <a href={i.url} target="_blank" rel="noreferrer">{i.url}</a>
          <button onclick={() => void stop(i.port)}>{t('Stop')}</button>
        {/if}
      </li>
    {/each}
  </ul>

  {#if self.parent}
    <p class="dim">{t('Started from')} <a href={self.parent}>{self.parent}</a>.</p>
  {:else}
    <form
      onsubmit={(e) => {
        e.preventDefault();
        void start();
      }}
    >
      <input placeholder={t('Sabre, Rapier and dagger, …')} bind:value={name} required />
      <button type="submit" disabled={busy || !name.trim()}>{t('Start another discipline')}</button>
    </form>
  {/if}
  {#if error}<p class="err">{error}</p>{/if}
</section>

<style>
  section {
    background: var(--panel);
    border-radius: var(--radius);
    padding: 1.1rem 1.25rem;
    margin-bottom: 1rem;
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
  }
  ul {
    list-style: none;
    margin: 0 0 0.8rem;
    padding: 0;
    display: grid;
    gap: 0.35rem;
  }
  li {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.6rem;
    font-size: 0.92rem;
  }
  li.self .who {
    color: var(--amber-bright);
  }
  .who {
    font-weight: 700;
  }
  .meta {
    color: var(--ink-dim);
  }
  form {
    display: flex;
    gap: 0.5rem;
  }
  form input {
    flex: 1;
  }
  button {
    padding: 0.45rem 0.8rem;
    font-size: 0.9rem;
    background: var(--panel-2);
    border: 1px solid var(--line);
  }
  .err {
    margin: 0.5rem 0 0;
    color: var(--amber-bright);
  }
</style>
