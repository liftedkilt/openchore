import { describe, it, expect } from 'vitest';
import { personColorVar, spreadLabels } from './personColor';
import { niceStep } from './LineChart';

describe('personColorVar', () => {
  it('maps a colour key to its person token', () => {
    expect(personColorVar('mint')).toBe('var(--person-mint)');
  });
  it('falls back to muted ink for unknown or missing colours (never a hex)', () => {
    expect(personColorVar('#ff0000')).toBe('var(--ink-muted)');
    expect(personColorVar(undefined)).toBe('var(--ink-muted)');
  });
});

describe('spreadLabels', () => {
  it('keeps labels at least `gap` apart, in input order', () => {
    expect(spreadLabels([50, 52, 10], 16, 0, 200)).toEqual([50, 66, 10]);
  });
  it('pushes labels back inside the bottom edge', () => {
    expect(spreadLabels([100, 100], 16, 0, 100)).toEqual([84, 100]);
  });
});

describe('niceStep', () => {
  it('rounds to 1, 2 or 5 times a power of ten', () => {
    expect(niceStep(0.3)).toBe(1);
    expect(niceStep(1.5)).toBe(2);
    expect(niceStep(3)).toBe(5);
    expect(niceStep(7)).toBe(10);
    expect(niceStep(23)).toBe(50);
  });
});
