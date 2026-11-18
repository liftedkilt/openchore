// Line icon for a pet-care reward, shown as the small badge on PetRewardThumb.
// One fixed map from pet_action to the icon set; anything unmapped falls back
// to the usual reward-icon resolution.
import type { Reward } from '../../../types';
import type { IconName } from '../../../design/icons';
import { resolveChoreIcon } from '../../../design/choreIcon';

const PET_ACTION_ICON: Record<string, IconName> = {
  feed: 'carrot',
  treat: 'apple',
  play: 'ball',
  restyle: 'brush',
  design: 'pencil',
  picture: 'camera',
  new_animal: 'swap',
  accessory: 'hat',
};

export function petRewardIcon(reward: Reward): IconName {
  const mapped = reward.pet_action ? PET_ACTION_ICON[reward.pet_action] : undefined;
  return mapped ?? resolveChoreIcon(reward.icon, 'bonus');
}
