export type Theme = 'default' | 'quest' | 'galaxy' | 'forest';

export interface CategoryLabels {
  required: string;
  core: string;
  bonus: string;
}

export interface SoundDef {
  notes: { freq: number; duration: number; delay: number }[];
  waveform: OscillatorType;
  gain: number;
}

export interface ThemeConfig {
  labels: CategoryLabels;
  categoryIcons: { required: string; core: string; bonus: string };
  greetings: { morning: string; afternoon: string; evening: string };
  messages: {
    choreComplete: string;
    allDone: string;
    empty: string;
    streakLabel: string;
  };
  confettiColors: string[];
  sounds: {
    complete: SoundDef;
    allDone: SoundDef;
  };
}

export const THEME_CONFIG: Record<Theme, ThemeConfig> = {
  default: {
    labels: { required: 'Essentials', core: 'Dailies', bonus: 'Bonus' },
    categoryIcons: { required: 'shield-check', core: 'circle-check', bonus: 'sparkles' },
    greetings: {
      morning: 'Good morning',
      afternoon: 'Good afternoon',
      evening: 'Good evening',
    },
    messages: {
      choreComplete: 'Nice work!',
      allDone: 'All done!',
      empty: 'No chores assigned for this period.',
      streakLabel: 'Streak',
    },
    confettiColors: ['#38bdf8', '#34d399', '#f472b6', '#a78bfa'],
    sounds: {
      complete: {
        notes: [
          { freq: 523, duration: 0.12, delay: 0 },
          { freq: 659, duration: 0.18, delay: 0.1 },
        ],
        waveform: 'sine',
        gain: 0.15,
      },
      allDone: {
        notes: [
          { freq: 523, duration: 0.12, delay: 0 },
          { freq: 659, duration: 0.12, delay: 0.1 },
          { freq: 784, duration: 0.12, delay: 0.2 },
          { freq: 1047, duration: 0.3, delay: 0.3 },
        ],
        waveform: 'sine',
        gain: 0.15,
      },
    },
  },
  quest: {
    labels: { required: 'Main Quest', core: 'Side Quest', bonus: 'Loot' },
    categoryIcons: { required: 'swords', core: 'scroll', bonus: 'coins' },
    greetings: {
      morning: 'Rise and shine',
      afternoon: 'Ready for adventure',
      evening: 'Good eve',
    },
    messages: {
      choreComplete: 'Quest complete!',
      allDone: 'All quests conquered!',
      empty: 'No quests today, hero. Rest well.',
      streakLabel: 'Quest Chain',
    },
    confettiColors: ['#fbbf24', '#f97316', '#dc2626', '#a3e635'],
    sounds: {
      complete: {
        notes: [
          { freq: 262, duration: 0.1, delay: 0 },
          { freq: 330, duration: 0.1, delay: 0.08 },
          { freq: 392, duration: 0.2, delay: 0.16 },
        ],
        waveform: 'triangle',
        gain: 0.12,
      },
      allDone: {
        notes: [
          { freq: 262, duration: 0.1, delay: 0 },
          { freq: 330, duration: 0.1, delay: 0.08 },
          { freq: 392, duration: 0.1, delay: 0.16 },
          { freq: 523, duration: 0.15, delay: 0.24 },
          { freq: 659, duration: 0.15, delay: 0.35 },
          { freq: 784, duration: 0.35, delay: 0.46 },
        ],
        waveform: 'triangle',
        gain: 0.12,
      },
    },
  },
  galaxy: {
    labels: { required: 'Critical Mission', core: 'Star Tasks', bonus: 'Discovery' },
    categoryIcons: { required: 'rocket', core: 'orbit', bonus: 'telescope' },
    greetings: {
      morning: 'Systems online',
      afternoon: 'Stellar afternoon',
      evening: 'Starlight awaits',
    },
    messages: {
      choreComplete: 'Mission accomplished!',
      allDone: 'Galaxy explored!',
      empty: 'Orbit is clear, cadet. Stand by.',
      streakLabel: 'Light Years',
    },
    confettiColors: ['#a855f7', '#818cf8', '#38bdf8', '#d946ef'],
    sounds: {
      complete: {
        notes: [
          { freq: 880, duration: 0.08, delay: 0 },
          { freq: 1100, duration: 0.06, delay: 0.06 },
          { freq: 1320, duration: 0.12, delay: 0.1 },
        ],
        waveform: 'sine',
        gain: 0.1,
      },
      allDone: {
        notes: [
          { freq: 440, duration: 0.08, delay: 0 },
          { freq: 554, duration: 0.08, delay: 0.06 },
          { freq: 659, duration: 0.08, delay: 0.12 },
          { freq: 880, duration: 0.1, delay: 0.18 },
          { freq: 1100, duration: 0.1, delay: 0.26 },
          { freq: 1320, duration: 0.25, delay: 0.34 },
        ],
        waveform: 'sine',
        gain: 0.1,
      },
    },
  },
  forest: {
    labels: { required: 'Roots', core: 'Branches', bonus: 'Leaves' },
    categoryIcons: { required: 'tree-pine', core: 'sprout', bonus: 'leaf' },
    greetings: {
      morning: 'The forest stirs',
      afternoon: 'The sun is high',
      evening: 'Twilight falls',
    },
    messages: {
      choreComplete: 'The forest grows!',
      allDone: 'Forest cleared!',
      empty: 'The grove is peaceful today.',
      streakLabel: 'Growth Ring',
    },
    confettiColors: ['#4ade80', '#22c55e', '#84cc16', '#fcd34d'],
    sounds: {
      complete: {
        notes: [
          { freq: 784, duration: 0.1, delay: 0 },
          { freq: 988, duration: 0.08, delay: 0.08 },
          { freq: 1175, duration: 0.06, delay: 0.14 },
        ],
        waveform: 'sine',
        gain: 0.08,
      },
      allDone: {
        notes: [
          { freq: 523, duration: 0.15, delay: 0 },
          { freq: 659, duration: 0.12, delay: 0.12 },
          { freq: 784, duration: 0.1, delay: 0.22 },
          { freq: 988, duration: 0.12, delay: 0.3 },
          { freq: 1175, duration: 0.25, delay: 0.4 },
        ],
        waveform: 'sine',
        gain: 0.08,
      },
    },
  },
};

