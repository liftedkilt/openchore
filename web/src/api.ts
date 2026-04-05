import type { User, ScheduledChore, Chore, ChoreSchedule, PointsData, PointBalance, Reward, RewardAssignment, RewardRedemption, RedemptionHistory, UserStreakData, StreakRewardItem, ChoreTrigger, Webhook, WebhookDelivery, UserDecayConfig, APIToken } from './types';

const API_BASE = '/api';

// Resize images client-side before uploading to avoid Nginx body size limits
// and reduce upload time on mobile. Returns a JPEG file ≤ maxDim on longest side.
function resizeImage(file: File, maxDim: number): Promise<File> {
  return new Promise((resolve) => {
    // Skip if already small enough (< 500KB)
    if (file.size < 500_000) {
      resolve(file);
      return;
    }
    const img = new Image();
    img.onload = () => {
      URL.revokeObjectURL(img.src);
      const { width, height } = img;
      if (width <= maxDim && height <= maxDim) {
        resolve(file);
        return;
      }
      const scale = maxDim / Math.max(width, height);
      const canvas = document.createElement('canvas');
      canvas.width = Math.round(width * scale);
      canvas.height = Math.round(height * scale);
      const ctx = canvas.getContext('2d')!;
      ctx.drawImage(img, 0, 0, canvas.width, canvas.height);
      canvas.toBlob(
        (blob) => {
          resolve(new File([blob!], file.name.replace(/\.\w+$/, '.jpg'), { type: 'image/jpeg' }));
        },
        'image/jpeg',
        0.85,
      );
    };
    img.onerror = () => resolve(file); // fallback to original on error
    img.src = URL.createObjectURL(file);
  });
}

export class APIError extends Error {
  status: number;
  data: any;
  constructor(message: string, status: number, data: any) {
    super(message);
    this.name = 'APIError';
    this.status = status;
    this.data = data;
  }
}

