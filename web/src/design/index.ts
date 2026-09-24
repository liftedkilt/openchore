// OpenChore design system. Importing this module installs the fonts, tokens,
// dials and component styles (see ./styles).
import './styles';

export * from './types';
export { ICONS, ICON_NAMES, isIconName, type IconName, type IconShape } from './icons';
export { Icon, type IconProps } from './Icon';
export { resolveChoreIcon, CATEGORY_ICON } from './choreIcon';
export {
  resolveSkin, resolveHouseTheme, isScheduledDark, msUntilNextMinute, salutationFor,
  BLOCKS_UNDER_AGE, DARK_FROM_MINUTES, DARK_UNTIL_MINUTES,
  type HouseThemeInput, type Salutation,
} from './theme';
export {
  Greeting, Avatar, blobFor, PointsChip, CategoryMark, CategoryHeader, Button, FamilyMember,
  type GreetingProps, type AvatarProps, type AvatarSize, type PointsChipProps, type CategoryMarkProps,
  type CategoryHeaderProps, type ButtonProps, type FamilyMemberProps,
} from './primitives';
export { ChoreRow, type ChoreRowProps } from './ChoreRow';
export { DayProgress, type DayProgressProps, type DayProgressItem } from './DayProgress';
export { TabBar, TABS, type TabBarProps, type TabLinkProps } from './TabBar';
export { Celebration, type CelebrationProps } from './Celebration';
export {
  SkinScope, HouseScope, useHouseTheme, usePrefersDark, useMinuteClock,
  type SkinScopeProps, type HouseScopeProps,
} from './Scopes';
