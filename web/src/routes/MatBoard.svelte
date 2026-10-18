<script lang="ts" module>
  /** Moves not applied yet, kept while the board is off screen -- a tab away, say. */
  let kept: { id: string; mat: number; index: number }[] = [];
</script>

<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError, type ForecastView, type ItemView, type StaffingView, type SuggestionView, type Warning } from '../api';
  import { roleLabel } from '../lib/items';
  import { MatsLive } from '../lib/mats.svelte';
  import { clockOf, minutesBetween } from '../lib/clock-of-day';
  import { roundLabel } from './lib-display.svelte';
  import { t } from '../lib/i18n.svelte';
  import DayTimeline from './DayTimeline.svelte';

  /**
   * The mat board (docs/proposals/one-event-many-disciplines.md §10, phase 2; #101): the
   * hall's mats side by side, and every discipline's work items on them as cards in the
   * order each mat runs them.
   *
   * A card moves two ways. The menu on each card moves it earlier, later or to another mat,
   * and works from a keyboard and one-handed on a tablet. Or the card is dragged, by any
   * part of it (#142): pointer events rather than HTML5 drag-and-drop, which does not fire
   * on touch. With a mouse the drag starts as the card moves; with a finger, after holding
   * it still for a moment, so a swipe across the board still scrolls the page.
   *
   * Moves are a draft until Apply (#142): the board shows them at once, and nothing is sent
   * until the organizer has tried a few and made up their mind; Discard puts it back. Each
   * move is "this item, to this mat, at this place", decided against the board as it then
   * looked, and applied in order. An item under way or finished cannot be moved, and nothing
   * goes in front of it -- here as on the server, which has the last word.
   *
   * The forecast (phase 4) is on every card: when it is expected to run, and how far that
   * has drifted from the plan. A card moved by hand is pinned, so a suggested plan keeps it
   * on its mat; the menu pins and unpins, and holds an item until a time. Suggest a plan
   * shows what would move and when the day would end, and changes nothing until applied.
   */
  let {
    canSetCount = false,
    onshort,
  }: {
    canSetCount?: boolean;
    /** Opens the staff, from a slot short of them (#142). Without it, a link to #staff. */
    onshort?: () => void;
  } = $props();

  const live = new MatsLive();
  onMount(() => {
    void live.start();
    return () => live.stop();
  });

  const view = $derived(live.view);
  const mats = $derived(view?.mats ?? []);

  // The draft: moves made on the board and not sent yet, in the order they were made.
  let draft = $state<{ id: string; mat: number; index: number }[]>(kept);
  $effect(() => {
    kept = draft;
  });

  /** The board as the server has it, then with the draft's moves made on it. */
  const byMat = $derived.by(() => {
    const out = new Map<number, ItemView[]>();
    for (const m of mats) out.set(m.mat, []);
    for (const it of view?.items ?? []) out.get(it.mat)?.push(it);
    for (const list of out.values()) list.sort((a, b) => a.position - b.position);
    for (const mv of draft) {
      const target = out.get(mv.mat);
      let it: ItemView | undefined;
      for (const list of out.values()) {
        const i = list.findIndex((o) => o.id === mv.id);
        if (i >= 0) [it] = list.splice(i, 1);
      }
      if (!it) continue;
      if (!it.movable || !target) {
        // It started since, or its mat is gone: it stays where the server has it.
        out.get(it.mat)?.splice(it.position - 1, 0, it);
        continue;
      }
      // Never in front of what a mat has under way or finished.
      let floor = 0;
      while (floor < target.length && !target[floor].movable) floor++;
      target.splice(Math.min(target.length, Math.max(floor, mv.index)), 0, it);
    }
    return out;
  });
  const moved = $derived(new Set(draft.map((m) => m.id)));

  /** Where a card is on the board as drafted: its mat and its place among the others. */
  function placeOf(id: string): { mat: number; index: number } | null {
    for (const [mat, list] of byMat) {
      const i = list.findIndex((o) => o.id === id);
      if (i >= 0) return { mat, index: i };
    }
    return null;
  }

  /** Adds a move to the draft, unless it leaves the card where it is. */
  function plan(it: ItemView, mat: number, index: number) {
    menuFor = null;
    const at = placeOf(it.id);
    if (at && at.mat === mat && at.index === index) return;
    draft = [...draft, { id: it.id, mat, index }];
  }

  async function applyDraft() {
    const moves = draft;
    error = '';
    busy = true;
    let done = 0;
    try {
      for (const m of moves) {
        await api.moveItem(m.id, { mat: m.mat, index: m.index });
        done++;
      }
      draft = [];
    } catch (e) {
      const why = e instanceof Error ? e.message : String(e);
      error = done > 0 ? t('{done} of {n} moves applied, then: {why}', { done, n: moves.length, why }) : why;
      draft = moves.slice(done);
    } finally {
      busy = false;
      await live.refresh();
    }
  }
  function discard() {
    draft = [];
  }
  const disciplines = $derived(new Set((view?.items ?? []).map((it) => it.discipline)).size);

  let error = $state('');
  let busy = $state(false);
  let menuFor = $state<string | null>(null);
  let holdUntil = $state('');

  // The forecast, again whenever the mats change: a finished match moves the day. The
  // staff with it, because who is free when moves with the day.
  let forecast = $state<ForecastView | null>(null);
  let staffing = $state<StaffingView | null>(null);
  async function loadForecast() {
    try {
      forecast = await api.forecast();
    } catch {
      // The board works without times.
    }
    try {
      staffing = await api.staff();
    } catch {
      // And without the staff.
    }
  }
  $effect(() => {
    void live.view;
    void loadForecast();
  });

  /**
   * The roles each item is short of, by item (#142). Only once the event has staff: an
   * event that does not plan them would otherwise be short of everybody, everywhere.
   */
  const shortOf = $derived.by(() => {
    const out = new Map<string, string[]>();
    if (!staffing || staffing.members.length === 0) return out;
    for (const s of staffing.short) {
      const roles = out.get(s.item) ?? [];
      const name = roleLabel(s.role);
      if (!roles.includes(name)) roles.push(name);
      out.set(s.item, roles);
    }
    return out;
  });
  function toStaff(e: MouseEvent) {
    if (!onshort) return;
    e.preventDefault();
    onshort();
  }
  const times = $derived(new Map((forecast?.items ?? []).map((x) => [x.id, x])));
  const itemById = $derived(new Map((view?.items ?? []).map((x) => [x.id, x])));

  /** "10:40–11:25". */
  function span(it: ItemView): string {
    const x = times.get(it.id);
    if (!x || !x.start) return '';
    return `${clockOf(x.start)}–${clockOf(x.end)}`;
  }
  /** How many minutes later than planned the item is forecast to start. */
  function lateBy(it: ItemView): number {
    const x = times.get(it.id);
    return x ? minutesBetween(x.plannedStart, x.start) : 0;
  }
  function itemName(id: string): string {
    const it = itemById.get(id);
    return it ? `${it.disciplineName} ${label(it).toLocaleLowerCase()}` : id;
  }
  function warningText(w: Warning): string {
    switch (w.kind) {
      case 'overlap':
        return t('{name} is in {a} and {b}, which overlap {from}–{to}.', {
          name: w.personName || t('Somebody'),
          a: itemName(w.items?.[0] ?? ''),
          b: itemName(w.items?.[1] ?? ''),
          from: clockOf(w.from),
          to: clockOf(w.to),
        });
      case 'dependency':
        return t('{item} is placed before what it waits for.', { item: itemName(w.items?.[0] ?? '') });
      case 'overrun':
        return t('The day ends at {end}, after the venue closes at {close}.', { end: clockOf(w.to), close: clockOf(w.from) });
    }
  }

  // --- suggestions ------------------------------------------------------------------------

  let suggestion = $state<SuggestionView | null>(null);
  let suggestNote = $state('');
  async function suggest() {
    error = '';
    busy = true;
    try {
      suggestion = await api.suggest();
      suggestNote = '';
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      busy = false;
    }
  }
  async function applySuggestion() {
    if (!suggestion) return;
    error = '';
    busy = true;
    try {
      forecast = await api.applySuggestion(suggestion.signature);
      suggestion = null;
      await live.refresh();
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        suggestion = (e.body as { suggestion?: SuggestionView }).suggestion ?? null;
        suggestNote = t('The plan changed since; this is a new suggestion.');
      } else {
        error = e instanceof Error ? e.message : String(e);
      }
    } finally {
      busy = false;
    }
  }

  function pin(it: ItemView, pinned: boolean) {
    menuFor = null;
    void send(() => api.flagItem(it.id, { pinned }));
  }
  function hold(it: ItemView, notBefore: string) {
    menuFor = null;
    void send(() => api.flagItem(it.id, { notBefore }));
  }
  function openMenu(it: ItemView) {
    menuFor = menuFor === it.id ? null : it.id;
    holdUntil = it.notBefore ?? '';
  }

  async function send(fn: () => Promise<unknown>) {
    error = '';
    busy = true;
    try {
      await fn();
      await live.refresh();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      busy = false;
    }
  }

  function moveTo(it: ItemView, mat: number) {
    const others = (byMat.get(mat) ?? []).filter((o) => o.id !== it.id);
    plan(it, mat, others.length);
  }
  function step(it: ItemView, move: 'up' | 'down') {
    const at = placeOf(it.id);
    if (!at) return;
    plan(it, at.mat, Math.max(0, at.index + (move === 'up' ? -1 : 1)));
  }
  function setCount(n: number) {
    void send(() => api.setMats(n));
  }

  /** "Pool 3", "Eliminations 2", "Bronze match", "Final". */
  function label(it: ItemView): string {
    switch (it.kind) {
      case 'pool':
        return t('Pool {n}', { n: it.number ?? '' });
      case 'eliminations': {
        const lanes = (view?.items ?? []).filter((o) => o.discipline === it.discipline && o.kind === 'eliminations').length;
        return lanes > 1 ? t('Eliminations {n}', { n: it.number ?? '' }) : t('Eliminations');
      }
      case 'bronze':
        return t('Bronze match');
      case 'final':
        return t('Final');
    }
  }

  function statusLabel(it: ItemView): string {
    switch (it.status) {
      case 'running':
        return t('under way');
      case 'done':
        return t('done');
      case 'waiting':
        return t('waiting for results');
      case 'planned':
        return t('not drawn yet');
      case 'queued':
        return t('after the block before');
      default:
        return '';
    }
  }

  // --- dragging -------------------------------------------------------------------------

  let pressed: { id: string; x: number; y: number; pointer: number } | null = null;
  let drag = $state<{ id: string; label: string; x: number; y: number; width: number; mat: number; index: number } | null>(null);
  // A finger has to hold a card still before it lifts, or a swipe could not scroll.
  let holding: { timer: ReturnType<typeof setTimeout>; x: number; y: number; pointer: number } | null = null;
  const HOLD_MS = 280;

  /** A press anywhere on a card that is not one of its controls. */
  function press(e: PointerEvent, it: ItemView) {
    if (!it.movable || busy) return;
    if ((e.target as HTMLElement).closest('button, a, input, select, form, .menu')) return;
    if (e.pointerType === 'mouse' && e.button !== 0) return;
    const card = e.currentTarget as HTMLElement;
    if (e.pointerType === 'touch') {
      cancelHold();
      const pointer = e.pointerId;
      const x = e.clientX;
      const y = e.clientY;
      holding = {
        x,
        y,
        pointer,
        timer: setTimeout(() => {
          holding = null;
          lift(card, it, x, y, pointer);
          navigator.vibrate?.(12);
        }, HOLD_MS),
      };
      return;
    }
    // A mouse: no text selection under the drag.
    e.preventDefault();
    lift(card, it, e.clientX, e.clientY, e.pointerId);
  }

  function lift(card: HTMLElement, it: ItemView, x: number, y: number, pointer: number) {
    pressed = { id: it.id, x, y, pointer };
    try {
      card.setPointerCapture(pointer);
    } catch {
      // The finger has already gone.
    }
  }

  function cancelHold() {
    if (holding) clearTimeout(holding.timer);
    holding = null;
  }

  // While a finger drags a card, the page must not scroll under it. Pointer events cannot
  // stop that once a touch has begun; a touchmove that is not passive can.
  $effect(() => {
    const stop = (e: TouchEvent) => {
      if (pressed) e.preventDefault();
    };
    window.addEventListener('touchmove', stop, { passive: false });
    return () => window.removeEventListener('touchmove', stop);
  });

  /** Where the pointer is over the board: which mat, and before which of its cards. */
  function dropAt(x: number, y: number, id: string): { mat: number; index: number } | null {
    const column = document
      .elementsFromPoint(x, y)
      .map((el) => (el as HTMLElement).closest?.('[data-mat]') as HTMLElement | null)
      .find((el) => el);
    if (!column) return null;
    const mat = Number(column.dataset.mat);
    const cards = [...column.querySelectorAll<HTMLElement>('[data-item]')].filter((c) => c.dataset.item !== id);
    let index = 0;
    for (const c of cards) {
      const r = c.getBoundingClientRect();
      if (y > r.top + r.height / 2) index++;
    }
    return { mat, index };
  }

  function drift(e: PointerEvent) {
    if (holding && e.pointerId === holding.pointer && Math.hypot(e.clientX - holding.x, e.clientY - holding.y) > 8) {
      // It moved before it lifted: a scroll, not a drag.
      cancelHold();
      return;
    }
    if (!pressed || e.pointerId !== pressed.pointer) return;
    if (!drag) {
      if (Math.hypot(e.clientX - pressed.x, e.clientY - pressed.y) < 6) return;
      const it = view?.items.find((i) => i.id === pressed!.id);
      const card = e.currentTarget as HTMLElement;
      if (!it) return;
      const at = placeOf(it.id);
      drag = { id: it.id, label: `${it.disciplineName} · ${label(it)}`, x: 0, y: 0, width: card.offsetWidth || 200, mat: at?.mat ?? it.mat, index: at?.index ?? it.position - 1 };
    }
    const at = dropAt(e.clientX, e.clientY, drag.id);
    drag = { ...drag, x: e.clientX, y: e.clientY, ...(at ?? {}) };
  }

  function release(e: PointerEvent) {
    cancelHold();
    if (!pressed || e.pointerId !== pressed.pointer) return;
    const done = drag;
    pressed = null;
    drag = null;
    if (!done) return;
    // The board as it is now, not as it was when the drag began.
    const it = view?.items.find((i) => i.id === done.id);
    if (!it || !it.movable) return;
    plan(it, done.mat, done.index);
  }

  function cancel() {
    cancelHold();
    pressed = null;
    drag = null;
  }

  /** Whether a drop line goes before this card of this mat, or (index past the end) after the last. */
  function dropBefore(mat: number, list: ItemView[], it: ItemView): boolean {
    if (!drag || drag.mat !== mat || it.id === drag.id) return false;
    const others = list.filter((o) => o.id !== drag!.id);
    return others.indexOf(it) === drag.index;
  }
  function dropAtEnd(mat: number, list: ItemView[]): boolean {
    if (!drag || drag.mat !== mat) return false;
    return drag.index >= list.filter((o) => o.id !== drag!.id).length;
  }
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && (cancel(), (menuFor = null))} />

