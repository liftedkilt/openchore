import { describe, it, expect } from 'vitest';
import { PERSON_COLORS } from './types';

// Vitest runs in Node, but the app tsconfig carries no Node types. The tokens
// live outside web/ (docs/design-system), so read them at test time rather
// than importing them: the web image builds with web/ as its only context.
type Fs = { readFileSync: (path: string, encoding: 'utf8') => string };
const fsModule: string = 'node:fs';
const { readFileSync } = (await import(/* @vite-ignore */ fsModule)) as Fs;

type Token = { name: string; value: string | Record<string, string> };
type Tokens = { color: { themes: { id: string }[]; tokens: Token[] } };
const tokens = JSON.parse(
  readFileSync(new URL('../../../docs/design-system/tokens.json', import.meta.url).pathname, 'utf8'),
) as Tokens;

// WCAG 2.x relative luminance and contrast ratio.
function luminance(hex: string): number {
  const m = /^#([0-9a-f]{6})$/i.exec(hex);
  if (!m) throw new Error(`not a #rrggbb colour: ${hex}`);
  const [r, g, b] = [0, 2, 4].map((i) => {
    const c = parseInt(m[1].slice(i, i + 2), 16) / 255;
    return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrast(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

const THEMES = tokens.color.themes.map((t) => t.id);
const COLOR: Record<string, Record<string, string>> = Object.fromEntries(
  tokens.color.tokens.map((t) => [t.name, t.value as Record<string, string>]),
);

// Text pairs from the token notes: 4.5:1 in every theme.
const TEXT: [fg: string, bg: string][] = [
  ['ink', 'surface'],
  ['ink', 'surface-raised'],
  ['ink', 'surface-sunk'],
  ['ink-muted', 'surface'],
  ['ink-muted', 'surface-raised'],
  ['ink-muted', 'surface-sunk'],
  ['on-accent', 'accent'],
  ['on-chip', 'chip'],
  ['ink', 'celebrate'],
  ['on-done', 'done'],
  ['waiting', 'surface'],
  ['waiting', 'surface-raised'],
  ['urgent', 'surface'],
  ['urgent', 'surface-raised'],
  ['on-row-essential', 'row-essential'],
  ['on-row-daily', 'row-daily'],
  ['on-row-bonus', 'row-bonus'],
  ...PERSON_COLORS.map((c): [string, string] => ['on-person', `person-${c}`]),
];

// Meaningful graphics: 3:1 in every theme, except the two documented
// graphic-only exceptions below.
const GRAPHIC: [fg: string, bg: string][] = [
  ['highlight', 'surface'],
  ['done', 'surface-raised'],
  ['ink-muted', 'surface-raised'], // the empty check ring
  ['cat-essential-icon', 'cat-essential-soft'],
  ['cat-daily-icon', 'cat-daily-soft'],
  ['cat-bonus-icon', 'cat-bonus-soft'],
];

// README, Accessibility: Sunroom's highlight arc always sits beside a written
// count, and Blocks' done mint is always drawn inside an ink outline.
const EXCEPTIONS = new Set(['sunroom:highlight/surface', 'blocks:done/surface-raised']);

describe('design tokens', () => {
  it('has a value for every theme on every colour token', () => {
    for (const t of tokens.color.tokens) {
      for (const theme of THEMES) expect(t.value, `${t.name} in ${theme}`).toHaveProperty(theme);
    }
  });

  it('defines all eight person colours', () => {
    for (const c of PERSON_COLORS) expect(COLOR).toHaveProperty(`person-${c}`);
  });

  for (const theme of THEMES) {
    describe(theme, () => {
      it.each(TEXT)('text %s on %s holds 4.5:1', (fg, bg) => {
        const ratio = contrast(COLOR[fg][theme], COLOR[bg][theme]);
        expect(ratio, `${fg} on ${bg} in ${theme}`).toBeGreaterThanOrEqual(4.5);
      });

      it.each(GRAPHIC)('graphic %s on %s holds 3:1', (fg, bg) => {
        const ratio = contrast(COLOR[fg][theme], COLOR[bg][theme]);
        if (EXCEPTIONS.has(`${theme}:${fg}/${bg}`)) {
          // Documented exception: must stay an exception on purpose.
          expect(ratio).toBeLessThan(3);
          return;
        }
        expect(ratio, `${fg} on ${bg} in ${theme}`).toBeGreaterThanOrEqual(3);
      });
    });
  }

  it("keeps Tint's person colours readable as its accent, ring and done check", () => {
    // On a person's own Tint screen, accent / highlight / done become --you.
    for (const c of PERSON_COLORS) {
      const you = COLOR[`person-${c}`].tint;
      expect(contrast(COLOR['on-accent'].tint, you), `on-accent on ${c}`).toBeGreaterThanOrEqual(4.5);
      expect(contrast(COLOR['on-done'].tint, you), `on-done on ${c}`).toBeGreaterThanOrEqual(4.5);
      expect(contrast(you, COLOR.surface.tint), `${c} ring on surface`).toBeGreaterThanOrEqual(3);
    }
  });

  it('keeps Blocks outlines at 3:1', () => {
    expect(contrast(COLOR.line.blocks, COLOR['surface-raised'].blocks)).toBeGreaterThanOrEqual(3);
  });
});
