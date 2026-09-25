import { describe, it, expect } from 'vitest';
import { resolveChoreIcon, CATEGORY_ICON } from './choreIcon';
import { ICON_NAMES } from './icons';

describe('resolveChoreIcon', () => {
  it('passes known icon names through', () => {
    for (const name of ICON_NAMES) expect(resolveChoreIcon(name)).toBe(name);
    expect(resolveChoreIcon(' Bed ')).toBe('bed');
  });

  it.each([
    ['🐱', 'paw'],
    ['🐈', 'paw'],
    ['🐶', 'paw'],
    ['🐕', 'paw'],
    ['🐾', 'paw'],
    ['🛏️', 'bed'],
    ['🛏', 'bed'], // without the variation selector
    ['🪥', 'tooth'],
    ['🍽️', 'dish'],
    ['📚', 'book'],
    ['📖', 'book'],
    ['👕', 'shirt'],
    ['🧺', 'shirt'],
    ['🌱', 'sprout'],
    ['🪴', 'sprout'],
    ['🧸', 'toy'],
    ['🐟', 'fish'],
    ['🐠', 'fish'],
    ['🎹', 'piano'],
    ['🗑️', 'bin'],
    ['🗑', 'bin'],
    ['🧹', 'broom'],
    ['🍦', 'cone'],
    ['🎬', 'film'],
    ['📺', 'screen'],
    ['🐈‍⬛', 'paw'], // ZWJ sequence
  ])('maps %s to %s', (emoji, icon) => {
    expect(resolveChoreIcon(emoji, 'bonus')).toBe(icon);
  });

  it('uses the first emoji of a longer string', () => {
    expect(resolveChoreIcon('🧹🧽')).toBe('broom');
    expect(resolveChoreIcon('🐶 walk')).toBe('paw');
  });

  it('falls back to the category icon', () => {
    expect(resolveChoreIcon('🦄', 'essential')).toBe(CATEGORY_ICON.essential);
    expect(resolveChoreIcon('something', 'daily')).toBe(CATEGORY_ICON.daily);
    expect(resolveChoreIcon('', 'bonus')).toBe(CATEGORY_ICON.bonus);
    expect(resolveChoreIcon(undefined)).toBe(CATEGORY_ICON.daily);
    expect(resolveChoreIcon(null, 'essential')).toBe(CATEGORY_ICON.essential);
  });
});
