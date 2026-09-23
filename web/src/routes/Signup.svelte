<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { api, type Snapshot, type SignupInfo, type SignupPreview, type SignupReady } from '../api';
  import { t } from '../lib/i18n.svelte';

  /**
   * Offline signup, the organizer's half (issue #91).
   *
   * One file goes out with the event written into it; a folder of files comes back and is
   * imported here. Nothing in between touches a network, which is the point: the project
   * exists so a club can run an event with no internet-facing server, and registration
   * was the last part that still meant typing forty names off a spreadsheet.
   *
   * Importing is two steps on screen because it is two calls on the server. Nothing is
   * written until the organizer has seen what would be, and the confirm re-checks rather
   * than trusting what this screen sends back.
   */
  let { snapshot, onchange }: { snapshot: Snapshot; onchange: () => void } = $props();

  const initial = untrack(() => snapshot.tournament.event?.signup ?? {});
  let definitionId = $state(initial.definitionId ?? '');
  let name = $state(initial.name ?? '');
  let venue = $state(initial.venue ?? '');
  let date = $state(initial.date ?? '');
  let tournament = $state(initial.tournament ?? '');
  let contact = $state(initial.contact ?? false);

  let ready = $state<SignupReady | null>(null);
  let saving = $state(false);
  let saved = $state(false);
  let error = $state('');

  // The chosen files and what the server says about them. Kept apart: the organizer can
  // look, change their mind, and pick a different folder without anything having
  // happened.
  let files = $state<{ source: string; body: string }[]>([]);
  let preview = $state<SignupPreview | null>(null);
  let busy = $state(false);
  let imported = $state<number | null>(null);

  async function refreshReady() {
    try {
      ready = await api.signupReady();
    } catch {
      // The panel still works; it just cannot say what is missing.
    }
  }

  onMount(refreshReady);

  function touch() {
    saved = false;
  }

  async function save() {
    error = '';
    saving = true;
    try {
      const signup: SignupInfo = { definitionId, name, venue, date, tournament, contact };
      await api.saveEvent({ ...(snapshot.tournament.event ?? {}), signup });
      saved = true;
      await refreshReady();
      onchange();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      saving = false;
    }
  }

  /**
   * Reading a folder of responses.
   *
   * `webkitdirectory` is what makes "point at the folder they are all in" work, and
   * unlike the File System Access API it is not gated on a secure context — which
   * matters, because every device but the organizer's own opens this over plain HTTP on
   * a LAN address (AGENTS.md). `multiple` is the fallback for picking files by hand.
   */
  async function choose(list: FileList | null) {
    if (!list) return;
    error = '';
    preview = null;
    imported = null;
    const picked: { source: string; body: string }[] = [];
    for (const file of Array.from(list)) {
      // A folder chosen wholesale brings whatever else is in it. Only the JSON is a
      // candidate, and the rest is not worth showing the organizer as forty errors.
      if (!file.name.toLowerCase().endsWith('.json')) continue;
      picked.push({ source: file.webkitRelativePath || file.name, body: await file.text() });
    }
    files = picked;
    if (picked.length === 0) {
      error = t('No .json files in there.');
      return;
    }
    await look();
  }

  async function look() {
    busy = true;
    try {
      preview = await api.previewSignups(files);
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      busy = false;
    }
  }

  async function confirm() {
    busy = true;
    error = '';
    try {
      const res = await api.importSignups(files);
      imported = res.added;
      preview = res.preview;
      onchange();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      busy = false;
    }
  }

  function clear() {
    files = [];
    preview = null;
    imported = null;
    error = '';
  }

  const verdictLabel: Record<string, string> = $derived({
    new: t('will be added'),
    already: t('already imported'),
    repeat: t('the same file twice'),
    'other-event': t('another event'),
    'not-here': t('another discipline'),
    unknown: t('unknown discipline'),
    invalid: t('not usable'),
  });

  // Counted for the summary line, so the organizer is not made to read forty rows to
  // find out that thirty-eight are fine.
  const counts = $derived.by(() => {
    const out: Record<string, number> = {};
    for (const row of preview?.rows ?? []) out[row.verdict] = (out[row.verdict] ?? 0) + 1;
    return out;
  });
</script>

