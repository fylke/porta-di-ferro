<script lang="ts">
  import { demoControls } from './adapter';
  import { t } from '../lib/i18n.svelte';
  import { route } from '../router.svelte';

  /**
   * The strip along the bottom of every demo screen (issue #88).
   *
   * It is there to stop the demo being mistaken for the application. This runs in one
   * browser with no server behind it: nothing reaches another device, what the visitor
   * changed is gone after half an hour away (issue #108), and a visitor who believed
   * otherwise would have a bad surprise waiting at their event. So it says so, on every
   * screen, rather than once on a landing page nobody reads.
   *
   * The two buttons are the demo's own and exist nowhere in the application. Play the
   * rest is how somebody who does not want to score forty matches still gets to see the
   * bracket and the podium, which is the part that sells this.
   *
   * The demo opens on the participants' landing page, and nothing there leads to /admin
   * -- rightly, at an event, where the address is on a poster for everyone. A visitor has
   * no poster telling them the organizer's side exists, so the strip links to it, and
   * back again from it.
   */
  const onAdmin = $derived(!!route('/admin'));
  let busy = $state(false);
  let open = $state(false);

  function run(fn: () => void) {
    busy = true;
    // A frame for the button to look pressed before the module blocks the thread.
    requestAnimationFrame(() => {
      try {
        fn();
      } finally {
        busy = false;
      }
    });
  }
</script>

<aside class="demo" class:open aria-label={t('Demo')}>
  <button class="toggle" onclick={() => (open = !open)} aria-expanded={open}>
    <span class="dot" aria-hidden="true"></span>
    {t('Demo')}
  </button>

  <p class="what">
    {t('A real tournament, running entirely in this browser. Your changes follow you from tab to tab until you leave it alone for half an hour, and no other device can see them.')}
  </p>

  <span class="actions">
    {#if onAdmin}
      <a class="go" href="/">{t('Landing page')}</a>
    {:else}
      <a class="go" href="/admin">{t('The organizer’s view')}</a>
    {/if}
    <button disabled={busy} onclick={() => run(demoControls.playRest)}>
      {t('Play the rest')}
    </button>
    <button disabled={busy} onclick={() => run(demoControls.reset)}>{t('Start over')}</button>
    <a href="https://github.com/fylke/porta-di-ferro" target="_blank" rel="noreferrer">
      {t('The project')}
    </a>
  </span>
</aside>

<style>
  /* Fixed to the bottom and out of the way of the score keeper's confirm button, which is
     the one control that must never be competed with. Collapsed to a tab on a phone,
     because the score keeper client is the screen most likely to be opened on one and it
     is laid out to the pixel. */
  .demo {
    position: fixed;
    left: 0;
    bottom: 0;
    z-index: 80;
    display: flex;
    align-items: center;
    gap: 0.9rem;
    padding: 0.4rem 0.8rem;
    background: var(--panel-2);
    border-top: 1px solid var(--line);
    border-right: 1px solid var(--line);
    border-radius: 0 10px 0 0;
    font-size: 0.8rem;
    max-width: 100%;
  }
  .toggle {
    display: none;
    align-items: center;
    gap: 0.4rem;
    background: none;
    border: none;
    color: var(--amber-bright);
    font-weight: 800;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    font-size: 0.72rem;
    padding: 0.2rem 0;
  }
  .dot {
    width: 0.5rem;
    height: 0.5rem;
    border-radius: 50%;
    background: var(--amber-bright);
  }
  .what {
    margin: 0;
    color: var(--ink-dim);
    line-height: 1.4;
  }
  .actions {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    flex: none;
  }
  .actions button {
    padding: 0.3rem 0.6rem;
    font-size: 0.78rem;
    background: var(--panel);
    border: 1px solid var(--line);
    color: var(--ink);
  }
  .actions button:disabled {
    opacity: 0.5;
  }
  .actions a {
    color: var(--ink-dim);
    font-size: 0.78rem;
  }
  /* The way into the organizer's side is the one thing here a visitor is looking for, so
     it reads as a button rather than as the quiet link to the project. */
  .actions a.go {
    padding: 0.3rem 0.6rem;
    border-radius: 6px;
    background: var(--amber-bright);
    color: #0d0f14;
    font-weight: 700;
    text-decoration: none;
    white-space: nowrap;
  }

  @media (max-width: 760px) {
    .demo {
      padding: 0.3rem 0.6rem;
      gap: 0.5rem;
    }
    .toggle {
      display: inline-flex;
    }
    .what,
    .actions {
      display: none;
    }
    .demo.open {
      right: 0;
      flex-wrap: wrap;
      border-radius: 0;
    }
    .demo.open .what,
    .demo.open .actions {
      display: flex;
    }
    .demo.open .what {
      flex-basis: 100%;
    }
    .demo.open .actions {
      flex: 1 1 100%;
      flex-wrap: wrap;
      row-gap: 0.4rem;
    }
  }
</style>
