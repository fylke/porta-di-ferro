<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type DisciplineSummary } from '../api';
  import { hall } from '../lib/event.svelte';
  import { stageLabel } from '../lib/stage';
  import { t } from '../lib/i18n.svelte';

  /**
   * The event's disciplines: add one, rename one, take one out (#4, #102).
   *
   * Every discipline of the day runs in this one application, at this one address, each in
   * a folder of its own (docs/proposals/one-event-many-disciplines.md). Adding one used to
   * start a second copy of the application on the next free port; it is now a folder and a
   * tournament in this one, and it is on the landing page beside the others the moment it
   * exists.
   *
   * The usual disciplines are on the list rather than typed, because the same four come
   * round at every event and two spellings of the women's longsword is two disciplines
   * where there should be one (issue #80).
   */
  let { onchange }: { onchange?: () => void } = $props();

  let presets = $state<string[]>([]);
  let name = $state('');
  let error = $state('');
  let busy = $state(false);
  let editing = $state<string | null>(null);
  let editingName = $state('');
  let retiring = $state<string | null>(null);

  onMount(() => {
    void hall.refresh();
    void (async () => {
      try {
        presets = await api.presets();
      } catch {
        // Without the list the field is still a field; it is a shortcut, not a gate.
      }
    })();
  });

  const disciplines = $derived(hall.view?.disciplines ?? []);
  // What is left to add: the list minus what is already here, so the quick picks are only
  // ever things that would work.
  const taken = $derived(new Set(disciplines.map((d) => d.name.toLowerCase())));
  const available = $derived(presets.filter((p) => !taken.has(p.toLowerCase())));

  async function act(fn: () => Promise<unknown>) {
    error = '';
    busy = true;
    try {
      await fn();
      await hall.refresh();
      // The one discipline's admin page refreshes its snapshot on a change -- unless the
      // change made the event several disciplines, when that page is about to give way to
      // the event's, and the address it would ask no longer means one discipline.
      if (!hall.multi) onchange?.();
      return true;
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
      return false;
    } finally {
      busy = false;
    }
  }

  async function add(discipline?: string) {
    const wanted = (discipline ?? name).trim();
    if (!wanted) return;
    // The field is cleared only when it was the field that was added, and only if it
    // still says so: a preset's button leaves it alone, and so does a save that comes
    // back after the organizer has typed something else.
    const typed = discipline === undefined ? name : null;
    if ((await act(() => api.addDiscipline(wanted))) && typed !== null && name === typed) name = '';
  }

  function startEditing(d: DisciplineSummary) {
    editing = d.slug;
    // An unnamed discipline opens on the default, so naming it is one press rather than
    // typing out "Women's and underrepresented genders Longsword".
    editingName = d.name || presets[0] || '';
    error = '';
  }

  async function saveName() {
    if (editing === null) return;
    const slug = editing;
    if (await act(() => api.renameDiscipline(slug, editingName))) editing = null;
  }

  async function retire(slug: string) {
    if (await act(() => api.retireDiscipline(slug))) retiring = null;
  }
</script>

<section>
  <h2>{t('Disciplines')}</h2>
  <p class="dim">{t('Every discipline of the event runs here, at this one address, each with its own data folder. Score keepers, screens and spectators reach all of them from the same place.')}</p>

  <ul>
    {#each disciplines as d (d.slug)}
      <li class:failed={!!d.error}>
        {#if editing === d.slug}
          <form
            class="rename"
            onsubmit={(e) => {
              e.preventDefault();
              void saveName();
            }}
          >
            <input list="discipline-names" bind:value={editingName} aria-label={t('Name of this discipline')} required />
            <button type="submit" disabled={busy || !editingName.trim()}>{t('Save')}</button>
            <button type="button" onclick={() => (editing = null)}>{t('Cancel')}</button>
          </form>
        {:else}
          <span class="who" class:unnamed={!d.name}>{d.name || t('Unnamed')}</span>
          {#if d.error}
            <span class="meta warn">{t('could not be read')}</span>
          {:else}
            <span class="meta">{stageLabel(d)}</span>
          {/if}
          {#if disciplines.length > 1}
            <a href="/d/{d.slug}/admin">{t('Admin')}</a>
          {/if}
          {#if !d.error}
            <button class="edit" onclick={() => startEditing(d)}>{t('Rename')}</button>
          {:else}
            <button onclick={() => void act(() => api.reloadDiscipline(d.slug))}>{t('Try again')}</button>
          {/if}
          {#if disciplines.length > 1}
            {#if retiring === d.slug}
              <span class="confirm">
                {t('Take {name} out of the event? Its folder is kept.', { name: d.name || d.slug })}
                <button class="danger" disabled={busy} onclick={() => void retire(d.slug)}>{t('Take it out')}</button>
                <button onclick={() => (retiring = null)}>{t('Cancel')}</button>
              </span>
            {:else}
              <button onclick={() => (retiring = d.slug)}>{t('Take out')}</button>
            {/if}
          {/if}
        {/if}
        {#if d.error}<p class="err small">{d.error}</p>{/if}
      </li>
    {/each}
  </ul>

  <!-- Shared by the rename field and the add field: the same four names either way. -->
  <datalist id="discipline-names">
    {#each presets as p (p)}<option value={p}></option>{/each}
  </datalist>

  {#if available.length > 0}
    <p class="presets">
      <span class="label">{t('Add one of the usual')}</span>
      {#each available as p (p)}
        <button type="button" disabled={busy} onclick={() => void add(p)}>{p}</button>
      {/each}
    </p>
  {/if}
  <form
    onsubmit={(e) => {
      e.preventDefault();
      void add();
    }}
  >
    <input list="discipline-names" placeholder={t('Rapier and dagger, Sword and buckler, …')} bind:value={name} required />
    <button type="submit" disabled={busy || !name.trim()}>{t('Add a discipline')}</button>
  </form>
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
    gap: 0.45rem;
  }
  li {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.6rem;
    font-size: 0.92rem;
  }
  .who {
    font-weight: 700;
  }
  .who.unnamed {
    color: var(--ink-dim);
    font-style: italic;
  }
  .meta {
    color: var(--ink-dim);
    font-size: 0.82rem;
  }
  .meta.warn,
  .err {
    color: var(--amber-bright);
  }
  .err.small {
    flex-basis: 100%;
    margin: 0;
    font-size: 0.8rem;
    overflow-wrap: anywhere;
  }
  button {
    padding: 0.3rem 0.6rem;
    font-size: 0.82rem;
  }
  button.danger {
    border-color: var(--red);
  }
  .confirm {
    display: inline-flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.85rem;
  }
  .rename,
  form {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
  }
  .rename input,
  form input {
    flex: 1 1 16rem;
    min-width: 0;
  }
  .presets {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.4rem;
    margin: 0 0 0.6rem;
  }
  .label {
    color: var(--ink-dim);
    font-size: 0.85rem;
  }
</style>
