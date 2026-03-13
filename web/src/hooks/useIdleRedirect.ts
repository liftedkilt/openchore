import { useEffect, useRef } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';

const IDLE_TIMEOUT = 5 * 60 * 1000; // 5 minutes
const EVENTS = ['mousedown', 'mousemove', 'keydown', 'touchstart', 'scroll'];

export function useIdleRedirect(targetPath: string) {
  const navigate = useNavigate();
  const location = useLocation();
  const timerRef = useRef<ReturnType<typeof setTimeout>>();

  useEffect(() => {
    // Don't set up idle redirect if already on the target page
    if (location.pathname === targetPath) return;

    const reset = () => {
      if (timerRef.current) clearTimeout(timerRef.current);
      timerRef.current = setTimeout(() => {
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
  }, [navigate, location.pathname, targetPath]);
}
