// A pet-care reward pictured on the person's own animal: wearing the
// accessory, or with the reward's icon (food, ball, paint…) beside it.
import clsx from 'clsx';
import type { Pet, Reward } from '../../../types';
import { FarmAnimal } from './FarmAnimal';
import { previewLook } from './petLook';
import s from './petSheet.module.css';

export function PetRewardThumb({ pet, reward, className }: { pet: Pet; reward: Reward; className?: string }) {
  const badge = reward.pet_action !== 'accessory' ? reward.icon : undefined;
  return (
    <span className={clsx(s.thumb, className)} title={reward.name}>
      <FarmAnimal species={pet.species} color={pet.color} {...previewLook(pet, reward)} className={s.thumbArt} />
      {badge && <span className={s.thumbBadge} aria-hidden="true">{badge}</span>}
    </span>
  );
}
