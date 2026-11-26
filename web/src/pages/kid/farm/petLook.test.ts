import { describe, expect, it } from 'vitest';
import type { Pet, Reward } from '../../../types';
import { characterPicture, petRewardOwned } from './petLook';
import { safeImageHref } from './FarmAnimal';

const pet = { items: ['straw_hat'], character: 'dragon' } as unknown as Pet;
const reward = (pet_action: Reward['pet_action'], pet_item?: string) => ({ pet_action, pet_item } as Reward);

describe('character rewards', () => {
  it('shows the character the reward gives, from the server', () => {
    const pic = characterPicture(reward('character', 'unicorn'));
    expect(pic).toEqual({ sprite: '/api/pets/characters/unicorn.png', spritePreview: true, spriteFrames: 6 });
    expect(safeImageHref(pic?.sprite)).toBe('/api/pets/characters/unicorn.png');
    expect(characterPicture(reward('picture'))).toBeNull();
  });

  it('counts the character the animal already is, and owned accessories, as owned', () => {
    expect(petRewardOwned(pet, reward('character', 'dragon'))).toBe(true);
    expect(petRewardOwned(pet, reward('character', 'yeti'))).toBe(false);
    expect(petRewardOwned(pet, reward('accessory', 'straw_hat'))).toBe(true);
    expect(petRewardOwned(pet, reward('accessory', 'bow'))).toBe(false);
    expect(petRewardOwned(pet, reward('feed'))).toBe(false);
  });

  it('still refuses picture addresses that are not ours', () => {
    expect(safeImageHref('/api/pets/characters/../../x.png')).toBeUndefined();
    expect(safeImageHref('https://example.com/api/pets/characters/dragon.png')).toBeUndefined();
  });
});
