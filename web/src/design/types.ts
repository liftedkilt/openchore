// Types for the OpenChore design system. These deliberately don't import
// from ../types so the design layer stays independent of the API model.

/** A person's own skin. House is never a personal skin. */
export type Skin = 'sunroom' | 'blocks' | 'tint';
export const SKINS: readonly Skin[] = ['sunroom', 'blocks', 'tint'];

/** The neutral frame for shared screens, by day and by night. */
export type HouseTheme = 'house' | 'house-dark';

/** Every value `data-theme` can take. */
export type ThemeId = Skin | HouseTheme;

/** How a HouseScope picks between House and House Dark. */
export type HouseMode = 'auto' | 'light' | 'dark';

/** A person's colour key (stored in `users.color`), not a hex value. */
export type PersonColor = 'coral' | 'mint' | 'butter' | 'sky' | 'rose' | 'leaf' | 'lilac' | 'sand';
export const PERSON_COLORS: readonly PersonColor[] = ['coral', 'mint', 'butter', 'sky', 'rose', 'leaf', 'lilac', 'sand'];

export function isSkin(v: unknown): v is Skin {
  return typeof v === 'string' && (SKINS as readonly string[]).includes(v);
}

export function isPersonColor(v: unknown): v is PersonColor {
  return typeof v === 'string' && (PERSON_COLORS as readonly string[]).includes(v);
}

/** Design category names. The DB uses required / core / bonus. */
export type Cat = 'essential' | 'daily' | 'bonus';
export const CATS: readonly Cat[] = ['essential', 'daily', 'bonus'];

/** Map a DB chore category (`required` | `core` | `bonus`) to a design category. */
export function catFromCategory(category: string | null | undefined): Cat {
  switch (category) {
    case 'required':
    case 'essential':
      return 'essential';
    case 'bonus':
      return 'bonus';
    default:
      return 'daily';
  }
}

export type ChoreState = 'todo' | 'done' | 'waiting' | 'locked';
export type TabId = 'today' | 'week' | 'rewards';
