/**
 * Format a Date as a local YYYY-MM-DD string (no timezone shifting).
 */
export function localDateStr(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

/**
 * Toggle an item in an array: remove it if present, append it if absent.
 */
export function toggleInArray<T>(arr: T[], item: T): T[] {
  return arr.includes(item) ? arr.filter(x => x !== item) : [...arr, item];
}
