import { describe, expect, it } from 'vitest';
import { chooseLayout } from './fit';

// The widest row decides for the whole list, so a list is never half one line and half
// two.
describe('chooseLayout', () => {
  it('uses two columns when the widest row fits in one of them', () => {
    expect(chooseLayout(300, 700, true, 16)).toBe('two');
  });

  it('drops to one column before it breaks a row', () => {
    expect(chooseLayout(400, 700, true, 16)).toBe('one');
  });

  it('stacks every row when even one column is too narrow', () => {
    expect(chooseLayout(420, 350, true, 16)).toBe('stacked');
    expect(chooseLayout(420, 350, false, 16)).toBe('stacked');
  });

  it('never offers two columns where the screen does not', () => {
    expect(chooseLayout(100, 700, false, 16)).toBe('one');
  });

  it('counts the gap between the columns', () => {
    expect(chooseLayout(342, 700, true, 16)).toBe('two');
    expect(chooseLayout(343, 700, true, 16)).toBe('one');
  });

  it('does not flip on a fraction of a pixel', () => {
    expect(chooseLayout(349.6, 350, false, 16)).toBe('one');
  });

  it('takes the roomiest layout when there is nothing to measure', () => {
    expect(chooseLayout(0, 700, true, 16)).toBe('two');
    expect(chooseLayout(0, 0, false, 16)).toBe('one');
  });
});
