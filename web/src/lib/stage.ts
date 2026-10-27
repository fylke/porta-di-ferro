/**
 * Where a discipline has got to, in words, for the event's landing page and admin.
 */
import type { DisciplineSummary } from '../api';
import { t } from './i18n.svelte';

export function stageLabel(
  d: Pick<DisciplineSummary, 'stage' | 'matchesDone' | 'matchesTotal' | 'competitors'> & { setUp?: boolean },
): string {
  switch (d.stage) {
    case 'setup':
      if (d.competitors > 0) return t('{n} entered, pools not drawn yet', { n: d.competitors });
      // Set up is the organizer's word for having finished its setup (#149).
      return d.setUp ? t('Set up, nobody entered yet') : t('Not set up yet');
    case 'pools':
      return t('Pools: {done} of {total} matches fenced', { done: d.matchesDone, total: d.matchesTotal });
    case 'waiting':
      return t('Pools done, eliminations next');
    case 'eliminations':
      return t('Eliminations');
    case 'done':
      return t('Finished');
  }
}
