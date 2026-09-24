// The OpenChore line icon set (docs/design-system/icons.svg.html) as typed
// shape data. Every icon sits on a 24px grid with round caps and joins; the
// stroke weight comes from the --icon-stroke dial.
//
// `bin` and `broom` were added for common chore emoji (🗑️, 🧹) in the same
// style as the rest of the set.

export type IconShape =
  | { tag: 'path'; d: string; fill?: string }
  | { tag: 'circle'; cx: number; cy: number; r: number; fill?: string }
  | { tag: 'rect'; x: number; y: number; width: number; height: number; rx?: number; fill?: string };

const p = (d: string): IconShape => ({ tag: 'path', d });
const c = (cx: number, cy: number, r: number, fill?: string): IconShape => ({ tag: 'circle', cx, cy, r, fill });
const r = (x: number, y: number, width: number, height: number, rx?: number): IconShape => ({ tag: 'rect', x, y, width, height, rx });

export const ICONS = {
  // Chore objects
  bed: [p('M3 18v-6.5A2.5 2.5 0 0 1 5.5 9h13a2.5 2.5 0 0 1 2.5 2.5V18M3 14.5h18M3 18v2M21 18v2'), p('M6 9V7.5A1.5 1.5 0 0 1 7.5 6h3A1.5 1.5 0 0 1 12 7.5V9')],
  tooth: [p('M8 3C5.5 3 4 4.9 4 7.2c0 2.3 1 3.6 1.6 5.6C6.3 15.2 6.5 21 8.5 21c1.7 0 1.6-5 3.5-5s1.8 5 3.5 5c2 0 2.2-5.8 2.9-8.2.6-2 1.6-3.3 1.6-5.6C20 4.9 18.5 3 16 3c-1.6 0-2.3 1-4 1S9.6 3 8 3z')],
  paw: [c(6.5, 10.5, 1.7), c(10, 6.5, 1.7), c(14, 6.5, 1.7), c(17.5, 10.5, 1.7), p('M12 11.5c-2.6 0-5 3.2-5 5.6 0 1.6 1.2 2.4 2.6 2.4 1 0 1.6-.5 2.4-.5s1.4.5 2.4.5c1.4 0 2.6-.8 2.6-2.4 0-2.4-2.4-5.6-5-5.6z')],
  dish: [c(12, 12, 8.5), c(12, 12, 4.5)],
  book: [p('M3.5 5.5c2.8-.9 5.6-.6 8.5 1.2v13c-2.9-1.8-5.7-2.1-8.5-1.2zM20.5 5.5c-2.8-.9-5.6-.6-8.5 1.2v13c2.9-1.8 5.7-2.1 8.5-1.2z')],
  shirt: [p('M8.5 4 3.5 6.8l1.8 3.9 2.2-1V20h9V9.7l2.2 1 1.8-3.9L15.5 4c-.6 1.4-1.9 2.2-3.5 2.2S9.1 5.4 8.5 4z')],
  sprout: [p('M12 20.5V12'), p('M12 12c0-4 3-6.5 7.5-6.5 0 4.2-3 6.5-7.5 6.5zM12 14.5C12 11 9.5 9 5 9c0 3.4 2.5 5.5 7 5.5z'), p('M7.5 20.5h9')],
  toy: [r(4, 11, 16, 9, 2), p('M8 11V8.5a1.5 1.5 0 0 1 3 0V11M13 11V8.5a1.5 1.5 0 0 1 3 0V11')],
  fish: [p('M3 12c2.5-4 6-5.5 9.5-5.5 3.5 0 6 2.5 7.5 5.5-1.5 3-4 5.5-7.5 5.5C9 17.5 5.5 16 3 12z'), p('M3 12 1.5 9M3 12l-1.5 3'), c(16, 11, 0.6, 'currentColor')],
  piano: [r(3, 5, 18, 14, 2), p('M8 19v-6M12 19v-6M16 19v-6M7 5v8h2V5M11 5v8h2V5M15 5v8h2V5')],
  table: [p('M3 9.5h18M5 9.5 4 20M19 9.5l1 10.5M8.5 5.5v4M12 4v5.5M15.5 5.5v4')],
  bin: [p('M4.5 7h15M9.5 7V5a1 1 0 0 1 1-1h3a1 1 0 0 1 1 1v2'), p('M6.5 7l.9 12.1A1.5 1.5 0 0 0 8.9 20.5h6.2a1.5 1.5 0 0 0 1.5-1.4L17.5 7M10.2 10.5v6.5M13.8 10.5v6.5')],
  broom: [p('M20.5 3.5 13 11'), p('M10.5 8.5l5 5-4.5 7c-3.5-1-6.5-4-7.5-7.5z'), p('M9.5 13.5 6 17M12.2 15.8 9.3 19.4')],
  // Status and rewards
  check: [p('M5 12.5l4.5 4.5L19 7.5')],
  flame: [p('M12 21c3.9 0 7-2.8 7-6.6 0-3.4-2.4-5.4-3.6-8.4-.3 2-1.3 3.2-2.4 3.6C13 6 11.5 4 9 3c.4 3-1 5-2.4 6.8C5.8 11 5 12.4 5 14.4 5 18.2 8.1 21 12 21z')],
  star: [p('M12 3.5l2.6 5.3 5.8.8-4.2 4.1 1 5.8-5.2-2.7-5.2 2.7 1-5.8-4.2-4.1 5.8-.8z')],
  spark: [p('M12 3c.7 5 3.9 8.2 9 9-5.1.8-8.3 4-9 9-.7-5-3.9-8.2-9-9 5.1-.8 8.3-4 9-9z')],
  gift: [r(4, 9, 16, 11, 1.5), p('M3 9h18M12 9v11M12 9c-1.5-3-5.5-4-5.5-1.5S10 9 12 9zm0 0c1.5-3 5.5-4 5.5-1.5S14 9 12 9z')],
  camera: [p('M4 8.5A1.5 1.5 0 0 1 5.5 7h2l1.5-2h6l1.5 2h2A1.5 1.5 0 0 1 20 8.5v9a1.5 1.5 0 0 1-1.5 1.5h-13A1.5 1.5 0 0 1 4 17.5z'), c(12, 13, 3.5)],
  clock: [c(12, 12, 8.5), p('M12 7.5V12l3 2')],
  home: [p('M4 11l8-7 8 7v8.5a1 1 0 0 1-1 1h-4.5V15h-5v5.5H5a1 1 0 0 1-1-1z')],
  cal: [r(4, 5, 16, 15, 2.5), p('M4 10h16M8.5 3v4M15.5 3v4')],
  sound: [p('M4 9.5h3.5L12 6v12l-4.5-3.5H4z'), p('M15.5 9a4 4 0 0 1 0 6M18 6.5a7.5 7.5 0 0 1 0 11')],
  lock: [r(5, 10.5, 14, 10, 2.5), p('M8 10.5V8a4 4 0 0 1 8 0v2.5')],
  bike: [c(5.5, 16, 3.5), c(18.5, 16, 3.5), p('M5.5 16 9 9.5h6.5l3 6.5M9 9.5l3 6.5H5.5M14 6.5h2.5l-1 3')],
  rocket: [p('M12 2.5c3 2.2 4.5 5.4 4.5 9.2V16h-9v-4.3c0-3.8 1.5-7 4.5-9.2z'), c(12, 9.5, 1.8), p('M7.5 12.5 5 15v3.5l2.5-2.5M16.5 12.5 19 15v3.5L16.5 16M10 19.5l2 2 2-2')],
  film: [r(3.5, 5, 17, 14, 2.5), p('M3.5 9.5h17M8 5l2 4.5M13 5l2 4.5')],
  bowl: [p('M3.5 11h17a8.5 8.5 0 0 1-17 0z'), p('M9 7.5c0-1.5 1-2 1-3.5M13 7.5c0-1.5 1-2 1-3.5')],
  moon: [p('M19.5 14.5A8 8 0 1 1 9.5 4.5a6.5 6.5 0 0 0 10 10z')],
  cone: [p('M7 10.5h10L12 21.5z'), p('M7 10.5a5 5 0 0 1 10 0')],
  screen: [r(3, 5, 18, 12, 2), p('M9 20.5h6M12 17v3.5')],
  // UI
  back: [p('M14.5 6l-6 6 6 6')],
  chev: [p('M9.5 6l6 6-6 6')],
  plus: [p('M12 5v14M5 12h14')],
  people: [c(9, 8.5, 3.5), p('M2.5 20c.6-3.6 3.2-5.5 6.5-5.5s5.9 1.9 6.5 5.5'), p('M15.5 5.2a3.5 3.5 0 0 1 0 6.6M18 14.8c2 .7 3.2 2.5 3.5 5.2')],
} satisfies Record<string, IconShape[]>;

export type IconName = keyof typeof ICONS;
export const ICON_NAMES = Object.keys(ICONS) as IconName[];

export function isIconName(v: unknown): v is IconName {
  return typeof v === 'string' && Object.prototype.hasOwnProperty.call(ICONS, v);
}
