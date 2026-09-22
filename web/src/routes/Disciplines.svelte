<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type Instance } from '../api';
  import { t } from '../lib/i18n.svelte';

  /**
   * Concurrent disciplines (design §7 item 9). Several at once are several runs of the
   * application at once, each with its own port and data directory; this is the organizer
   * starting the second one from here rather than from a terminal, and seeing them all.
   *
   * Every one of them is named from here too, this one included. The name used to arrive
   * only as a command-line flag, which the shortcut that starts the first run does not
   * pass -- so the one discipline an organizer always has was called "Unnamed" and there
   * was nowhere to say otherwise (issue #80). The usual runs are on the list rather than
   * typed, because the same four come round at every event and two spellings of the
   * women's longsword is two disciplines where there should be one.
   */
  let { self, onrenamed }: { self: Instance; onrenamed?: () => void } = $props();

  let instances = $state<Instance[]>([]);
  let presets = $state<string[]>([]);
  let name = $state('');
  let error = $state('');
  let busy = $state(false);

  // The discipline being renamed, by port, with the text as it stands.
  let editingPort = $state<number | null>(null);
  let editingName = $state('');

  async function refresh() {
    try {
      instances = await api.instances();
    } catch {
      // Fine: the list is a convenience, and it refreshes on the next action.
    }
  }

  onMount(() => {
    void refresh();
    void (async () => {
      try {
        presets = await api.disciplines();
      } catch {
        // Without the list the field is still a field; it is a shortcut, not a gate.
      }
    })();
    const poll = setInterval(() => {
      if (document.visibilityState === 'visible') void refresh();
    }, 5000);
    return () => clearInterval(poll);
  });

  // What is left to start: the list minus what is already running, so the quick picks
  // are only ever things that would work.
  const running = $derived(new Set(instances.map((i) => i.name.toLowerCase())));
  const available = $derived(presets.filter((p) => !running.has(p.toLowerCase())));

  function startEditing(i: Instance) {
    editingPort = i.port;
    // An unnamed run opens on the default, so naming it is one press rather than typing
    // out "Women's and underrepresented genders Longsword".
    editingName = i.name || presets[0] || '';
    error = '';
  }

  async function saveName() {
    if (editingPort === null) return;
    error = '';
    busy = true;
    try {
      instances = await api.renameInstance(editingPort, editingName);
      editingPort = null;
      onrenamed?.();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      busy = false;
    }
  }

  async function start(discipline?: string) {
    error = '';
    const wanted = (discipline ?? name).trim();
    if (!wanted) return;
    busy = true;
    try {
      const inst = await api.startInstance(wanted);
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
        {#if editingPort === i.port}
          <form
            class="rename"
            onsubmit={(e) => {
              e.preventDefault();
              void saveName();
            }}
          >
            <input
              list="discipline-names"
              bind:value={editingName}
              aria-label={t('Name of this discipline')}
              required
            />
            <button type="submit" disabled={busy || !editingName.trim()}>{t('Save')}</button>
            <button type="button" onclick={() => (editingPort = null)}>{t('Cancel')}</button>
          </form>
        {:else}
          <span class="who" class:unnamed={!i.name}>{i.name || t('Unnamed')}</span>
          {#if i.self}
            <span class="meta">{t('this one · port {n}', { n: i.port })}</span>
          {:else}
            <a href={i.url} target="_blank" rel="noreferrer">{i.url}</a>
          {/if}
          <button class="edit" onclick={() => startEditing(i)}>{t('Rename')}</button>
          {#if !i.self}
            <button onclick={() => void stop(i.port)}>{t('Stop')}</button>
          {/if}
        {/if}
      </li>
    {/each}
  </ul>

  <!-- Shared by the rename field and the start field: the same four names either way. -->
  <datalist id="discipline-names">
    {#each presets as p (p)}<option value={p}></option>{/each}
  </datalist>

  {#if self.parent}
    <p class="dim">{t('Started from')} <a href={self.parent}>{self.parent}</a>.</p>
  {:else}
    {#if available.length > 0}
      <p class="presets">
        <span class="label">{t('Start one of the usual')}</span>
        {#each available as p (p)}
          <button type="button" disabled={busy} onclick={() => void start(p)}>{p}</button>
        {/each}
      </p>
    {/if}
    <form
      onsubmit={(e) => {
        e.preventDefault();
        void start();
      }}
    >
      <input
        list="discipline-names"
        placeholder={t('Rapier and dagger, Sword and buckler, …')}
        bind:value={name}
        required
      />
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
  /* A run nobody has named yet says so quietly, and the button beside it is the answer. */
  .who.unnamed {
    font-weight: 400;
    font-style: italic;
    color: var(--ink-dim);
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
  .rename {
    flex: 1;
    min-width: 0;
  }
  /* The quick picks: the four that come round at every event, one press each, and only
     the ones not already running. */
  .presets {
    margin: 0 0 0.6rem;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.4rem;
  }
  .presets .label {
    font-size: 0.8rem;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--ink-dim);
    margin-right: 0.2rem;
  }
  button {
    padding: 0.45rem 0.8rem;
    font-size: 0.9rem;
    background: var(--panel-2);
    border: 1px solid var(--line);
  }
  .edit {
    padding: 0.3rem 0.6rem;
    font-size: 0.82rem;
    color: var(--ink-dim);
  }
  .edit:hover {
    color: var(--ink);
  }
  .err {
    margin: 0.5rem 0 0;
    color: var(--amber-bright);
  }
</style>
