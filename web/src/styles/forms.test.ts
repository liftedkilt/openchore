import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

// Issue #20 regression tests. CreateChoreWizard and EditChoreModal used to
// duplicate their form CSS verbatim. The fix extracted the shared rules into
// styles/forms.module.css and wires each component's module to inherit from
// it via the CSS Modules `composes:` directive. These tests statically
// validate that wiring so a future edit that deletes the shared file, removes
// a `composes:` directive, or re-inlines the duplicated rules will fail CI.

const here = new URL('.', import.meta.url).pathname;
const sharedCssPath = resolve(here, 'forms.module.css');
const wizardCssPath = resolve(
  here,
  '../components/CreateChoreWizard/CreateChoreWizard.module.css'
);
const editCssPath = resolve(
  here,
  '../components/EditChoreModal/EditChoreModal.module.css'
);

const sharedCss = readFileSync(sharedCssPath, 'utf8');
const wizardCss = readFileSync(wizardCssPath, 'utf8');
const editCss = readFileSync(editCssPath, 'utf8');

// The 8 classes that were identical between the two components before the
// refactor and now live in the shared module.
const SHARED_CLASSES = [
  'formGrid',
  'formRow',
  'formGroup',
  'label',
  'input',
  'checkRow',
  'checkLabel',
  'btnPrimary',
] as const;

// Match `.name {` as a rule declaration (start of line, optional whitespace).
function hasRule(css: string, name: string): boolean {
  return new RegExp(`(^|\\n)\\s*\\.${name}\\s*[{,]`).test(css);
}

// Match `composes: <name> from '...forms.module.css'` anywhere in `css`.
function composesFromShared(css: string, name: string): boolean {
  const pattern = new RegExp(
    `composes:\\s*${name}\\s+from\\s+['\"][^'\"]*styles/forms\\.module\\.css['\"]`
  );
  return pattern.test(css);
}

describe('shared form CSS module (#20)', () => {
  it('defines every class that CreateChoreWizard and EditChoreModal compose from', () => {
    for (const name of SHARED_CLASSES) {
      expect(hasRule(sharedCss, name), `forms.module.css missing .${name}`).toBe(true);
    }
  });

  it('each shared class has non-empty declarations (not a stub)', () => {
    for (const name of SHARED_CLASSES) {
      // Capture the body of `.name { ... }` and require at least one `:`.
      const body = new RegExp(`\\.${name}\\s*{([^}]*)}`).exec(sharedCss)?.[1] ?? '';
      expect(body.includes(':'), `forms.module.css .${name} has empty body`).toBe(true);
    }
  });
});

describe('CreateChoreWizard composes from shared form CSS (#20)', () => {
  for (const name of SHARED_CLASSES) {
    it(`.${name} composes from styles/forms.module.css`, () => {
      expect(composesFromShared(wizardCss, name)).toBe(true);
    });
  }

  it('does not re-inline the shared declarations that should now be composed', () => {
    // Guard against a regression where someone copies the rules back in.
    // Each of these properties was part of the shared base and should NOT
    // appear on the local .input rule anymore (padding/min-height/font-size
    // stay local as legitimate overrides).
    const inputRule = /\.input\s*{([^}]*)}/.exec(wizardCss)?.[1] ?? '';
    expect(inputRule).not.toMatch(/\bbackground:\s*rgba\(0,\s*0,\s*0/);
    expect(inputRule).not.toMatch(/\bborder:\s*1px\s+solid\s+rgba\(255/);
  });
});

describe('EditChoreModal composes from shared form CSS (#20)', () => {
  for (const name of SHARED_CLASSES) {
    it(`.${name} composes from styles/forms.module.css`, () => {
      expect(composesFromShared(editCss, name)).toBe(true);
    });
  }

  it('does not re-inline the shared declarations that should now be composed', () => {
    const inputRule = /\.input\s*{([^}]*)}/.exec(editCss)?.[1] ?? '';
    expect(inputRule).not.toMatch(/\bbackground:\s*rgba\(0,\s*0,\s*0/);
    expect(inputRule).not.toMatch(/\bborder:\s*1px\s+solid\s+rgba\(255/);
  });
});
