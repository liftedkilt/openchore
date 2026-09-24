import { useEffect, useRef } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';

const IDLE_TIMEOUT = 5 * 60 * 1000; // 5 minutes
const EVENTS = ['mousedown', 'mousemove', 'keydown', 'touchstart', 'scroll'];

interface IdleOptions {
  // Runs before redirecting (e.g. ending a shared-device session).
  onIdle?: () => Promise<void> | void;
  // Personal devices never idle out.
  disabled?: boolean;
}

export function useIdleRedirect(targetPath: string, excludePaths: string[] = [], { onIdle, disabled = false }: IdleOptions = {}) {
  const navigate = useNavigate();
  const location = useLocation();
  const timerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const excludeRef = useRef(excludePaths);
  excludeRef.current = excludePaths;
  const onIdleRef = useRef(onIdle);
  onIdleRef.current = onIdle;

  useEffect(() => {
    if (disabled) return;

    // Don't set up idle redirect if already on the target page
    if (location.pathname === targetPath) return;

    // Don't set up idle redirect if on an excluded path
    if (excludeRef.current.some(p => location.pathname.startsWith(p))) return;

    const reset = () => {
      if (timerRef.current) clearTimeout(timerRef.current);
      timerRef.current = setTimeout(async () => {
        if (onIdleRef.current) await onIdleRef.current();
        navigate(targetPath);
      }, IDLE_TIMEOUT);
    };

    reset();
    for (const event of EVENTS) {
      window.addEventListener(event, reset, { passive: true });
    }

    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
      for (const event of EVENTS) {
        window.removeEventListener(event, reset);
      }
    };
  }, [navigate, location.pathname, targetPath, disabled]);
}
