import { t } from './i18n.svelte';

/**
 * The names of work items and staff roles, as every page that lists them says them: the mat
 * board, the staff panel and a person's page.
 */

/** "Pool 3", "Eliminations 2", "Bronze match", "Final". lanes is how many eliminations items the discipline has. */
export function itemLabel(kind: string, number: number | undefined, lanes = 1): string {
  switch (kind) {
    case 'pool':
      return t('Pool {n}', { n: number ?? '' });
    case 'eliminations':
      return lanes > 1 ? t('Eliminations {n}', { n: number ?? '' }) : t('Eliminations');
    case 'bronze':
      return t('Bronze match');
    case 'final':
      return t('Final');
  }
  return kind;
}

/** The roles of #5, in the order a crew is listed. */
export const ROLES = ['head-ref', 'assistant-ref', 'score-keeper', 'physician'] as const;

/** The roles a mat needs while it runs. */
export const MAT_ROLES = ['head-ref', 'assistant-ref', 'score-keeper'] as const;

export function roleLabel(role: string): string {
  switch (role) {
    case 'head-ref':
      return t('Head referee');
    case 'assistant-ref':
      return t('Assistant referee');
    case 'score-keeper':
      return t('Score keeper');
    case 'physician':
      return t('Physician');
  }
  return role;
}
