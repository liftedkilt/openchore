// A pet-care reward pictured on the person's own animal: wearing the
// accessory, or with a line icon for the action (food, ball, paint…) beside it.
import clsx from 'clsx';
import type { Pet, Reward } from '../../../types';
import { Icon } from '../../../design/Icon';
import { FarmAnimal } from './FarmAnimal';
import { petRewardIcon } from './petIcon';
import { previewLook } from './petLook';
import s from './petSheet.module.css';

export function PetRewardThumb({ pet, reward, className }: { pet: Pet; reward: Reward; className?: string }) {
  const showBadge = reward.pet_action !== 'accessory';
  return (
    <span className={clsx(s.thumb, className)} title={reward.name}>
      <FarmAnimal species={pet.species} color={pet.color} {...previewLook(pet, reward)} className={s.thumbArt} />
      {showBadge && <span className={s.thumbBadge} aria-hidden="true"><Icon name={petRewardIcon(reward)} size={14} /></span>}
    </span>
  );
}
