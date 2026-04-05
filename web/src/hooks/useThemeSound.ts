import { useRef, useCallback, useEffect } from 'react';
import { useTheme } from '../ThemeContext';
import type { SoundDef } from '../types';

function playNotes(ctx: AudioContext, def: SoundDef) {
  const now = ctx.currentTime;
  for (const { freq, duration, delay } of def.notes) {
    const osc = ctx.createOscillator();
    const gain = ctx.createGain();
    osc.type = def.waveform;
    osc.frequency.value = freq;
    gain.gain.setValueAtTime(def.gain, now + delay);
    gain.gain.exponentialRampToValueAtTime(0.001, now + delay + duration);
    osc.connect(gain).connect(ctx.destination);
    osc.start(now + delay);
    osc.stop(now + delay + duration);
  }
}

export function useThemeSound() {
  const { config } = useTheme();
  const ctxRef = useRef<AudioContext | null>(null);

  const getCtx = () => {
    if (!ctxRef.current) ctxRef.current = new AudioContext();
    if (ctxRef.current.state === 'suspended') ctxRef.current.resume();
    return ctxRef.current;
  };

  const playComplete = useCallback(() => {
    try { playNotes(getCtx(), config.sounds.complete); } catch { /* no audio support */ }
  }, [config]);

  const playAllDone = useCallback(() => {
    try { playNotes(getCtx(), config.sounds.allDone); } catch { /* no audio support */ }
  }, [config]);

  useEffect(() => {
    return () => {
      ctxRef.current?.close();
      ctxRef.current = null;
    };
  }, []);

  return { playComplete, playAllDone };
}
