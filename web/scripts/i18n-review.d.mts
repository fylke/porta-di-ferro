// Types for the translation review tool (i18n-review.mjs), for its test under src/.
export interface SvEntry {
  key: string;
  sv: string;
  section: string;
  line: number;
}
export interface Usage {
  files: string[];
  kinds: string[];
}
export interface ReviewEntry {
  sv: string;
  approved?: boolean;
  suggestion?: string;
  note?: string;
}
export function parseSv(source: string): SvEntry[];
export function kindOf(line: string): string;
export function usages(files: { path: string; text: string }[], keys: string[]): Record<string, Usage>;
export function standing(entries: { key: string; sv: string }[], reviewed: Record<string, string>): Record<string, 'new' | 'changed' | 'reviewed'>;
export function accept(
  reviewed: Record<string, string>,
  review: { entries: Record<string, ReviewEntry> },
  entries: SvEntry[],
): { reviewed: Record<string, string>; suggestions: { key: string; line: number; from: string; to: string; note: string }[] };
