import { describe, expect, it } from 'vitest';
import { tap } from './tap';

// A button as far as tap() can tell: something that takes listeners and may be disabled.
function button(disabled = false) {
  const node = new EventTarget() as EventTarget & { disabled: boolean };
  node.disabled = disabled;
  return node as unknown as HTMLElement;
}

function fire(node: HTMLElement, type: string) {
  node.dispatchEvent(new Event(type, { cancelable: true }));
}

describe('tap', () => {
  it('counts each finger as it lands, two buttons at once included (#145)', () => {
    const pressed: string[] = [];
    const red = button();
    const blue = button();
    tap(red, () => pressed.push('red'));
    tap(blue, () => pressed.push('blue'));
    // Two fingers down together: a browser cancels both as a pinch and clicks neither.
    fire(red, 'pointerdown');
    fire(blue, 'pointerdown');
    fire(red, 'pointercancel');
    fire(blue, 'pointercancel');
    expect(pressed).toEqual(['red', 'blue']);
  });

  it('does not count the click that follows a tap a second time', () => {
    let n = 0;
    const b = button();
    tap(b, () => n++);
    fire(b, 'pointerdown');
    fire(b, 'pointerup');
    fire(b, 'click');
    expect(n).toBe(1);
  });

  it('counts a click with no finger before it, which is the keyboard', () => {
    let n = 0;
    const b = button();
    tap(b, () => n++);
    fire(b, 'click');
    expect(n).toBe(1);
  });

  it('leaves a disabled button alone', () => {
    let n = 0;
    const b = button(true);
    tap(b, () => n++);
    fire(b, 'pointerdown');
    expect(n).toBe(0);
  });
});
