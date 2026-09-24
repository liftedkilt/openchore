// A person's skin (users.theme). An empty theme resolves by age on the client.
export type Theme = 'sunroom' | 'blocks' | 'tint';

// A person's colour key (users.color). Each theme defines its own shade.
export type PersonColor = 'coral' | 'mint' | 'butter' | 'sky' | 'rose' | 'leaf' | 'lilac' | 'sand';

export const PERSON_COLORS: readonly PersonColor[] = ['coral', 'mint', 'butter', 'sky', 'rose', 'leaf', 'lilac', 'sand'];

export interface SoundDef {
  notes: { freq: number; duration: number; delay: number }[];
  waveform: OscillatorType;
  gain: number;
}

// What a skin may change about feedback. Words, categories and greetings are
// shared by every skin (design i18n); only sounds and the buzz differ.
export interface ThemeConfig {
  sounds: {
    complete: SoundDef;
    allDone: SoundDef;
  };
  /** navigator.vibrate pattern on finishing a chore. */
  vibrate: number | number[];
}

export const THEME_CONFIG: Record<Theme, ThemeConfig> = {
  sunroom: {
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
    vibrate: 50,
  },
  blocks: {
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
    vibrate: [30, 40, 30],
  },
  tint: {
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
    vibrate: 40,
  },
};

export interface User {
  id: number;
  name: string;
  avatar_url: string;
  role: 'admin' | 'child';
  age?: number;
  theme: Theme;
  line_color?: string;
  color?: PersonColor;
  paused: boolean;
  has_pin: boolean;
  // IDs of OIDC providers linked to this profile ("Continue with ...").
  auth_providers: string[];
  created_at: string;
}

export interface SessionInfo {
  method: 'tap' | 'pin' | 'oidc' | 'upload';
  expires_at: string;
  // Personal-device (OIDC) sessions persist and skip the kiosk idle logout.
  persistent: boolean;
  provider?: string;
}

export interface AuthSession {
  user: User;
  session: SessionInfo;
}

export interface AuthProvider {
  id: string;
  name: string;
}

export interface LinkedIdentity {
  id: number;
  user_id: number;
  provider: string;
  email?: string;
  display_name?: string;
  created_at: string;
  last_login_at?: string;
}

export interface Chore {
  id: number;
  title: string;
  description: string;
  category: 'required' | 'core' | 'bonus';
  icon?: string;
  points_value: number;
  missed_penalty_value: number;
  estimated_minutes?: number;
  requires_approval: boolean;
  requires_photo: boolean;
  photo_source?: 'child' | 'external' | 'both';
  tts_description?: string;
  tts_audio_url?: string;
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
  reason: 'chore_complete' | 'chore_uncomplete' | 'reward_redeem' | 'streak_bonus' | 'admin_adjust' | 'expiry_penalty' | 'points_decay' | 'missed_chore' | 'commit_to_goal' | 'goal_break';
  reference_id?: number;
  note?: string;
  created_at: string;
}

export interface RewardCommitment {
  id: number;
  user_id: number;
  reward_id: number;
  reward_name?: string;
  reward_icon?: string;
  target_cost: number;
  amount_saved: number;
  auto_contribute_percent: number;
  status: 'active' | 'redeemed' | 'cancelled';
  shared_pool_id?: number;
  pool?: SharedCommitmentPool;
  created_at: string;
  redeemed_at?: string;
  cancelled_at?: string;
}

export interface PointsData {
  balance: number;
  committed: number;
  active_commitments: RewardCommitment[];
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
  shareable: boolean;
  created_by: number;
  created_at: string;
  assignments?: RewardAssignment[];
}

export interface PoolContributor {
  user_id: number;
  user_name: string;
  avatar_url?: string;
  amount_saved: number;
}

export interface SharedCommitmentPool {
  id: number;
  reward_id: number;
  reward_name?: string;
  reward_icon?: string;
  target_cost: number;
  amount_saved: number;
  status: 'active' | 'redeemed' | 'cancelled';
  contributors?: PoolContributor[];
  created_at: string;
  redeemed_at?: string;
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
  missed_penalty_value: number;
  estimated_minutes?: number;
  requires_approval: boolean;
  requires_photo: boolean;
  photo_source?: 'child' | 'external' | 'both';
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
  photo_url?: string;
  date: string;
  completion_status?: 'approved' | 'pending' | 'rejected' | 'ai_rejected' | 'excused';
  ai_feedback?: string;
  completed_by_name?: string;
  completed_by_sibling?: boolean;
  tts_description?: string;
  tts_audio_url?: string;
}

// Shape of rows returned by GET /api/completions/pending. Exposed as a typed
// surface (the admin UI cares about assigned_user_id to attribute pending
// approvals to the kid the chore belongs to, not just whoever clicked it).
export interface PendingCompletion {
  id: number;
  chore_title: string;
  child_name: string;
  assigned_user_id: number;
  photo_url: string;
  completion_date: string;
  completed_at: string;
  // Who clicked "complete" (child_name is their name).
  completed_by?: number;
  category?: 'required' | 'core' | 'bonus';
  icon?: string;
  points_value?: number;
  ai_feedback?: string;
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

export interface ChoreTrigger {
  id: number;
  uuid: string;
  chore_id: number;
  default_assigned_to?: number;
  default_due_by?: string;
  default_available_at?: string;
  enabled: boolean;
  cooldown_minutes: number;
  assignment_type: string;
  last_triggered_at?: string;
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

export interface APIToken {
  id: number;
  name: string;
  last_used_at?: string;
  revoked: boolean;
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

export interface AIReviewError {
  error: string;
  ai_review: {
    complete: boolean;
    confidence: number;
    feedback: string;
    feedback_audio?: string;
  };
}
