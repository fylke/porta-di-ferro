<script lang="ts">
  import { hall } from '../lib/event.svelte';
  import { path, query } from '../router.svelte';
  import { t } from '../lib/i18n.svelte';
  import LangToggle from './LangToggle.svelte';

  /**
   * An address with no discipline in it, in an event with several (#102).
   *
   * /score/1, /display/mats and /who/c7 meant the one discipline there was, and every
   * bookmark, QR code and spare screen made before the second discipline was added still
   * points at one of them. Rather than guess which discipline it was meant for, the page
   * asks -- and every answer is the same page in that discipline.
   */
  const rest = $derived(path() + (query().toString() ? `?${query().toString()}` : ''));
</script>

<main>
  <div class="top"><LangToggle /></div>
  <h1>{t('Which discipline?')}</h1>
  <p class="dim">{t('This event runs several disciplines, each with its own mats and screens. Pick the one you are here for.')}</p>
  <ul>
    {#each hall.view?.disciplines ?? [] as d (d.slug)}
      <li>
        <a href="/d/{d.slug}{rest}" class:failed={!!d.error}>
          <span class="name">{d.name || t('Unnamed')}</span>
          {#if d.error}<span class="dim">{t('Not available just now.')}</span>{/if}
        </a>
      </li>
    {/each}
  </ul>
  <p><a class="dim" href="/">{t('Landing page')}</a></p>
</main>

<style>
  main {
    max-width: 34rem;
    margin: 2rem auto;
    padding: 0 1.25rem;
  }
  .top {
    display: flex;
    justify-content: flex-end;
  }
  h1 {
    margin: 0 0 0.5rem;
  }
  .dim {
    color: var(--ink-dim);
    line-height: 1.5;
  }
  ul {
    list-style: none;
    margin: 1rem 0;
    padding: 0;
    display: grid;
    gap: 0.6rem;
  }
  a {
    display: grid;
    gap: 0.2rem;
    padding: 1rem 1.1rem;
    border-radius: var(--radius);
    background: var(--panel);
    color: var(--ink);
    text-decoration: none;
  }
  ul a:hover {
    outline: 1px solid var(--line);
  }
  .name {
    font-size: 1.2rem;
    font-weight: 700;
  }
  .failed {
    opacity: 0.6;
  }
  p a {
    display: inline;
    padding: 0;
    background: none;
  }
</style>
