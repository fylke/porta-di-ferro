/**
 * Sections a viewer has folded away (#110): a pool's results, the bracket, a list of names.
 * Open unless folded, and remembered in this browser per page, so a phone in the hall that
 * folded away the pools it is not in finds them folded on the next visit. Only ever a
 * convenience: storage that is missing or blocked leaves everything open.
 */
export class Folds {
  #closed = $state<Record<string, boolean>>({});
  private readonly key: string;

  constructor(scope: string) {
    this.key = `porta.folds.${scope}`;
    try {
      const raw = localStorage.getItem(this.key);
      if (raw) this.#closed = JSON.parse(raw) as Record<string, boolean>;
    } catch {
      // No storage: everything open.
    }
  }

  open(id: string): boolean {
    return !this.#closed[id];
  }

  toggle(id: string): void {
    const next = { ...this.#closed };
    if (next[id]) delete next[id];
    else next[id] = true;
    this.#closed = next;
    try {
      localStorage.setItem(this.key, JSON.stringify(next));
    } catch {
      // The fold still works for this visit.
    }
  }
}
