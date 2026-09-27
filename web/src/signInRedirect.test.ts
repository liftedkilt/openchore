// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';
import { claimAutoSignIn, clearAutoSignIn } from './signInRedirect';

afterEach(() => sessionStorage.clear());

describe('automatic sign-in redirect', () => {
  it('redirects once, then not again within a minute', () => {
    expect(claimAutoSignIn(1_000)).toBe(true);
    expect(claimAutoSignIn(30_000)).toBe(false);
    expect(claimAutoSignIn(61_001)).toBe(true);
  });

  it('allows it again once someone has signed in', () => {
    expect(claimAutoSignIn(1_000)).toBe(true);
    clearAutoSignIn();
    expect(claimAutoSignIn(2_000)).toBe(true);
  });
});
