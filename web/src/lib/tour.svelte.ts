/**
 * The guided tour (#149): a walk through the organizer's pages, a step at a time, from the
 * welcome screen. Each step is a page and, where there is one, the panel on it to look at
 * -- marked in the page with data-tour="…" -- and a few words about what it is for.
 *
 * Held in this module rather than in storage: it lives as long as the tab, the pages it
 * visits are reached by navigate(), and a reload simply ends it. Nothing here touches the
 * event; the tour shows the pages as they are.
 */
import { navigate } from '../router.svelte';
import { t } from './i18n.svelte';

export interface TourStep {
  /** The page the step is on. */
  path: string;
  /** The data-tour mark of the panel to look at; the card stands alone without one. */
  target?: string;
  title: () => string;
  text: () => string;
}

export const steps: TourStep[] = [
  {
    path: '/admin',
    target: 'welcome-plan',
    title: () => t('Everything starts with a plan'),
    text: () => t('Plan an event takes you through it in three steps: the event, its disciplines, and the day. This tour shows each of them, and the pages you use on the day.'),
  },
  {
    path: '/plan?step=1',
    target: 'plan-event',
    title: () => t('The event'),
    text: () => t('Its identifier, name, venue and date go in every signup file, so they come first and are needed to go on. The welcome and the wifi are for the info sheet by the door.'),
  },
  {
    path: '/plan?step=2',
    target: 'plan-disciplines',
    title: () => t('Disciplines'),
    text: () => t('Add each discipline you fence. Set up opens its own short setup in a new tab: the mats it uses and the size of its pools.'),
  },
  {
    path: '/plan?step=3',
    target: 'plan-timings',
    title: () => t('Planning the day'),
    text: () => t('How long a match takes, when the day starts, the mats and the breaks. The programme is written from these, so nobody types it by hand.'),
  },
  {
    path: '/plan?step=3',
    target: 'plan-programme',
    title: () => t('The programme'),
    text: () => t('Gear check, staff briefing, pools, eliminations, finals and lunch, as the plan has them. Anything a day usually has and this one lacks is pointed out here.'),
  },
  {
    path: '/admin?view=before',
    target: 'signup',
    title: () => t('Before the day'),
    text: () => t('One signup file for the whole event goes out to participants, who fill it in offline and send it back. Importing the answers enters them.'),
  },
  {
    path: '/admin?view=before',
    target: 'staff',
    title: () => t('Staff'),
    text: () => t('Who works the mats. A suggestion fills every pool’s crew, never with somebody who is fencing at the time.'),
  },
  {
    path: '/admin?view=day',
    target: 'matboard',
    title: () => t('On the day'),
    text: () => t('Every mat and what it runs, as the day goes. Pools can be moved between mats; the eliminations and the finals follow them.'),
  },
  {
    path: '/admin?view=day',
    target: 'join',
    title: () => t('Score keepers and screens'),
    text: () => t('A score keeper’s tablet scans this code and picks its mat. Screens in the hall show each mat, the roster and the results.'),
  },
  {
    path: '/admin',
    title: () => t('That is the tour'),
    text: () => t('Plan an event when you are ready. Everything can be changed afterwards.'),
  },
];

class TourState {
  active = $state(false);
  index = $state(0);

  get step(): TourStep {
    return steps[this.index];
  }
  get last(): boolean {
    return this.index === steps.length - 1;
  }

  start(): void {
    this.active = true;
    this.go(0);
  }
  next(): void {
    if (this.last) this.stop();
    else this.go(this.index + 1);
  }
  back(): void {
    if (this.index > 0) this.go(this.index - 1);
  }
  stop(): void {
    this.active = false;
  }

  private go(i: number): void {
    this.index = i;
    navigate(steps[i].path);
  }
}

export const tour = new TourState();