// Backward compat alias
export const THEME_LABELS: Record<Theme, CategoryLabels> = Object.fromEntries(
  Object.entries(THEME_CONFIG).map(([k, v]) => [k, v.labels])
) as Record<Theme, CategoryLabels>;

export interface User {
  id: number;
  name: string;
  avatar_url: string;
  role: 'admin' | 'child';
  age?: number;
  theme: Theme;
  created_at: string;
}

export interface Chore {
  id: number;
  title: string;
  description: string;
  category: 'required' | 'core' | 'bonus';
  icon?: string;
  points_value: number;
  estimated_minutes?: number;
}

export interface ChoreSchedule {
  id: number;
  chore_id: number;
  assigned_to: number;
  assignment_type: string;
  day_of_week?: number;
  specific_date?: string;
  available_at?: string;
  due_by?: string;
  expiry_penalty: 'block' | 'no_points' | 'penalty';
  expiry_penalty_value: number;
  points_multiplier: number;
  start_date?: string;
  end_date?: string;
  recurrence_interval?: number;
  recurrence_start?: string;
  created_at: string;
}

export const DAY_NAMES = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'] as const;

// --- Points & Rewards ---

export interface PointTransaction {
  id: number;
  user_id: number;
  amount: number;
  reason: 'chore_complete' | 'chore_uncomplete' | 'reward_redeem' | 'streak_bonus' | 'admin_adjust' | 'expiry_penalty' | 'points_decay';
  reference_id?: number;
  note?: string;
  created_at: string;
}

export interface PointsData {
  balance: number;
  transactions: PointTransaction[];
}

export interface PointBalance {
  user_id: number;
  balance: number;
}

export interface RewardAssignment {
  id: number;
  reward_id: number;
  user_id: number;
  custom_cost?: number;
}

export interface Reward {
  id: number;
  name: string;
  description: string;
  icon?: string;
  cost: number;
  effective_cost: number;
  stock?: number;
  active: boolean;
  created_by: number;
  created_at: string;
  assignments?: RewardAssignment[];
}

export interface RewardRedemption {
  id: number;
  reward_id: number;
  user_id: number;
  points_spent: number;
  created_at: string;
}

// --- Streaks ---

export interface UserStreakData {
  current_streak: number;
  longest_streak: number;
  streak_start_date?: string;
  earned_rewards: StreakRewardItem[];
  next_reward?: StreakRewardItem & { days_remaining: number };
}

export interface StreakRewardItem {
  id: number;
  streak_days: number;
  bonus_points: number;
  label: string;
}

export interface ScheduledChore {
  schedule_id: number;
  chore_id: number;
  title: string;
  description: string;
  category: 'required' | 'core' | 'bonus';
  icon?: string;
  points_value: number;
  estimated_minutes?: number;
  assignment_type: string;
  available_at?: string;
  due_by?: string;
  expiry_penalty: 'block' | 'no_points' | 'penalty';
  expiry_penalty_value: number;
  available: boolean;
  expired: boolean;
  completed: boolean;
  completion_id?: number;
  completed_at?: string;
  date: string;
}

export interface UserDecayConfig {
  user_id: number;
  enabled: boolean;
  decay_rate: number;
  decay_interval_hours: number;
  last_decay_at?: string;
}

export interface RedemptionHistory {
  id: number;
  reward_name: string;
  reward_icon: string;
  points_spent: number;
  created_at: string;
}

export interface Webhook {
  id: number;
  url: string;
  secret: string;
  events: string;
  active: boolean;
  created_at: string;
}

export interface WebhookDelivery {
  id: number;
  webhook_id: number;
  event: string;
  payload: string;
  status_code?: number;
  response_body?: string;
  error?: string;
  created_at: string;
}
