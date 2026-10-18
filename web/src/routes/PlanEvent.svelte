<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../api';
  import { hall } from '../lib/event.svelte';
  import { navigate, query } from '../router.svelte';
  import { t } from '../lib/i18n.svelte';
  import EventEditor from './EventEditor.svelte';
  import Disciplines from './Disciplines.svelte';
  import Planning from './Planning.svelte';
  import LangToggle from './LangToggle.svelte';

  /**
   * Plan the event (#142): what is set once, before the day, in the order it is set -- the
   * day around the fencing, the disciplines, and how long it all takes on how many mats.
   * It used to be half the admin page, between the mats and the screens an organizer
   * watches all day; it is here now, a step at a time, and the admin page links to it.
   *
   * The steps are addresses (/admin/plan?step=2), so Back works and a step can be opened
   * straight from wherever it is wanted -- the landing page's Edit opens the first.
   */
  const steps = [
    { title: () => t('The day'), hint: () => t('The welcome, the programme and the venue wifi: what the landing page and the info sheet put around the fencing.') },
    { title: () => t('Disciplines'), hint: () => t('What is fenced. Each discipline has its own competitors, pools and admin page.') },
    { title: () => t('Planning the day'), hint: () => t('How long things take, how many mats there are, and in what order the day runs.') },
  ];

  onMount(() => {
    void hall.refresh();
    hall.follow();
    return () => hall.unfollow();
  });

  const view = $derived(hall.view);
  const step = $derived(Math.min(steps.length, Math.max(1, Number(query().get('step')) || 1)));

  function go(n: number) {
    navigate(n > steps.length ? '/admin' : `/admin/plan?step=${n}`);
    window.scrollTo(0, 0);
  }
</script>

<main>
  <header>
    <h1>
      {t('Plan the event')}
      {#if view?.name}<span class="event">{view.name}</span>{/if}
    </h1>
    <nav>
      <a href="/admin">{t('Back to the admin page')}</a>
      <LangToggle />
    </nav>
  </header>

  <ol class="steps">
    {#each steps as s, i (i)}
      <li class:current={step === i + 1}>
        <a href="/admin/plan?step={i + 1}" aria-current={step === i + 1 ? 'step' : undefined}>
          <span class="n">{i + 1}</span>{s.title()}
        </a>
      </li>
    {/each}
  </ol>
  <p class="hint">{steps[step - 1].hint()}</p>

  {#if !view}
    <p class="hint">{hall.error || t('Loading…')}</p>
  {:else if step === 1}
    <!-- Keyed on the event, so a new event after a delete is a new form; never on the
         view itself, which changes with every exchange and would lose what is typed. -->
    {#key view.id}
      <EventEditor event={view.info} save={api.saveEventInfo} onchange={() => void hall.refresh()} />
    {/key}
  {:else if step === 2}
    <Disciplines />
  {:else}
    <Planning />
  {/if}

  <div class="pager">
    {#if step > 1}
      <button onclick={() => go(step - 1)}>{t('Back')}</button>
    {/if}
    <button class="next" onclick={() => go(step + 1)}>
      {step < steps.length ? t('Next: {step}', { step: steps[step].title() }) : t('Done')}
    </button>
  </div>
</main>

<style>
  main {
    max-width: 68rem;
    margin: 0 auto;
    padding: 1.25rem 1.25rem 4rem;
  }
  header {
    display: flex;
    flex-wrap: wrap;
    gap: 1rem;
    justify-content: space-between;
    align-items: baseline;
    margin-bottom: 1rem;
  }
  h1 {
    margin: 0;
    font-size: 1.5rem;
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 0.6rem;
  }
  .event {
    font-size: 1.1rem;
    font-weight: 600;
    color: var(--amber-bright);
  }
  nav {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem 1rem;
    font-size: 0.9rem;
  }
  .steps {
    list-style: none;
    margin: 0 0 0.6rem;
    padding: 0;
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
  }
  .steps a {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    padding: 0.35rem 0.8rem 0.35rem 0.4rem;
    border-radius: 999px;
    border: 1px solid var(--line);
    color: var(--ink-dim);
    text-decoration: none;
    font-size: 0.9rem;
  }
  .steps .current a {
    border-color: var(--amber-bright);
    color: var(--ink);
    font-weight: 700;
  }
  .n {
    display: inline-grid;
    place-items: center;
    width: 1.4rem;
    height: 1.4rem;
    border-radius: 50%;
    background: var(--panel-2);
    font-size: 0.8rem;
    font-weight: 800;
  }
  .current .n {
    background: var(--amber-bright);
    color: #0d0f14;
  }
  .hint {
    margin: 0 0 1rem;
    color: var(--ink-dim);
    line-height: 1.5;
  }
  .pager {
    display: flex;
    justify-content: flex-end;
    gap: 0.6rem;
  }
  .next {
    font-weight: 700;
    background: var(--ink);
    color: #0d0f14;
    border: none;
    padding: 0.55rem 1.1rem;
  }
</style>
