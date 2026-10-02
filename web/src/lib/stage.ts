/**
 * Where a discipline has got to, in words, for the event's landing page and admin.
 */
import type { DisciplineSummary } from '../api';
import { t } from './i18n.svelte';

export function stageLabel(d: Pick<DisciplineSummary, 'stage' | 'matchesDone' | 'matchesTotal' | 'competitors'>): string {
  switch (d.stage) {
    case 'setup':
      return d.competitors > 0 ? t('{n} entered, pools not drawn yet', { n: d.competitors }) : t('Not set up yet');
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
