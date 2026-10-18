/**
 * A press that counts the moment a finger lands, whatever the other fingers are doing
 * (issue #145).
 *
 * A browser makes a click of a touch only when it ends, and only when it was the one
 * finger on the screen: two fingers landing together -- red's 1 and blue's 1 for a double
 * hit -- read as the start of a pinch, and neither button got its click. Pointer events
 * come one per finger, so each button acts on its own pointerdown.
 *
 * The click that follows a single tap is skipped, so nothing counts twice; a click with no
 * pointer before it -- Enter or Space on a focused button -- still counts.
 */
export function tap(node: HTMLElement, handler: () => void) {
  let act = handler;
  let pressed = false;
  let lastUp = -Infinity;

  const down = (e: PointerEvent) => {
    if (e.pointerType === 'mouse' && e.button !== 0) return;
    if ((node as HTMLButtonElement).disabled) return;
    pressed = true;
    act();
  };
  const up = () => {
    if (!pressed) return;
    pressed = false;
    lastUp = performance.now();
  };
  const click = (e: MouseEvent) => {
    if (pressed || performance.now() - lastUp < 600) {
      e.preventDefault();
      return;
    }
    act();
  };

  node.addEventListener('pointerdown', down);
  node.addEventListener('pointerup', up);
  node.addEventListener('pointercancel', up);
  node.addEventListener('click', click);
  return {
    update(next: () => void) {
      act = next;
    },
    destroy() {
      node.removeEventListener('pointerdown', down);
      node.removeEventListener('pointerup', up);
      node.removeEventListener('pointercancel', up);
      node.removeEventListener('click', click);
    },
  };
}
