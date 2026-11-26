// Display helpers for farm-pet looks: accessory and coat names.
import type { TFunction } from 'i18next';
import { PET_COLORS, PET_ITEMS, characterPictureUrl, type Pet, type PetItem, type PetSpecies, type Reward } from '../../../types';

/** A custom emoji accessory, "e:<slot>:<emoji>". */
export function emojiItem(item?: string): { slot: string; emoji: string } | null {
  const m = item?.match(/^e:(head|face|neck):(.+)$/u);
  return m ? { slot: m[1], emoji: m[2] } : null;
}

/** A name for an accessory: the built-in's translated name, or the emoji. */
export function itemLabel(t: TFunction, item: PetItem): string {
  const e = emojiItem(item);
  if (e) return e.emoji;
  if (item.startsWith('img:')) return t('kid.farm.design.mine');
  return PET_ITEMS.includes(item) ? t(`kid.farm.item.${item}`) : item;
}

/** The pet's coat, with "" read as the species' default. */
export function petColor(species: PetSpecies, color?: string): string {
  return color && PET_COLORS[species].includes(color) ? color : PET_COLORS[species][0];
}

export const colorLabel = (t: TFunction, color: string) => t(`kid.farm.color.${color}`);

/** Rewards that need a pick (animal and/or coat) before redeeming. */
export const needsPick = (r: Reward) => r.pet_action === 'new_animal' || r.pet_action === 'restyle' || r.pet_action === 'design' || r.pet_action === 'picture' || r.pet_action === 'gift' || r.pet_action === 'playdate';

/** A pet-care reward the animal already has: an accessory it owns, or the character it already is. */
export function petRewardOwned(pet: Pet, r: Reward): boolean {
  if (!r.pet_item) return false;
  if (r.pet_action === 'accessory') return pet.items.includes(r.pet_item);
  return r.pet_action === 'character' && pet.character === r.pet_item;
}

/** The picture a "character" reward would give, as FarmAnimal props; null for any other reward. */
export function characterPicture(r: Reward): { sprite: string; spritePreview: true; spriteFrames: number } | null {
  return r.pet_action === 'character' && r.pet_item ? { sprite: characterPictureUrl(r.pet_item), spritePreview: true, spriteFrames: 6 } : null;
}

/** How a pet-care reward would make the person's animal look. */
export function previewLook(pet: Pet, r: Reward): { accessory?: string; mood: Pet['mood'] } {
  if (r.pet_action === 'accessory' && r.pet_item) return { accessory: r.pet_item, mood: 'happy' };
  return { accessory: pet.accessory, mood: r.pet_action === 'feed' || r.pet_action === 'treat' || r.pet_action === 'play' || r.pet_action === 'character' ? 'happy' : 'content' };
}
