import type { User, ScheduledChore, Chore, ChoreSchedule, PointsData, PointBalance, Reward, RewardAssignment, RewardRedemption, RedemptionHistory, UserStreakData, StreakRewardItem, Webhook, WebhookDelivery, UserDecayConfig } from './types';

const API_BASE = '/api';

async function fetchWithAuth<T>(path: string, options: RequestInit = {}): Promise<T> {
  const userStr = localStorage.getItem('openchore_user');
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(userStr ? { 'X-User-ID': JSON.parse(userStr).id.toString() } : {}),
    ...options.headers as Record<string, string>,
  };

  const resp = await fetch(`${API_BASE}${path}`, { ...options, headers });
  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ error: 'Unknown error' }));
    throw new Error(err.error || `HTTP error! status: ${resp.status}`);
  }
  if (resp.status === 204) return {} as T;
  return resp.json();
}

async function fetchPublic<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...options.headers as Record<string, string>,
  };
  const resp = await fetch(`${API_BASE}${path}`, { ...options, headers });
  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ error: 'Unknown error' }));
    throw new Error(err.error || `HTTP error! status: ${resp.status}`);
  }
  if (resp.status === 204) return {} as T;
  return resp.json();
}

// Fetch as a specific user (for ambient dashboard)
export async function fetchAsUser<T>(userId: number, path: string): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    'X-User-ID': userId.toString(),
  };
  const resp = await fetch(`${API_BASE}${path}`, { headers });
  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ error: 'Unknown error' }));
    throw new Error(err.error || `HTTP error! status: ${resp.status}`);
  }
  return resp.json();
}

export const api = {
  users: {
    list: () => fetchPublic<User[]>('/users'),
    get: (id: number) => fetchPublic<User>(`/users/${id}`),
    create: (data: Partial<User>) => fetchWithAuth<User>('/users', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
    update: (id: number, data: Partial<User>) => fetchWithAuth<User>(`/users/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
    delete: (id: number) => fetchWithAuth(`/users/${id}`, { method: 'DELETE' }),
    updateTheme: (id: number, theme: string) =>
      fetchWithAuth<User>(`/users/${id}/theme`, {
        method: 'PUT',
        body: JSON.stringify({ theme }),
      }),
    updateAvatar: (id: number, avatar_url: string) =>
      fetchWithAuth<User>(`/users/${id}/avatar`, {
        method: 'PUT',
        body: JSON.stringify({ avatar_url }),
      }),
    getChores: (id: number, view: 'daily' | 'weekly', date: string) =>
      fetchWithAuth<ScheduledChore[]>(`/users/${id}/chores?view=${view}&date=${date}`),
  },
  chores: {
    list: () => fetchWithAuth<Chore[]>('/chores'),
    get: (id: number) => fetchWithAuth<Chore>(`/chores/${id}`),
    create: (data: Partial<Chore>) => fetchWithAuth<Chore>('/chores', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
    update: (id: number, data: Partial<Chore>) => fetchWithAuth<Chore>(`/chores/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
    delete: (id: number) => fetchWithAuth(`/chores/${id}`, { method: 'DELETE' }),
    complete: (scheduleId: number, date: string) =>
      fetchWithAuth(`/schedules/${scheduleId}/complete`, {
        method: 'POST',
        body: JSON.stringify({ completion_date: date }),
      }),
    uncomplete: (scheduleId: number, date: string) =>
      fetchWithAuth(`/schedules/${scheduleId}/complete?date=${date}`, {
        method: 'DELETE',
      }),
    listSchedules: (choreId: number) =>
      fetchWithAuth<ChoreSchedule[]>(`/chores/${choreId}/schedules`),
    createSchedule: (choreId: number, data: Partial<ChoreSchedule>) =>
      fetchWithAuth<ChoreSchedule>(`/chores/${choreId}/schedules`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    deleteSchedule: (choreId: number, scheduleId: number) =>
      fetchWithAuth(`/chores/${choreId}/schedules/${scheduleId}`, { method: 'DELETE' }),
  },
  points: {
    getForUser: (userId: number) => fetchWithAuth<PointsData>(`/users/${userId}/points`),
    getAllBalances: () => fetchWithAuth<PointBalance[]>('/points/balances'),
    adjust: (userId: number, amount: number, note: string) =>
      fetchWithAuth('/points/adjust', {
        method: 'POST',
        body: JSON.stringify({ user_id: userId, amount, note }),
      }),
  },
  rewards: {
    list: () => fetchWithAuth<Reward[]>('/rewards'),
    listAll: () => fetchWithAuth<Reward[]>('/rewards/all'),
    create: (data: Partial<Reward>) => fetchWithAuth<Reward>('/rewards', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
    update: (id: number, data: Partial<Reward>) => fetchWithAuth<Reward>(`/rewards/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
    delete: (id: number) => fetchWithAuth(`/rewards/${id}`, { method: 'DELETE' }),
    setAssignments: (id: number, assignments: { user_id: number; custom_cost?: number }[]) =>
      fetchWithAuth(`/rewards/${id}/assignments`, {
        method: 'PUT',
        body: JSON.stringify({ assignments }),
      }),
    redeem: (id: number) => fetchWithAuth<RewardRedemption>(`/rewards/${id}/redeem`, { method: 'POST' }),
    listRedemptions: (userId: number) => fetchWithAuth<RedemptionHistory[]>(`/users/${userId}/redemptions`),
    undoRedemption: (redemptionId: number) => fetchWithAuth(`/redemptions/${redemptionId}`, { method: 'DELETE' }),
  },
  streaks: {
    getForUser: (userId: number) => fetchWithAuth<UserStreakData>(`/users/${userId}/streak`),
    listRewards: () => fetchWithAuth<StreakRewardItem[]>('/admin/streak-rewards'),
    createReward: (data: Partial<StreakRewardItem>) =>
      fetchWithAuth<StreakRewardItem>('/admin/streak-rewards', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    deleteReward: (id: number) => fetchWithAuth(`/admin/streak-rewards/${id}`, { method: 'DELETE' }),
  },
  decay: {
    getConfig: (userId: number) => fetchWithAuth<UserDecayConfig>(`/admin/users/${userId}/decay`),
    setConfig: (userId: number, data: Partial<UserDecayConfig>) =>
      fetchWithAuth<UserDecayConfig>(`/admin/users/${userId}/decay`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
  },
  admin: {
    verifyPasscode: (passcode: string) =>
      fetchPublic<{ valid: boolean }>('/admin/verify', {
        method: 'POST',
        body: JSON.stringify({ passcode }),
      }),
    updatePasscode: (oldPasscode: string, newPasscode: string) =>
      fetchWithAuth('/admin/passcode', {
        method: 'PUT',
        body: JSON.stringify({ old_passcode: oldPasscode, new_passcode: newPasscode }),
      }),
  },
  webhooks: {
    list: () => fetchWithAuth<Webhook[]>('/admin/webhooks'),
    create: (data: { url: string; secret?: string; events?: string }) =>
      fetchWithAuth<Webhook>('/admin/webhooks', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    update: (id: number, data: Partial<Webhook>) =>
      fetchWithAuth<Webhook>(`/admin/webhooks/${id}`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    delete: (id: number) => fetchWithAuth(`/admin/webhooks/${id}`, { method: 'DELETE' }),
    listDeliveries: (id: number) => fetchWithAuth<WebhookDelivery[]>(`/admin/webhooks/${id}/deliveries`),
  },
};
