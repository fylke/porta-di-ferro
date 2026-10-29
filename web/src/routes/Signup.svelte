<script lang="ts">
  import { onMount, untrack } from "svelte";
  import {
    api,
    type EventInfo,
    type EventSignupPreview,
    type EventSignupReady,
    type EventSignupRow,
    type Snapshot,
    type SignupInfo,
    type SignupPreview,
    type SignupReady,
    type SignupUpload,
    type StaffMember,
  } from "../api";
  import { t } from "../lib/i18n.svelte";
  import { apiBase } from "../lib/paths";
  import { discipline } from "../router.svelte";
  import SaveButton from "./SaveButton.svelte";

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
  /**
   * Where it runs. 'single': a discipline on its own, or an event's only one, as it always
   * was. 'event': the event's admin, where one import gives every discipline its share
   * (phase 3). 'share': a discipline of an event with several, which keeps its staff and
   * points at the event's import.
   */
  let {
    snapshot,
    info,
    onchange,
    scope = "single",
  }: {
    snapshot?: Snapshot;
    /** The event's day, for the event's panel. */
    info?: EventInfo;
    onchange: () => void;
    scope?: "single" | "event" | "share";
  } = $props();

  // The files are the event's, or this discipline's: the definition, and the app with it
  // baked in.
  // svelte-ignore state_referenced_locally
  const base = scope === "event" ? "/api/event" : apiBase(discipline());
  // svelte-ignore state_referenced_locally
  const several = scope === "event";

  const initial = untrack(
    () =>
      (scope === "event" ? info?.signup : snapshot?.tournament.event?.signup) ??
      {},
  );
  let definitionId = $state(initial.definitionId ?? "");
  let name = $state(initial.name ?? "");
  let venue = $state(initial.venue ?? "");
  let date = $state(initial.date ?? "");

  let ready = $state<SignupReady | EventSignupReady | null>(null);
  // What a participant chooses from: the disciplines themselves, by their names (#142).
  const offered = $derived(ready?.tournaments ?? []);
  let saving = $state(false);
  let dirty = $state(false);
  let error = $state("");
  // When each discipline starts at the earliest, by slug: the event's plan (#149).
  let starts = $state<Record<string, string>>({});

  // The chosen files and what the server says about them. Kept apart: the organizer can
  // look, change their mind, and pick a different folder without anything having
  // happened.
  let files = $state<SignupUpload[]>([]);
  let preview = $state<SignupPreview | EventSignupPreview | null>(null);
  const rows: EventSignupRow[] = $derived(preview?.rows ?? []);
  // Over capacity, and imported after the draw: per discipline at the event's import.
  const capacity = $derived(
    !preview
      ? []
      : "disciplines" in preview
        ? preview.disciplines.flatMap((d) => d.capacity ?? [])
        : (preview.capacity ?? []),
  );
  const drawnInto = $derived(
    !preview
      ? []
      : "disciplines" in preview
        ? preview.disciplines
            .filter((d) => d.poolsDrawn && d.adding > 0)
            .map((d) => d.name)
        : preview.poolsDrawn && preview.adding > 0
          ? [""]
          : [],
  );
  let busy = $state(false);
  let imported = $state<{ competitors: number; staff: number } | null>(null);

  // Who has offered to work this discipline rather than fence in it (issue #5).
  const staff: StaffMember[] = $derived(snapshot?.tournament.staff ?? []);

  async function refreshReady() {
    if (scope === "share") return;
    try {
      ready =
        scope === "event"
          ? await api.eventSignupReady()
          : await api.signupReady();
      if (scope === "event")
        starts = { ...((await api.forecast()).starts ?? {}) };
    } catch {
      // The panel still works; it just cannot say what is missing.
    }
  }

  // The event's disciplines, edited where they are listed (#149): the name, and when it
  // starts at the earliest -- shown as the planned start until one is given.
  async function rename(slug: string, to: string) {
    error = "";
    try {
      await api.renameDiscipline(slug, to);
      await refreshReady();
      onchange();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }
  async function setStart(slug: string, at: string) {
    error = "";
    try {
      starts = { ...((await api.setStarts({ [slug]: at })).starts ?? {}) };
      await refreshReady();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }

  onMount(refreshReady);

  function touch() {
    dirty = true;
  }

  async function save() {
    error = "";
    saving = true;
    try {
      if (scope === "event") {
        await api.saveEventInfo({
          ...(info ?? {}),
          signup: { definitionId, name, venue, date },
        });
      } else {
        const signup: SignupInfo = { definitionId, name, venue, date };
        await api.saveEvent({ ...(snapshot?.tournament.event ?? {}), signup });
      }
      dirty = false;
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
    error = "";
    preview = null;
    imported = null;
    files = [];
    const picked: SignupUpload[] = [];
    try {
      for (const file of Array.from(list)) {
        const pdf = file.name.toLowerCase().endsWith(".pdf");
        if (!pdf && !file.name.toLowerCase().endsWith(".json")) continue;
        const source = file.webkitRelativePath || file.name;
        if (pdf) {
          try {
            const { pdfSignupJSON } = await import("../lib/signup-pdf");
            const body = await pdfSignupJSON(await file.arrayBuffer());
            picked.push({ source, body });
          } catch (e) {
            throw new Error(
              `${t("Could not read {file}.", { file: file.name })} ${e instanceof Error ? e.message : String(e)}`,
            );
          }
        } else {
          picked.push({ source, body: await file.text() });
        }
      }
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
      return;
    }
    files = picked;
    if (picked.length === 0) {
      error = t("No .json or .pdf files in there.");
      return;
    }
    await look();
  }

  async function look() {
    busy = true;
    try {
      preview =
        scope === "event"
          ? await api.previewEventSignups(files)
          : await api.previewSignups(files);
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      busy = false;
    }
  }

  async function confirm() {
    busy = true;
    error = "";
    try {
      const res =
        scope === "event"
          ? await api.importEventSignups(files)
          : await api.importSignups(files);
      imported = { competitors: res.added, staff: res.addedStaff };
      preview = res.preview;
      if ("error" in res && res.error) error = res.error;
      onchange();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      busy = false;
    }
  }

  async function removeStaff(id: string) {
    error = "";
    try {
      await api.removeStaff(id);
      onchange();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }

  function clear() {
    files = [];
    preview = null;
    imported = null;
    error = "";
  }

  const verdictLabel: Record<string, string> = $derived({
    new: t("will be added"),
    staff: t("will be added as staff"),
    already: t("already imported"),
    repeat: t("the same file twice"),
    "other-event": t("another event"),
    "not-here": several ? t("no discipline takes it") : t("another discipline"),
    unknown: t("unknown discipline"),
    invalid: t("not usable"),
  });

  const roleLabel: Record<string, string> = $derived({
    "head-ref": t("Head referee"),
    "assistant-ref": t("Assistant referee"),
    "score-keeper": t("Score keeper"),
    physician: t("Physician"),
  });

  function rolesText(roles: string[] | undefined): string {
    return (roles ?? []).map((r) => roleLabel[r] ?? r).join(", ");
  }

  /** "2 competitors and 1 staff member", leaving out whichever is none. */
  function howMany(competitors: number, staffCount: number): string {
    const parts: string[] = [];
    if (competitors > 0 || staffCount === 0) {
      // The event's import counts entries: one response entering two disciplines is two
      // of them and one person (#128).
      if (scope === "event")
        parts.push(
          competitors === 1
            ? t("1 entry")
            : t("{n} entries", { n: competitors }),
        );
      else
        parts.push(
          competitors === 1
            ? t("1 competitor")
            : t("{n} competitors", { n: competitors }),
        );
    }
    if (staffCount > 0) {
      parts.push(
        staffCount === 1
          ? t("1 staff member")
          : t("{n} staff", { n: staffCount }),
      );
    }
    return parts.length === 2
      ? t("{a} and {b}", { a: parts[0], b: parts[1] })
      : parts[0];
  }

  // Counted for the summary line, so the organizer is not made to read forty rows to
  // find out that thirty-eight are fine.
  const counts = $derived.by(() => {
    const out: Record<string, number> = {};
    for (const row of rows) out[row.verdict] = (out[row.verdict] ?? 0) + 1;
    return out;
  });

  /** Where a response goes, at the event's import. */
  function into(row: EventSignupRow): string {
    return (row.into ?? [])
      .map((i) =>
        i.verdict === "staff"
          ? t("{discipline} as staff", { discipline: i.name })
          : i.name,
      )
      .join(", ");
  }
</script>

<section>
  <h2>{t("Signup files")}</h2>
  {#if scope === "share"}
    <p class="dim">
      {t("This event takes its signups once, for every discipline:")}
      <a href="/admin">{t("on the event’s admin page")}</a>.
    </p>
  {:else}
    <p class="dim">
      {t(
        "Send people a file they fill in offline and send back. Nothing here needs the internet, and neither does what they open.",
      )}
    </p>

    {#if several}
      <!-- The event's name, venue, date and identifier are set in Plan the event (#149). -->
      <p class="identity">
        <span class="strong">{info?.signup?.name || t("No name yet")}</span>
        {#if info?.signup?.venue}&middot; {info.signup.venue}{/if}
        {#if info?.signup?.date}&middot; {info.signup.date}{/if}
        {#if info?.signup?.definitionId}&middot; <span class="mono"
            >{info.signup.definitionId}</span
          >{/if}
        <a href="/plan?step=1">{t("Edit")}</a>
      </p>

      <!-- What goes out is the disciplines themselves, under the names they have here:
         there is nothing to match up (#142). Listed as the event is, with their fields
         (#149); the ruleset will join them. -->
      <div class="offered">
        <h3>{t("Disciplines")}</h3>
        {#if offered.length > 0}
          <div class="drow head" aria-hidden="true">
            <span>{t("Identifier")}</span><span>{t("Name")}</span><span
              >{t("Time")}</span
            >
          </div>
          {#each offered as d (d.id)}
            <div class="drow">
              <input
                class="mono"
                value={d.id}
                readonly
                aria-label={t("Identifier of {name}", { name: d.label })}
                title={t(
                  "The discipline’s address, and its name in the signup files. It does not change.",
                )}
              />
              <input
                value={d.label}
                aria-label={t("Name of {name}", { name: d.label })}
                onchange={(e) => void rename(d.id, e.currentTarget.value)}
              />
              <input
                type="time"
                value={starts[d.id] || d.startsAt || ""}
                aria-label={t("When {name} starts", { name: d.label })}
                onchange={(e) => void setStart(d.id, e.currentTarget.value)}
              />
            </div>
          {/each}
        {/if}
        <p class="dim small">
          {t(
            "A time is when the discipline starts at the earliest; until one is given it is the planned start. Name, club and email are asked of everybody; a second signup with the same email is the same person.",
          )}
        </p>
      </div>
      {#if error}<p class="err">{error}</p>{/if}
    {:else}
      <div class="fields">
        <label>
          {t("Event identifier")}
          <input
            bind:value={definitionId}
            oninput={touch}
            placeholder="msl-open-2026"
          />
        </label>
        <label>
          {t("Event name")}
          <input
            bind:value={name}
            oninput={touch}
            placeholder={t("MSL Open")}
          />
        </label>
        <label>
          {t("Venue")}
          <input
            bind:value={venue}
            oninput={touch}
            placeholder={t("Linköping")}
          />
        </label>
        <label>
          {t("Date")}
          <input bind:value={date} oninput={touch} placeholder="2026-11-15" />
        </label>
      </div>
      <p class="dim small">
        {t(
          "The identifier keeps a response from last year’s event out of this one. It goes in every file and cannot change once they are sent.",
        )}
      </p>

      <!-- What goes out is the disciplines themselves, under the names they have here: there
       is nothing to match up (#142). Everybody gives an email, which is how a second
       signup from the same person, from any device, is the same person. -->
      <div class="offered">
        <h3>{t("Disciplines")}</h3>
        {#if offered.length > 0}
          <ul>
            {#each offered as d (d.id)}
              <li>
                <span class="strong">{d.label}</span>
                {#if d.startsAt}<span class="dim-inline">{d.startsAt}</span
                  >{/if}
                {#if d.capacity}<span class="dim-inline"
                    >{t("up to {n}", { n: d.capacity })}</span
                  >{/if}
              </li>
            {/each}
          </ul>
        {/if}
        <p class="dim small">
          {t(
            "Every discipline with a name. Name, club and email are asked of everybody; a second signup with the same email is the same person.",
          )}
        </p>
      </div>

      {#if error}<p class="err">{error}</p>{/if}
      <div class="actions">
        <SaveButton {dirty} busy={saving} onclick={() => void save()} />
      </div>
    {/if}

    {#if ready && (ready.missing?.length ?? 0) > 0}
      <p class="warn">
        {t("Before the files can go out, this still needs:")}
        {ready.missing.join(", ")}.
      </p>
    {:else if ready}
      <div class="send">
        <h3>{t("Send this out")}</h3>
        <p class="dim small">
          {t(
            "One file, with the event already in it. They open it, fill it in and send back a small .json.",
          )}
        </p>
        <p class="row">
          <a class="button" href="{base}/signup/app.html?download=1"
            >{t("The signup app")}</a
          >
          <a class="button" href="{base}/signup/form.pdf">{t("Fillable PDF")}</a
          >
          <a
            class="quiet"
            href="{base}/signup/app.html"
            target="_blank"
            rel="noreferrer">{t("Preview it")}</a
          >
          <a class="quiet" href="{base}/signup/definition.json?download=1"
            >{t("Just the definition")}</a
          >
        </p>
      </div>
    {/if}

    <div class="import">
      <h3>{t("Bring responses in")}</h3>
      <p class="row">
        <label class="button">
          {t("Choose a folder")}
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
          {t("or pick files")}
          <input
            type="file"
            multiple
            accept=".json,.pdf,application/json,application/pdf"
            onchange={(e) => void choose(e.currentTarget.files)}
          />
        </label>
        {#if files.length > 0}
          <button class="quiet" onclick={clear}>{t("Clear")}</button>
        {/if}
      </p>

      {#if busy}
        <p class="dim">{t("Reading…")}</p>
      {/if}

      {#if preview}
        {#if imported !== null}
          <p class="ok big">
            {t("Imported: {what}.", {
              what: howMany(imported.competitors, imported.staff),
            })}
          </p>
        {/if}

        <p class="summary">
          {#each Object.entries(counts) as [verdict, n] (verdict)}
            <span class="count {verdict}">{n} {verdictLabel[verdict]}</span>
          {/each}
        </p>

        {#each capacity as warning (warning)}
          <p class="warn">{warning}</p>
        {/each}
        {#if drawnInto.length > 0 && !several}
          <p class="warn">
            {t(
              "The pools are already drawn. Anyone imported now will not be in one until you draw again.",
            )}
          </p>
        {:else if drawnInto.length > 0}
          <p class="warn">
            {t(
              "The pools are already drawn in {disciplines}. Anyone imported there will not be in one until you draw again.",
              { disciplines: drawnInto.join(", ") },
            )}
          </p>
        {/if}

        <div class="scroller">
          <table>
            <thead>
              <tr>
                <th class="l">{t("Name")}</th>
                <th class="l">{t("Club")}</th>
                <th class="l">{t("Entering")}</th>
                <th class="l">{t("File")}</th>
                <th class="l">{t("What happens")}</th>
              </tr>
            </thead>
            <tbody>
              {#each rows as row (row.source + row.submissionId)}
                <tr class={row.verdict}>
                  <td class="l strong">{row.name || "—"}</td>
                  <td class="l">{row.club || ""}</td>
                  <td class="l">{(row.entries ?? []).join(", ")}</td>
                  <td class="l file">{row.source}</td>
                  <td class="l">
                    <span class="verdict {row.verdict}"
                      >{verdictLabel[row.verdict]}</span
                    >
                    {#if several && (row.into?.length ?? 0) > 0}
                      <span class="dim problem"
                        >{t("into {where}", { where: into(row) })}</span
                      >
                    {/if}
                    {#if row.verdict === "staff"}
                      <span class="dim problem"
                        >{t("as {roles}", {
                          roles: rolesText(row.roles),
                        })}</span
                      >
                    {/if}
                    {#if row.problem}<span class="dim problem"
                        >{row.problem}</span
                      >{/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>

        {#if imported === null}
          <div class="actions">
            <button
              class="save"
              disabled={busy || preview.adding + preview.addingStaff === 0}
              onclick={confirm}
            >
              {t("Import {what}", {
                what: howMany(preview.adding, preview.addingStaff),
              })}
            </button>
            {#if preview.adding + preview.addingStaff === 0}
              <span class="dim">{t("Nothing in there to add.")}</span>
            {/if}
          </div>
        {/if}
      {/if}
    </div>
  {/if}

  {#if staff.length > 0}
    <div class="staff-list">
      <h3>{t("Staff")}</h3>
      <p class="dim small">
        {t(
          "Offered to work this discipline. Who stands where is still up to you.",
        )}
      </p>
      <ul>
        {#each staff as member (member.id)}
          <li>
            <span class="who">
              <span class="strong">{member.name}</span>
              {#if member.club}<span class="dim-inline">{member.club}</span
                >{/if}
            </span>
            <span class="roles">{rolesText(member.roles)}</span>
            <button class="quiet" onclick={() => void removeStaff(member.id)}
              >{t("Remove")}</button
            >
          </li>
        {/each}
      </ul>
    </div>
  {/if}
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
  .fields input {
    padding: 0.4rem 0.5rem;
    min-width: 0;
  }

  .offered ul {
    list-style: none;
    margin: 0 0 0.5rem;
    padding: 0;
    display: grid;
    gap: 0.3rem;
    font-size: 0.9rem;
  }
  .offered li {
    display: flex;
    flex-wrap: wrap;
    gap: 0.2rem 0.6rem;
    align-items: baseline;
  }
  .offered,
  .send,
  .import,
  .staff-list {
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
  /* The links and file pickers, as the admin's buttons look (#149). */
  .button,
  .quiet {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    min-height: 2.25rem;
    padding: 0.4rem 0.9rem;
    border-radius: 8px;
    font-size: 0.9rem;
    font-weight: 600;
    line-height: 1.2;
    cursor: pointer;
    text-decoration: none;
  }
  .button {
    background: var(--ink);
    color: #0d0f14;
    border: 1px solid var(--ink);
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
  .identity {
    display: flex;
    flex-wrap: wrap;
    gap: 0.3rem 0.5rem;
    align-items: baseline;
    margin: 0 0 0.6rem;
  }
  .drow {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1.4fr) 7rem;
    gap: 0.4rem;
    margin-bottom: 0.35rem;
  }
  .drow input {
    min-width: 0;
    padding: 0.35rem 0.5rem;
    font-size: 0.9rem;
  }
  .drow input[readonly] {
    color: var(--ink-dim);
    background: transparent;
  }
  .drow.head {
    font-size: 0.72rem;
    color: var(--ink-dim);
    margin-bottom: 0.2rem;
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
  .count.new,
  .count.staff {
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
  .verdict.new,
  .verdict.staff {
    color: var(--ok);
    font-weight: 700;
  }
  .verdict.invalid,
  .verdict.unknown,
  .verdict.other-event {
    color: var(--amber-bright);
  }
  .staff-list ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .staff-list li {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.3rem 0.8rem;
    padding: 0.4rem 0;
    border-bottom: 1px solid var(--line);
    font-size: 0.9rem;
  }
  .staff-list .who {
    flex: 1 1 10rem;
    min-width: 0;
  }
  .staff-list .roles {
    color: var(--ink-dim);
    font-size: 0.82rem;
  }
  .dim-inline {
    color: var(--ink-dim);
    margin-left: 0.4rem;
  }
  .problem {
    display: block;
    font-size: 0.75rem;
    overflow-wrap: anywhere;
  }
</style>
