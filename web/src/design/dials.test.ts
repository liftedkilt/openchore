import { describe, it, expect } from 'vitest';

// Vitest runs in Node, but the app tsconfig carries no Node types.
type Fs = { readFileSync: (path: URL, encoding: 'utf8') => string };
const fsModule: string = 'node:fs';
const { readFileSync } = (await import(/* @vite-ignore */ fsModule)) as Fs;
const themesCss = readFileSync(new URL('./themes.css', import.meta.url), 'utf8');
const components = readFileSync(new URL('./components.css', import.meta.url), 'utf8');

const themes = themesCss.replace(/\/\*[\s\S]*?\*\//g, '');

/** The body of the first rule whose selector list contains `selector`. */
function block(selector: string): string {
  const re = /([^{}]+)\{([^}]*)\}/g;
  for (let m = re.exec(themes); m; m = re.exec(themes)) {
    if (m[1].split(',').map((s) => s.trim()).includes(selector)) return m[2];
  }
  throw new Error(`no rule for ${selector}`);
}

describe('--color-scheme dial', () => {
  it('is light on light grounds and dark on dark ones', () => {
    const expected: Record<string, string> = {
      '[data-theme="house"]': 'light',
      '[data-theme="house-dark"]': 'dark',
      '[data-theme="sunroom"]': 'light',
      '[data-theme="blocks"]': 'light',
      '[data-theme="tint"]': 'dark',
    };
    for (const [sel, scheme] of Object.entries(expected)) {
      const all = [...themes.matchAll(/([^{}]+)\{([^}]*)\}/g)]
        .filter((m) => m[1].split(',').map((x: string) => x.trim()).includes(sel))
        .map((m) => /--color-scheme:\s*(\w+)/.exec(m[2])?.[1])
        .filter(Boolean);
      expect(all.at(-1), sel).toBe(scheme);
    }
    expect(block(':root')).toMatch(/--color-scheme:\s*light/);
  });

  it('is applied by every scope', () => {
    expect(components).toMatch(/\.oc-scope \{[^}]*color-scheme: var\(--color-scheme/);
  });
});