async function fetchWithAuth<T>(path: string, options: RequestInit = {}, skipContentType = false): Promise<T> {
  const userStr = localStorage.getItem('openchore_user');
  const headers: Record<string, string> = {
    ...(skipContentType ? {} : { 'Content-Type': 'application/json' }),
    ...(userStr ? { 'X-User-ID': JSON.parse(userStr).id.toString() } : {}),
    ...options.headers as Record<string, string>,
  };

  const resp = await fetch(`${API_BASE}${path}`, { ...options, headers });
  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ error: 'Unknown error' }));
    throw new APIError(err.error || `HTTP error! status: ${resp.status}`, resp.status, err);
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
    updateLineColor: (id: number, line_color: string) =>
      fetchWithAuth<User>(`/users/${id}/line-color`, {
        method: 'PUT',
        body: JSON.stringify({ line_color }),
      }),
    pause: (id: number) =>
      fetchWithAuth<User>(`/users/${id}/pause`, { method: 'PUT' }),
    unpause: (id: number) =>
      fetchWithAuth<User>(`/users/${id}/unpause`, { method: 'PUT' }),
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
    complete: (scheduleId: number, date: string, photoUrl?: string) =>
      fetchWithAuth(`/schedules/${scheduleId}/complete`, {
        method: 'POST',
        body: JSON.stringify({ completion_date: date, photo_url: photoUrl }),
      }),
    uncomplete: (scheduleId: number, date: string) =>
      fetchWithAuth(`/schedules/${scheduleId}/complete?date=${date}`, {
        method: 'DELETE',
      }),
    upload: async (file: File) => {
      const resized = await resizeImage(file, 1280);
      const formData = new FormData();
      formData.append('photo', resized);
      return fetchWithAuth<{ url: string }>('/upload', {
        method: 'POST',
        body: formData,
      }, true);
    },
    attachPhoto: (completionId: number, photoUrl: string) =>
      fetchWithAuth(`/completions/${completionId}/photo`, {
        method: 'PUT',
        body: JSON.stringify({ photo_url: photoUrl }),
      }),
    listPending: () => fetchWithAuth<any[]>('/completions/pending'),
    approve: (completionId: number) => fetchWithAuth(`/completions/${completionId}/approve`, { method: 'POST' }),
    reject: (completionId: number) => fetchWithAuth(`/completions/${completionId}/reject`, { method: 'POST' }),
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
  reports: {
    get: (period: string, date: string) =>
      fetchWithAuth<any>(`/admin/reports?period=${period}&date=${date}`),
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
    getSetting: (key: string) => fetchWithAuth<{ key: string; value: string }>(`/admin/settings/${key}`),
    setSetting: (key: string, value: string) => fetchWithAuth<{ key: string; value: string }>(`/admin/settings/${key}`, {
      method: 'PUT',
      body: JSON.stringify({ value }),
    }),
    testAIReview: (choreTitle: string, photoUrl: string) =>
      fetchWithAuth<{ complete: boolean; confidence: number; feedback: string; feedback_audio: string }>('/admin/ai/test', {
        method: 'POST',
        body: JSON.stringify({ chore_title: choreTitle, photo_url: photoUrl }),
      }),
    synthesizeTTS: (text: string) =>
      fetchWithAuth<{ audio_url: string }>('/admin/ai/tts', {
        method: 'POST',
        body: JSON.stringify({ text }),
      }),
    triggerTTSSync: () =>
      fetchWithAuth<{ status: string }>('/admin/ai/tts-sync', { method: 'POST' }),
    generateDescription: (title: string, category: string) =>
      fetchWithAuth<{ description: string }>('/admin/ai/generate-description', {
        method: 'POST',
        body: JSON.stringify({ title, category }),
      }),
    suggestPoints: (title: string, description: string, category: string) =>
      fetchWithAuth<{ points: number; estimated_minutes: number; reasoning: string }>('/admin/ai/suggest-points', {
        method: 'POST',
        body: JSON.stringify({ title, description, category }),
      }),
    getAISummary: (userId: number, period: string, date: string) =>
      fetchWithAuth<{ summary: string }>(`/admin/reports/ai-summary?user_id=${userId}&period=${period}&date=${date}`),
    getAISettings: () => Promise.all([
      fetchWithAuth<{ key: string; value: string }>('/admin/settings/ai_enabled').catch(() => ({ key: 'ai_enabled', value: 'false' })),
      fetchWithAuth<{ key: string; value: string }>('/admin/settings/ai_auto_approve_threshold').catch(() => ({ key: 'ai_auto_approve_threshold', value: '0.85' })),
      fetchWithAuth<{ key: string; value: string }>('/admin/settings/ai_tts_enabled').catch(() => ({ key: 'ai_tts_enabled', value: 'false' })),
    ]).then(settings => Object.fromEntries(settings.map(s => [s.key, s.value]))),
    exportConfig: async (sections: string[]) => {
      const userStr = localStorage.getItem('openchore_user');
      const headers: Record<string, string> = userStr
        ? { 'X-User-ID': JSON.parse(userStr).id.toString() }
        : {};
      const resp = await fetch(`${API_BASE}/admin/export-config?sections=${sections.join(',')}`, { headers });
      if (!resp.ok) throw new Error('export failed');
      return resp.blob();
    },
  },
  setup: (data: { children: { name: string; theme: string }[]; chores: { title: string; icon: string; category: string; points_value: number }[] }) =>
    fetchPublic<{ admin: User; children: User[] }>('/setup', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  triggers: {
    listForChore: (choreId: number) =>
      fetchWithAuth<ChoreTrigger[]>(`/chores/${choreId}/triggers`),
    create: (choreId: number, data: Partial<ChoreTrigger>) =>
      fetchWithAuth<ChoreTrigger>(`/chores/${choreId}/triggers`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    update: (triggerId: number, data: Partial<ChoreTrigger>) =>
      fetchWithAuth<ChoreTrigger>(`/triggers/${triggerId}`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    delete: (triggerId: number) =>
      fetchWithAuth(`/triggers/${triggerId}`, { method: 'DELETE' }),
  },
  tokens: {
    list: () => fetchWithAuth<APIToken[]>('/admin/tokens'),
    create: (name: string) => fetchWithAuth<{ id: number; name: string; token: string }>('/admin/tokens', {
      method: 'POST',
      body: JSON.stringify({ name }),
    }),
    revoke: (id: number) => fetchWithAuth('/admin/tokens/' + id, { method: 'DELETE' }),
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