<section class="board">
  <div class="head">
    <h2>{t('Mats')}</h2>
    {#if canSetCount && view}
      <span class="count">
        <button aria-label={t('One mat fewer')} disabled={busy || mats.length <= 1} onclick={() => setCount(mats.length - 1)}>&minus;</button>
        <span>{t('{n} mats in the hall', { n: mats.length })}</span>
        <button aria-label={t('One mat more')} disabled={busy || mats.length >= 8} onclick={() => setCount(mats.length + 1)}>+</button>
      </span>
    {/if}
  </div>
  <p class="dim">
    {disciplines > 1
      ? t('Every discipline’s pools, eliminations and finals, in the order each mat runs them. Drag a card, or use its menu; on a touch screen, hold it a moment first. What a mat is running stays where it is.')
      : t('The pools, eliminations and finals, in the order each mat runs them. Drag a card, or use its menu; on a touch screen, hold it a moment first. What a mat is running stays where it is.')}
  </p>
  {#if error}<p class="err" role="alert">{error}</p>{/if}

  {#if draft.length > 0}
    <!-- Nothing has moved for anyone else yet: the score keepers and the screens still
         follow the plan as it was. -->
    <div class="draft" role="region" aria-label={t('Moves not applied yet')}>
      <span>
        {draft.length === 1 ? t('1 move not applied yet.') : t('{n} moves not applied yet.', { n: draft.length })}
        <span class="dim-inline">{t('The times and the mats in the hall change when it is applied.')}</span>
      </span>
      <span class="actions">
        <button class="suggest" disabled={busy} onclick={applyDraft}>{t('Apply')}</button>
        <button disabled={busy} onclick={discard}>{t('Discard')}</button>
      </span>
    </div>
  {/if}

  {#if forecast && forecast.end}
    <div class="day">
      <p>
        {forecast.live ? t('Forecast to end at {time}', { time: clockOf(forecast.end) }) : t('Planned to end at {time}', { time: clockOf(forecast.end) })}
        {#if forecast.live && forecast.plannedEnd && clockOf(forecast.plannedEnd) !== clockOf(forecast.end)}
          <span class="dim-inline">{t('(planned {time})', { time: clockOf(forecast.plannedEnd) })}</span>
        {/if}
      </p>
      <button class="suggest" disabled={busy} onclick={suggest}>{t('Suggest a plan')}</button>
    </div>
    {#each forecast.warnings as w, i (i)}
      <p class="warn">{warningText(w)}</p>
    {/each}
  {/if}

  {#if suggestion}
    <div class="suggestion" role="region" aria-label={t('Suggested plan')}>
      {#if suggestNote}<p class="warn">{suggestNote}</p>{/if}
      {#if suggestion.moves.length === 0}
        <p>{t('Nothing to move: the plan is already as good as the suggestion can make it.')}</p>
      {:else}
        <p>
          {t('This would end the day at {end} instead of {before}.', { end: clockOf(suggestion.end), before: clockOf(suggestion.before) })}
        </p>
        <ul>
          {#each suggestion.moves as m (m.id)}
            <li>
              <span class="strong">{itemName(m.id)}</span>:
              {m.fromMat === m.toMat
                ? t('place {from} to {to} on mat {mat}', { from: m.fromPosition, to: m.toPosition, mat: m.toMat })
                : t('mat {from} to mat {to}', { from: m.fromMat, to: m.toMat })}
              <span class="dim-inline">{t('free at {time}', { time: clockOf(m.start) })}</span>
            </li>
          {/each}
        </ul>
      {/if}
      <div class="actions">
        {#if suggestion.moves.length > 0}
          <button class="suggest" disabled={busy} onclick={applySuggestion}>{t('Apply')}</button>
        {/if}
        <button disabled={busy} onclick={() => (suggestion = null)}>{t('Dismiss')}</button>
      </div>
      <p class="dim small">{t('Anything pinned stays on its mat, and what a mat is running stays where it is.')}</p>
    </div>
  {/if}

  {#if forecast && forecast.items.length > 0 && view}
    <DayTimeline
      {forecast}
      items={view.items}
      mats={mats.map((m) => m.mat)}
      matNames={Object.fromEntries(mats.map((m) => [m.mat, m.name ?? '']))}
      name={itemName}
      short={shortOf}
      {onshort}
    />
  {/if}

  {#if !view}
    <p class="dim">{live.error || t('Loading…')}</p>
  {:else}
    <div class="columns" class:dragging={!!drag}>
      {#each mats as m (m.mat)}
        {@const list = byMat.get(m.mat) ?? []}
        <div class="column" data-mat={m.mat}>
          <h3>{t('Mat {n}', { n: m.mat })}{#if m.name}{' · '}{m.name}{/if}</h3>
          {#each m.away ?? [] as a (a.from)}
            <p class="away">{t('away {from}–{to}', { from: a.from, to: a.to })}</p>
          {/each}
          {#if m.current}
            <p class="now">
              <span class="disc">{m.current.disciplineName}</span>
              {m.current.match.round ? roundLabel(m.current.match) : t('Pool {n}', { n: m.current.match.pool })}:
              {m.current.red} {t('v')} {m.current.blue}
            </p>
          {:else}
            <p class="now dim">{list.length === 0 ? t('Nothing placed here.') : t('Nothing to fence just now.')}</p>
          {/if}
          <ol>
            {#each list as it (it.id)}
              <li
                class="card {it.status}"
                class:short={shortOf.has(it.id) && it.status !== 'done'}
                class:movable={it.movable}
                class:moved={moved.has(it.id)}
                class:lifted={drag?.id === it.id}
                class:drop-before={dropBefore(m.mat, list, it)}
                data-item={it.id}
                onpointerdown={(e) => press(e, it)}
                onpointermove={drift}
                onpointerup={release}
                onpointercancel={cancel}
                oncontextmenu={(e) => (holding || pressed) && e.preventDefault()}
              >
                <!-- The whole card is what is dragged; this only says it can be. -->
                <span class="grip" class:locked={!it.movable} aria-hidden="true" title={it.movable ? '' : statusLabel(it)}>{it.movable ? '⠿' : '•'}</span>
                <span class="what">
                  {#if disciplines > 1}<span class="disc">{it.disciplineName}</span>{/if}
                  <span class="name">{label(it)}</span>
                  <span class="meta">
                    {it.done}/{it.total}{#if statusLabel(it)}{' '}&middot; {statusLabel(it)}{/if}
                  </span>
                  {#if span(it) && it.status !== 'done'}
                    <span class="meta when">
                      {span(it)}
                      {#if lateBy(it) >= 5}<span class="late">{t('+{n} min', { n: lateBy(it) })}</span>{/if}
                      {#if lateBy(it) <= -5}<span class="early">{t('{n} min early', { n: -lateBy(it) })}</span>{/if}
                    </span>
                  {/if}
                  {#if shortOf.has(it.id) && it.status !== 'done'}
                    <a class="meta staff-short" href="#staff" onclick={toStaff}>
                      {t('Short of {roles}', { roles: shortOf.get(it.id)!.join(', ').toLocaleLowerCase() })}
                    </a>
                  {/if}
                  {#if it.pinned || it.notBefore}
                    <span class="meta flags">
                      {#if it.pinned}<span class="flag">{t('pinned')}</span>{/if}
                      {#if it.notBefore}<span class="flag">{t('not before {time}', { time: it.notBefore })}</span>{/if}
                    </span>
                  {/if}
                </span>
                {#if it.movable}
                  <button class="more" aria-haspopup="menu" aria-expanded={menuFor === it.id} aria-label={t('Move {item}', { item: label(it) })} onclick={() => openMenu(it)}>&hellip;</button>
                  {#if menuFor === it.id}
                    <div class="menu" role="menu">
                      <button role="menuitem" disabled={busy} onclick={() => step(it, 'up')}>{t('Earlier')}</button>
                      <button role="menuitem" disabled={busy} onclick={() => step(it, 'down')}>{t('Later')}</button>
                      {#each mats.filter((o) => o.mat !== it.mat) as o (o.mat)}
                        <button role="menuitem" disabled={busy} onclick={() => moveTo(it, o.mat)}>{t('To the end of mat {n}', { n: o.mat })}</button>
                      {/each}
                      <button role="menuitem" disabled={busy} onclick={() => pin(it, !it.pinned)}>{it.pinned ? t('Unpin') : t('Pin to this mat')}</button>
                      <form class="hold" onsubmit={(e) => (e.preventDefault(), hold(it, holdUntil))}>
                        <label>
                          {t('Not before')}
                          <input type="time" bind:value={holdUntil} />
                        </label>
                        <button type="submit" disabled={busy || !holdUntil}>{t('Hold')}</button>
                        {#if it.notBefore}
                          <button type="button" disabled={busy} onclick={() => hold(it, '')}>{t('Let go')}</button>
                        {/if}
                      </form>
                    </div>
                  {/if}
                {/if}
              </li>
            {/each}
            {#if dropAtEnd(m.mat, list)}<li class="drop-end" aria-hidden="true"></li>{/if}
          </ol>
        </div>
      {/each}
    </div>
  {/if}

  {#if drag}
    <div class="ghost" style="left: {drag.x}px; top: {drag.y}px; width: {drag.width}px" aria-hidden="true">{drag.label}</div>
  {/if}
</section>

<style>
  .board {
    background: var(--panel);
    border-radius: var(--radius);
    padding: 1.1rem 1.25rem;
    margin-bottom: 1rem;
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    justify-content: space-between;
    gap: 0.5rem 1rem;
  }
  h2 {
    margin: 0 0 0.5rem;
    font-size: 1.15rem;
  }
  .count {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.9rem;
  }
  .count button {
    width: 2rem;
    padding: 0.2rem 0;
  }
  .dim {
    margin: 0 0 0.8rem;
    color: var(--ink-dim);
    line-height: 1.5;
    font-size: 0.9rem;
  }
  .err {
    color: var(--amber-bright);
  }
  .columns {
    display: grid;
    grid-auto-flow: column;
    grid-auto-columns: minmax(13.5rem, 1fr);
    gap: 0.75rem;
    overflow-x: auto;
    padding-bottom: 0.3rem;
  }
  .column {
    background: var(--panel-2);
    border-radius: var(--radius);
    padding: 0.7rem;
    min-height: 6rem;
  }
  .columns.dragging {
    user-select: none;
  }
  .columns.dragging .column {
    outline: 1px dashed var(--line);
  }
  h3 {
    margin: 0 0 0.3rem;
    font-size: 0.75rem;
    font-weight: 800;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--ink-dim);
  }
  .away {
    margin: 0 0 0.3rem;
    font-size: 0.75rem;
    color: var(--amber-bright);
  }
  .now {
    margin: 0 0 0.6rem;
    font-size: 0.82rem;
    line-height: 1.4;
    overflow-wrap: anywhere;
  }
  .disc {
    display: block;
    font-size: 0.72rem;
    font-weight: 700;
    color: var(--amber-bright);
  }
  ol {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.35rem;
  }
  .card {
    position: relative;
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    align-items: center;
    gap: 0.4rem;
    padding: 0.4rem 0.45rem;
    border-radius: 8px;
    background: var(--panel);
    border: 1px solid var(--line);
  }
  .card.running {
    border-color: var(--amber-bright);
  }
  /* Not drawn yet: planned time, not work anybody can fence. */
  .card.planned {
    border-style: dashed;
    background: transparent;
  }
  .when {
    font-variant-numeric: tabular-nums;
  }
  .late {
    margin-left: 0.3rem;
    color: var(--amber-bright);
    font-weight: 700;
  }
  .early {
    margin-left: 0.3rem;
    color: var(--ok);
  }
  .flags {
    display: flex;
    flex-wrap: wrap;
    gap: 0.25rem;
  }
  .flag {
    padding: 0 0.3rem;
    border: 1px solid var(--line);
    border-radius: 4px;
  }
  .day {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem 1rem;
    margin-bottom: 0.4rem;
  }
  .day p {
    margin: 0;
    font-weight: 700;
  }
  .dim-inline {
    color: var(--ink-dim);
    font-weight: 400;
    margin-left: 0.3rem;
  }
  .suggest {
    padding: 0.45rem 0.9rem;
    font-weight: 700;
    background: var(--ink);
    color: #0d0f14;
    border: none;
  }
  .warn {
    margin: 0.2rem 0;
    color: var(--amber-bright);
    font-size: 0.88rem;
    line-height: 1.45;
  }
  .suggestion {
    margin: 0.6rem 0 0.8rem;
    padding: 0.7rem 0.8rem;
    border: 1px dashed var(--amber-bright);
    border-radius: var(--radius);
    font-size: 0.9rem;
  }
  .suggestion p {
    margin: 0 0 0.4rem;
  }
  .suggestion ul {
    margin: 0 0 0.6rem;
    padding-left: 1.1rem;
    line-height: 1.5;
  }
  .suggestion .actions {
    display: flex;
    gap: 0.5rem;
    margin-bottom: 0.4rem;
  }
  .strong {
    font-weight: 700;
  }
  .small {
    font-size: 0.8rem;
    margin: 0;
  }
  .hold {
    display: flex;
    flex-wrap: wrap;
    align-items: end;
    gap: 0.3rem;
    padding: 0.35rem 0.6rem;
    border-top: 1px solid var(--line);
    margin-top: 0.2rem;
  }
  .hold label {
    display: grid;
    gap: 0.15rem;
    font-size: 0.75rem;
    color: var(--ink-dim);
  }
  .hold input {
    padding: 0.2rem 0.3rem;
  }
  .hold button {
    padding: 0.3rem 0.5rem;
    font-size: 0.8rem;
  }
  .card.done {
    opacity: 0.55;
  }
  .card.lifted {
    opacity: 0.35;
  }
  .card.drop-before::before,
  .drop-end {
    content: '';
    display: block;
    height: 3px;
    border-radius: 2px;
    background: var(--amber-bright);
  }
  .card.drop-before::before {
    position: absolute;
    left: 0;
    right: 0;
    top: -0.3rem;
  }
  .card.movable {
    cursor: grab;
    user-select: none;
    -webkit-user-select: none;
    -webkit-touch-callout: none;
  }
  .columns.dragging .card.movable {
    cursor: grabbing;
  }
  /* Short of staff (#142): orange, the colour of what needs the organizer, and a way to
     the staff from the card. */
  .card.short {
    box-shadow: inset 3px 0 0 var(--amber-bright);
  }
  .staff-short {
    color: var(--amber-bright);
    font-weight: 700;
  }
  /* Moved in the draft, not applied yet. */
  .card.moved {
    border-style: dashed;
    border-color: var(--amber-bright);
  }
  .grip {
    display: grid;
    place-items: center;
    width: 1.2rem;
    color: var(--ink-dim);
    font-size: 1rem;
  }
  .grip.locked {
    font-size: 0.9rem;
  }
  .draft {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem 1rem;
    margin: 0 0 0.8rem;
    padding: 0.6rem 0.8rem;
    border: 1px dashed var(--amber-bright);
    border-radius: var(--radius);
    font-size: 0.9rem;
  }
  .draft .actions {
    display: flex;
    gap: 0.5rem;
  }
  .what {
    display: grid;
    min-width: 0;
  }
  .name {
    font-weight: 700;
    font-size: 0.92rem;
  }
  .meta {
    font-size: 0.75rem;
    color: var(--ink-dim);
  }
  .more {
    padding: 0.2rem 0.5rem;
    font-size: 1rem;
  }
  .menu {
    position: absolute;
    right: 0.3rem;
    top: calc(100% + 0.2rem);
    z-index: 20;
    display: grid;
    min-width: 11rem;
    background: var(--panel-2);
    border: 1px solid var(--line);
    border-radius: 8px;
    padding: 0.25rem;
    box-shadow: 0 6px 18px rgba(0, 0, 0, 0.4);
  }
  .menu button {
    text-align: left;
    background: none;
    border: none;
    padding: 0.45rem 0.6rem;
    font-size: 0.85rem;
  }
  .menu button:hover {
    background: var(--panel);
  }
  .ghost {
    position: fixed;
    z-index: 100;
    transform: translate(-1rem, -50%);
    pointer-events: none;
    padding: 0.5rem 0.7rem;
    border-radius: 8px;
    background: var(--panel-2);
    border: 1px solid var(--amber-bright);
    font-weight: 700;
    font-size: 0.9rem;
    box-shadow: 0 8px 20px rgba(0, 0, 0, 0.45);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
