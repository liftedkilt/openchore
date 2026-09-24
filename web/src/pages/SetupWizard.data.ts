import { PERSON_COLORS, type PersonColor } from '../design';

/** The first colour nobody has yet (or the first colour, if all are taken). */
export function firstFreeColor(taken: readonly (PersonColor | null | undefined)[]): PersonColor {
  return PERSON_COLORS.find((c) => !taken.includes(c)) ?? PERSON_COLORS[0];
}
