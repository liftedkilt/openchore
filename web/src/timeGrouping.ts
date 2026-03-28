import type { ScheduledChore } from './types';

export type TimePeriod = 'morning' | 'afternoon' | 'evening';

export interface TimePeriodConfig {
  key: TimePeriod;
  label: string;
  emoji: string;
  startHour: number;
}

export interface TimePeriodGroup extends TimePeriodConfig {
  chores: ScheduledChore[];
  nextStartHour: number;
}

export const TIME_PERIOD_CONFIG: TimePeriodConfig[] = [
  { key: 'morning', label: 'Morning', emoji: '\u{1F305}', startHour: 0 },
  { key: 'afternoon', label: 'Afternoon', emoji: '\u2600\uFE0F', startHour: 12 },
  { key: 'evening', label: 'Evening', emoji: '\u{1F319}', startHour: 17 },
];

/**
 * Classify a time string (HH:MM format from available_at) into a time period.
 * - Morning: before 12:00
 * - Afternoon: 12:00 - 16:59
 * - Evening: 17:00+
 * Null/undefined/empty values default to morning.
 */
export function getTimePeriod(availableAt?: string): TimePeriod {
  if (!availableAt) return 'morning';
  const [hrs] = availableAt.split(':').map(Number);
  if (hrs < 12) return 'morning';
  if (hrs < 17) return 'afternoon';
  return 'evening';
}

/**
 * Check if a time period is the currently active one.
 * Active means current hour is >= startHour and < nextStartHour.
 */
export function isTimePeriodActive(startHour: number, nextStartHour: number, now: Date = new Date()): boolean {
  const hour = now.getHours();
  return hour >= startHour && hour < nextStartHour;
}

/**
 * Check if a time period has already passed.
 * Past means current hour is >= the next period's start hour.
 */
export function isTimePeriodPast(nextStartHour: number, now: Date = new Date()): boolean {
  return now.getHours() >= nextStartHour;
}

/**
 * Group chores into time period buckets based on their available_at field.
 * Empty groups are filtered out.
 */
export function groupChoresByTimePeriod(list: ScheduledChore[]): TimePeriodGroup[] {
  const groups: Record<TimePeriod, ScheduledChore[]> = {
    morning: [],
    afternoon: [],
    evening: [],
  };
  list.forEach(c => {
    groups[getTimePeriod(c.available_at)].push(c);
  });
  return TIME_PERIOD_CONFIG
    .map((period, idx) => ({
      ...period,
      chores: groups[period.key],
      nextStartHour: TIME_PERIOD_CONFIG[idx + 1]?.startHour ?? 24,
    }))
    .filter(g => g.chores.length > 0);
}
