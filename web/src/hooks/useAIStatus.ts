import { useEffect, useState } from 'react';
import { api } from '../api';
import type { AIStatus } from '../types';

const NOT_CONFIGURED: AIStatus = { ai: { configured: false }, tts: { configured: false } };

// The server's AI configuration only changes on restart, so one request per
// page load is enough.
let cached: Promise<AIStatus> | null = null;

/** Which optional AI services are configured. Reports nothing configured
 * until the answer arrives (or if it can't be fetched). */
export function useAIStatus(): AIStatus {
  const [status, setStatus] = useState<AIStatus>(NOT_CONFIGURED);
  useEffect(() => {
    if (!cached) {
      cached = api.admin.aiStatus().catch(() => {
        cached = null;
        return NOT_CONFIGURED;
      });
    }
    let live = true;
    cached.then(s => { if (live) setStatus(s); });
    return () => { live = false; };
  }, []);
  return status;
}
