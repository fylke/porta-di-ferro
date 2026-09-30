/**
 * One layout for a whole list of names and clubs, decided by the widest row.
 *
 * Left to itself, a flex row wraps only where it has to, so a list with one long club in
 * it ends up with most rows on one line and a few on two -- which reads as a broken
 * table, not as a list. Instead every row is measured on a single line, and the widest
 * decides for all of them: two columns if it fits in one of two, one column if it fits
 * across, and otherwise every row on two lines, name over club.
 */

export type Layout = 'two' | 'one' | 'stacked';

/**
 * The layout for a list whose widest single-line row is `widest` pixels, in a list
 * `width` pixels wide. `twoColumns` is whether the screen is wide enough to offer two
 * columns at all, and `gap` is the space between them.
 */
export function chooseLayout(widest: number, width: number, twoColumns: boolean, gap: number): Layout {
  // Nothing measured yet -- an empty list, or one not laid out -- takes the roomiest.
  if (widest <= 0 || width <= 0) return twoColumns ? 'two' : 'one';
  // Rounded against each other so a subpixel difference does not flip the layout.
  const need = Math.ceil(widest);
  if (twoColumns && need <= Math.floor((width - gap) / 2)) return 'two';
  if (need <= Math.floor(width)) return 'one';
  return 'stacked';
}

/**
 * Measures a list and reports its layout to `onlayout`, which the component binds to
 * `data-layout` in its markup -- where Svelte can see it, so the CSS for each layout is
 * kept. Rows are the elements marked `.fit-row`.
 *
 * To measure, the list gets the class `fit-measure` for the length of one synchronous
 * read, and the component's own CSS lays each row out on one line at its natural width
 * under it. Nothing is painted in between, so the organizer never sees it.
 */
export function fitRows(
  node: HTMLElement,
  options: { onlayout: (layout: Layout) => void; twoColumnsFrom?: string; gap?: number },
) {
  let lastWidth = -1;

  function measure() {
    node.classList.add('fit-measure');
    let widest = 0;
    for (const row of node.querySelectorAll<HTMLElement>('.fit-row')) {
      widest = Math.max(widest, row.getBoundingClientRect().width);
    }
    node.classList.remove('fit-measure');

    const rem = parseFloat(getComputedStyle(document.documentElement).fontSize) || 16;
    const two = !!options.twoColumnsFrom && matchMedia(options.twoColumnsFrom).matches;
    options.onlayout(chooseLayout(widest, node.clientWidth, two, (options.gap ?? 1) * rem));
  }

  // A width change can change the answer; a height change is the answer being applied,
  // and measuring again on it would only arrive at the same place.
  const resize = new ResizeObserver(() => {
    if (node.clientWidth !== lastWidth) {
      lastWidth = node.clientWidth;
      measure();
    }
  });
  // Somebody added, removed or renamed. Attributes are left out: the layout is one.
  const mutate = new MutationObserver(measure);

  resize.observe(node);
  mutate.observe(node, { childList: true, subtree: true, characterData: true });
  measure();
  // A web font arriving late changes every width.
  void document.fonts?.ready.then(measure);

  return {
    destroy() {
      resize.disconnect();
      mutate.disconnect();
    },
  };
}