<section>
  <h2>{t('Signup files')}</h2>
  <p class="dim">
    {t('Send people a file they fill in offline and send back. Nothing here needs the internet, and neither does what they open.')}
  </p>

  <div class="fields">
    <label>
      {t('Event identifier')}
      <input bind:value={definitionId} oninput={touch} placeholder="msl-open-2026" />
    </label>
    <label>
      {t('Event name')}
      <input bind:value={name} oninput={touch} placeholder={t('MSL Open')} />
    </label>
    <label>
      {t('Venue')}
      <input bind:value={venue} oninput={touch} placeholder={t('Linköping')} />
    </label>
    <label>
      {t('Date')}
      <input bind:value={date} oninput={touch} placeholder="2026-11-15" />
    </label>
  </div>
  <p class="dim small">
    {t('The identifier keeps a response from last year’s event out of this one. It goes in every file and cannot change once they are sent.')}
  </p>

  <div class="fields">
    <label>
      {t('This run is')}
      <select bind:value={tournament} onchange={touch}>
        <option value="">{t('the only discipline')}</option>
        {#each ready?.tournaments ?? [] as tn (tn.id)}
          <option value={tn.id}>{tn.label}</option>
        {/each}
      </select>
    </label>
    <label class="check">
      <input type="checkbox" bind:checked={contact} onchange={touch} />
      {t('Ask for a contact detail')}
    </label>
  </div>
  <p class="dim small">
    {t('An event with several disciplines is several runs of the application. Each imports the responses naming its own, so you can point all of them at the same folder.')}
  </p>

  {#if error}<p class="err">{error}</p>{/if}
  <div class="actions">
    <button class="save" disabled={saving} onclick={save}>{t('Save')}</button>
    {#if saved}<span class="ok">{t('Saved')}</span>{/if}
  </div>

  {#if ready && (ready.missing?.length ?? 0) > 0}
    <p class="warn">
      {t('Before the files can go out, this still needs:')}
      {ready.missing.join(', ')}.
    </p>
  {:else if ready}
    <div class="send">
      <h3>{t('Send this out')}</h3>
      <p class="dim small">
        {t('One file, with the event already in it. They open it, fill it in and send back a small .json.')}
      </p>
      <p class="row">
        <a class="button" href="/api/signup/app.html?download=1">{t('The signup app')}</a>
        <a class="quiet" href="/api/signup/app.html" target="_blank" rel="noreferrer">{t('Preview it')}</a>
        <a class="quiet" href="/api/signup/definition.json?download=1">{t('Just the definition')}</a>
      </p>
    </div>
  {/if}

  <div class="import">
    <h3>{t('Bring responses in')}</h3>
    <p class="row">
      <label class="button">
        {t('Choose a folder')}
        <!-- webkitdirectory is what makes picking a whole folder work, and it needs no
             secure context, unlike the File System Access API. -->
        <input
          type="file"
          multiple
          webkitdirectory
          onchange={(e) => void choose(e.currentTarget.files)}
        />
      </label>
      <label class="quiet">
        {t('or pick files')}
        <input
          type="file"
          multiple
          accept=".json,application/json"
          onchange={(e) => void choose(e.currentTarget.files)}
        />
      </label>
      {#if files.length > 0}
        <button class="quiet" onclick={clear}>{t('Clear')}</button>
      {/if}
    </p>

    {#if busy}
      <p class="dim">{t('Reading…')}</p>
    {/if}

    {#if preview}
      {#if imported !== null}
        <p class="ok big">
          {imported === 1
            ? t('1 competitor imported.')
            : t('{n} competitors imported.', { n: imported })}
        </p>
      {/if}

      <p class="summary">
        {#each Object.entries(counts) as [verdict, n] (verdict)}
          <span class="count {verdict}">{n} {verdictLabel[verdict]}</span>
        {/each}
      </p>

      {#each preview.capacity ?? [] as warning (warning)}
        <p class="warn">{warning}</p>
      {/each}
      {#if preview.poolsDrawn && preview.adding > 0}
        <p class="warn">
          {t('The pools are already drawn. Anyone imported now will not be in one until you draw again.')}
        </p>
      {/if}

      <div class="scroller">
        <table>
          <thead>
            <tr>
              <th class="l">{t('Name')}</th>
              <th class="l">{t('Club')}</th>
              <th class="l">{t('Entering')}</th>
              <th class="l">{t('File')}</th>
              <th class="l">{t('What happens')}</th>
            </tr>
          </thead>
          <tbody>
            {#each preview.rows as row (row.source + row.submissionId)}
              <tr class={row.verdict}>
                <td class="l strong">{row.name || '—'}</td>
                <td class="l">{row.club || ''}</td>
                <td class="l">{(row.entries ?? []).join(', ')}</td>
                <td class="l file">{row.source}</td>
                <td class="l">
                  <span class="verdict {row.verdict}">{verdictLabel[row.verdict]}</span>
                  {#if row.problem}<span class="dim problem">{row.problem}</span>{/if}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>

      {#if imported === null}
        <div class="actions">
          <button class="save" disabled={busy || preview.adding === 0} onclick={confirm}>
            {preview.adding === 1
              ? t('Import 1 competitor')
              : t('Import {n} competitors', { n: preview.adding })}
          </button>
          {#if preview.adding === 0}
            <span class="dim">{t('Nothing in there to add.')}</span>
          {/if}
        </div>
      {/if}
    {/if}
  </div>
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
  h3 {
    margin: 0 0 0.4rem;
    font-size: 0.72rem;
    font-weight: 800;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--ink-dim);
  }
  .dim {
    color: var(--ink-dim);
    line-height: 1.5;
    font-size: 0.9rem;
    margin: 0 0 0.9rem;
  }
  .small {
    font-size: 0.82rem;
  }
  .fields {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
    gap: 0.6rem;
    margin-bottom: 0.6rem;
  }
  .fields label {
    display: grid;
    gap: 0.3rem;
    font-size: 0.85rem;
    color: var(--ink-dim);
    min-width: 0;
  }
  .fields input,
  .fields select {
    padding: 0.4rem 0.5rem;
    min-width: 0;
  }
  .fields label.check {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    align-self: end;
    padding-bottom: 0.5rem;
  }
  .fields label.check input {
    width: 1.1rem;
    height: 1.1rem;
  }

  .send,
  .import {
    margin-top: 1.2rem;
    padding-top: 1rem;
    border-top: 1px solid var(--line);
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    align-items: center;
    margin: 0 0 0.6rem;
  }
  /* The file inputs are hidden inside their labels: a bare file input cannot be styled
     and reads as an afterthought on a page of buttons. */
  .button,
  .quiet {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.5rem 0.9rem;
    border-radius: var(--radius);
    font-size: 0.9rem;
    font-weight: 700;
    cursor: pointer;
    text-decoration: none;
  }
  .button {
    background: var(--ink);
    color: #0d0f14;
    border: none;
  }
  .quiet {
    background: var(--panel-2);
    color: var(--ink);
    border: 1px solid var(--line);
    font-weight: 600;
  }
  .button input,
  .quiet input {
    display: none;
  }

  .actions {
    display: flex;
    align-items: center;
    gap: 0.8rem;
    margin-top: 0.9rem;
    flex-wrap: wrap;
  }
  .save {
    padding: 0.55rem 1rem;
    font-weight: 700;
    background: var(--ink);
    color: #0d0f14;
    border: none;
  }
  .save:disabled {
    opacity: 0.5;
  }
  .ok {
    color: var(--ok);
    font-size: 0.88rem;
  }
  .ok.big {
    font-size: 1rem;
    font-weight: 700;
  }
  .err,
  .warn {
    margin: 0.5rem 0 0;
    color: var(--amber-bright);
    line-height: 1.5;
    font-size: 0.9rem;
  }

  .summary {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem 1rem;
    margin: 0.8rem 0 0.4rem;
    font-size: 0.85rem;
    color: var(--ink-dim);
  }
  .count.new {
    color: var(--ok);
    font-weight: 700;
  }

  .scroller {
    overflow-x: auto;
    margin-top: 0.6rem;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.85rem;
    min-width: 30rem;
  }
  th,
  td {
    padding: 0.35rem 0.4rem;
    text-align: left;
    border-bottom: 1px solid var(--line);
    vertical-align: top;
  }
  th {
    color: var(--ink-dim);
    font-weight: 500;
    font-size: 0.72rem;
  }
  .l {
    text-align: left;
  }
  .strong {
    font-weight: 700;
  }
  .file {
    color: var(--ink-dim);
    font-size: 0.78rem;
    overflow-wrap: anywhere;
  }
  /* A row that will not be imported is dimmed rather than coloured: most of them are not
     errors, they are somebody else's discipline or a file already dealt with. */
  tr.already,
  tr.repeat,
  tr.not-here {
    color: var(--ink-dim);
  }
  .verdict {
    display: block;
    font-size: 0.78rem;
  }
  .verdict.new {
    color: var(--ok);
    font-weight: 700;
  }
  .verdict.invalid,
  .verdict.unknown,
  .verdict.other-event {
    color: var(--amber-bright);
  }
  .problem {
    display: block;
    font-size: 0.75rem;
    overflow-wrap: anywhere;
  }
</style>
