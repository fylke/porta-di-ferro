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
  let {
    onchange,
    bare = false,
    links = false,
  }: {
    onchange?: () => void;
    /** Without its own box and introduction: the list inside another panel (#142). */
    bare?: boolean;
    /** With each discipline's landing page beside its admin page. */
    links?: boolean;
  } = $props();

  let presets = $state<string[]>([]);
  let name = $state('');
  let error = $state('');
  let busy = $state(false);
  let editing = $state<string | null>(null);
  let editingName = $state('');
  let retiring = $state<string | null>(null);
  let adding = $state(false);

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
    if (await act(() => api.addDiscipline(wanted))) {
      if (typed !== null && name === typed) name = '';
      adding = false;
    }
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

{#if bare}
  {@render body()}
{:else}
  <section>
    <h2>{t('Disciplines')}</h2>
    <p class="dim">{t('Every discipline of the event runs here, at this one address, each with its own data folder. Score keepers, screens and spectators reach all of them from the same place.')}</p>
    {@render body()}
  </section>
{/if}

{#snippet body()}
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
          <!-- Links and buttons alike, in one style: the row's actions (#142). -->
          <span class="acts">
            {#if disciplines.length > 1}
              <a class="act" href="/d/{d.slug}/admin">{t('Admin')}</a>
              {#if links && !d.error}<a class="act" href="/d/{d.slug}/">{t('Landing page')}</a>{/if}
            {/if}
            {#if !d.error}
              <button class="act" onclick={() => startEditing(d)}>{t('Rename')}</button>
            {:else}
              <button class="act" onclick={() => void act(() => api.reloadDiscipline(d.slug))}>{t('Try again')}</button>
            {/if}
            {#if disciplines.length > 1}
              {#if retiring === d.slug}
                <span class="confirm">
                  {t('Take {name} out of the event? Its folder is kept.', { name: d.name || d.slug })}
                  <button class="act danger" disabled={busy} onclick={() => void retire(d.slug)}>{t('Take it out')}</button>
                  <button class="act" onclick={() => (retiring = null)}>{t('Cancel')}</button>
                </span>
              {:else}
                <button class="act" onclick={() => (retiring = d.slug)}>{t('Take out')}</button>
              {/if}
            {/if}
          </span>
        {/if}
        {#if d.error}<p class="err small">{d.error}</p>{/if}
      </li>
    {/each}
  </ul>

  <!-- Shared by the rename field and the add field: the same four names either way. -->
  <datalist id="discipline-names">
    {#each presets as p (p)}<option value={p}></option>{/each}
  </datalist>

  <!-- Adding one is rare -- once per discipline per event -- so it is a button that opens
       the choice rather than a form always on the page (#142). -->
  <button class="act add" disabled={busy} onclick={() => (adding = true)}>{t('Add a discipline…')}</button>
  {#if error && !adding}<p class="err">{error}</p>{/if}
{/snippet}

{#if adding}
  <div class="scrim" role="dialog" aria-modal="true" aria-label={t('Add a discipline')}>
    <div class="card">
      <h3>{t('Add a discipline')}</h3>
      {#if available.length > 0}
        <p class="label">{t('One of the usual')}</p>
        <div class="presets">
          {#each available as p (p)}
            <button type="button" disabled={busy} onclick={() => void add(p)}>{p}</button>
          {/each}
        </div>
        <p class="label">{t('Or another')}</p>
      {/if}
      <form
        onsubmit={(e) => {
          e.preventDefault();
          void add();
        }}
      >
        <input list="discipline-names" placeholder={t('Rapier and dagger, Sword and buckler, …')} bind:value={name} required />
        <button type="submit" class="go" disabled={busy || !name.trim()}>{t('Add')}</button>
      </form>
      {#if error}<p class="err">{error}</p>{/if}
      <p class="dim">{t('Each discipline gets its own competitors, pools and admin page, at this same address.')}</p>
      <button type="button" class="close" onclick={() => ((adding = false), (error = ''))}>{t('Cancel')}</button>
    </div>
  </div>
{/if}

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
  /* One style for every action on a row, link or button. */
  .act {
    display: inline-block;
    padding: 0.3rem 0.6rem;
    font-size: 0.82rem;
    line-height: 1.2;
    border: 1px solid var(--line);
    border-radius: 6px;
    background: var(--panel-2);
    color: var(--ink);
    text-decoration: none;
  }
  .act.danger {
    border-color: var(--red);
  }
  .acts {
    display: inline-flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.35rem;
  }
  .add {
    margin-top: 0.1rem;
  }
  .scrim {
    position: fixed;
    inset: 0;
    background: rgba(6, 8, 12, 0.86);
    display: grid;
    place-items: center;
    padding: 1rem;
    z-index: 60;
  }
  .card {
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: 14px;
    padding: 1.4rem;
    width: min(32rem, 100%);
    display: grid;
    gap: 0.6rem;
  }
  .card h3 {
    margin: 0;
    font-size: 1.2rem;
  }
  .card .presets button {
    padding: 0.45rem 0.7rem;
  }
  .go {
    font-weight: 700;
  }
  .close {
    justify-self: end;
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
  }
  .label {
    margin: 0;
    color: var(--ink-dim);
    font-size: 0.85rem;
  }
</style>
