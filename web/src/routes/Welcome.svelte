<script lang="ts">
  import { onMount } from 'svelte';
  import { hall } from '../lib/event.svelte';
  import { navigate } from '../router.svelte';
  import { tour } from '../lib/tour.svelte';
  import { t } from '../lib/i18n.svelte';
  import LangToggle from './LangToggle.svelte';

  /**
   * What the organizer sees at /admin before anything is planned (#149): the name, the
   * gate, and the two things there are to do. The admin page proper -- the mats, the
   * screens, the staff -- has nothing to show until there is an event, and showing it
   * empty, or full of the last event's leftovers, buried the way in.
   */
  const logo = `${import.meta.env.BASE_URL}favicon.svg`;

  onMount(() => {
    hall.follow();
    return () => hall.unfollow();
  });
</script>

<main class="admin welcome">
  <div class="lang"><LangToggle /></div>
  <img class="logo" src={logo} alt="" width="96" height="96" />
  <h1>Porta di Ferro</h1>
  <p class="tag">{t('Pools, brackets and the day around them, for one event at a time.')}</p>
  <div class="actions">
    <button class="secondary" onclick={() => tour.start()}>{t('Take a tour')}</button>
    <button data-tour="welcome-plan" onclick={() => navigate('/plan?step=1')}>{t('Plan an event')}</button>
  </div>
</main>

<style>
  .welcome {
    min-height: 100%;
    display: grid;
    place-content: center;
    justify-items: center;
    gap: 0.6rem;
    padding: 2rem 16px;
    text-align: center;
    position: relative;
  }
  .lang {
    position: absolute;
    top: 1rem;
    right: 16px;
  }
  .logo {
    width: 6rem;
    height: 6rem;
  }
  h1 {
    margin: 0.4rem 0 0;
    font-size: clamp(1.8rem, 6vw, 2.6rem);
  }
  .tag {
    margin: 0 0 1rem;
    color: var(--ink-dim);
    max-width: 28rem;
    line-height: 1.5;
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 0.6rem;
  }
</style>
