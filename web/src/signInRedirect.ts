// Sending a signed-out visitor straight to a sign-in provider (the "start
// page" option) must not loop: if the provider sends them back still signed
// out (a cookie the browser refused, a provider error page left with Back),
// the next visit within a minute shows the family picker instead.
const KEY = 'openchore_auto_sign_in';
const WINDOW_MS = 60_000;

/** True when an automatic redirect is allowed now; records that it happened. */
export function claimAutoSignIn(now = Date.now()): boolean {
  try {
    const last = Number(sessionStorage.getItem(KEY));
    if (last && now - last < WINDOW_MS) return false;
    sessionStorage.setItem(KEY, String(now));
  } catch {
    // Storage unavailable: redirect anyway rather than never.
  }
  return true;
}

/** Forget the last redirect once someone is signed in. */
export function clearAutoSignIn(): void {
  try {
    sessionStorage.removeItem(KEY);
  } catch {
    // ignore
  }
}
