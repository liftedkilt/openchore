import { useEffect, useState } from 'react';
import { api } from '../api';
import type { AIStatus } from '../types';

const NOT_CONFIGURED: AIStatus = { ai: { configured: false }, tts: { configured: false } };

// The server's AI configuration only changes when a parent edits it in
// Settings, so one request per page load is enough until refreshAIStatus().
let cached: Promise<AIStatus> | null = null;
const listeners = new Set<(s: AIStatus) => void>();

function load(): Promise<AIStatus> {
  if (!cached) {
    cached = api.admin.aiStatus().catch(() => {
      cached = null;
      return NOT_CONFIGURED;
    });
  }
  return cached;
}

/** Re-fetches the AI status for every mounted useAIStatus (after the
 * connection settings change). */
export function refreshAIStatus(): void {
  cached = null;
  load().then(s => listeners.forEach(l => l(s)));
}

/** Which optional AI services are configured. Reports nothing configured
 * until the answer arrives (or if it can't be fetched). */
export function useAIStatus(): AIStatus {
  const [status, setStatus] = useState<AIStatus>(NOT_CONFIGURED);
  useEffect(() => {
    let live = true;
    const update = (s: AIStatus) => { if (live) setStatus(s); };
    listeners.add(update);
    load().then(update);
    return () => { live = false; listeners.delete(update); };
  }, []);
  return status;
}
