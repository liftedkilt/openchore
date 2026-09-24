// OpenChore Redesign: shared components. Every skin renders these same components;
// set data-theme ("sunroom" | "blocks" | "tint") and data-person on the screen root.
type Cat = 'essential' | 'daily' | 'bonus';
type PersonColor = 'coral' | 'mint' | 'butter' | 'sky';

export function Greeting(p: { salutation?: string; name: string }): JSX.Element;
export function Avatar(p: { name: string; color?: PersonColor; size?: 'sm' | 'md' | 'lg' }): JSX.Element;
export function PointsChip(p: { points: number }): JSX.Element;
export function CategoryHeader(p: { cat: Cat; count?: string; label?: string }): JSX.Element;
export function CategoryMark(p: { cat: Cat; done?: boolean }): JSX.Element;
export function ChoreRow(p: {
  cat?: Cat; icon?: string; title: string; meta?: string; points?: number;
  state?: 'todo' | 'done' | 'waiting' | 'locked'; urgent?: boolean; photo?: boolean; readAloud?: boolean;
}): JSX.Element;
export function DayProgress(p: { items: { cat: Cat; done?: boolean; at?: number }[]; now?: number; from?: string; to?: string }): JSX.Element;
export function Button(p: { variant?: 'primary' | 'quiet'; block?: boolean; icon?: string; onClick?: () => void; children: React.ReactNode }): JSX.Element;
export function TabBar(p: { active?: 'today' | 'week' | 'rewards' }): JSX.Element;
export function Celebration(p: { points: number; title: string; streak?: number; next?: string; headline?: string }): JSX.Element;
export function FamilyMember(p: { name: string; color?: PersonColor; done: number; total: number }): JSX.Element;
export function Icon(p: { name: string; size?: number; fill?: boolean }): JSX.Element;
/** Presentation only: a phone frame for mockups. Sets data-theme and data-person on itself. */
export function DeviceFrame(p: { theme: 'sunroom' | 'blocks' | 'tint'; person?: PersonColor; time?: string; children?: React.ReactNode }): JSX.Element;
export const ICON_NAMES: string[];
