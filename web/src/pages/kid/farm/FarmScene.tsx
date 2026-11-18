// The family farm: every person's animal on one field. Healthy animals
// wander about, sick ones shuffle slowly, very sick ones lie down, and an
// animal whose owner missed chores hides in its shelter by the fence where
// everyone can see it.
import { useEffect, useMemo, useRef, useState } from 'react';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import type { Pet } from '../../../types';
import { FarmAnimal, petPicture, FarmBackdrop, Shelter } from './FarmAnimal';
import { Rosette } from './Rosette';
import s from './farm.module.css';

interface Spot { x: number; y: number; face: 1 | -1; ms: number; walking: boolean }

/** Field bounds, as fractions of the scene. */
const X_MIN = 0.08;
const X_MAX = 0.86;
const Y_MIN = 0.64;
const Y_MAX = 0.9;

const MOOD_BADGE: Partial<Record<Pet['mood'], string>> = {
  very_sick: '🤢', sick: '🤒', hungry: '🥕', bored: '💤', happy: '💛',
};

function prefersReducedMotion() {
  return typeof window !== 'undefined' && !!window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
}

/** A stable start spot per pet, so the farm doesn't reshuffle on reload. */
function startSpot(id: number, i: number, n: number): Spot {
  const slot = n > 1 ? i / (n - 1) : 0.5;
  const jitter = ((id * 37) % 10) / 100;
  return {
    x: X_MIN + (X_MAX - X_MIN) * Math.min(1, Math.max(0, slot * 0.9 + jitter)),
    y: Y_MIN + (Y_MAX - Y_MIN) * (((id * 53) % 100) / 100),
    face: id % 2 ? 1 : -1,
    ms: 0,
    walking: false,
  };
}

export interface FarmSceneProps {
  pets: Pet[];
  /** Highlight this person's animal. */
  meId?: number;
  compact?: boolean;
  /** Show owners' names under the animals (default: unless compact). */
  owners?: boolean;
  onSelect?: (pet: Pet) => void;
  className?: string;
}

export function FarmScene({ pets, meId, compact, owners = !compact, onSelect, className }: FarmSceneProps) {
  const { t } = useTranslation();
  const out = pets.filter(p => !p.hiding);
  const hiding = pets.filter(p => p.hiding);
  const [spots, setSpots] = useState<Record<number, Spot>>({});
  const petsRef = useRef(out);
  petsRef.current = out;
  const outKey = out.map(p => `${p.id}:${p.mood}`).join(',');

  // Place newcomers.
  useEffect(() => {
    setSpots(prev => {
      const next: Record<number, Spot> = {};
      out.forEach((p, i) => { next[p.id] = prev[p.id] ?? startSpot(p.id, i, out.length); });
      return next;
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [outKey]);

  // Wander: each animal picks a nearby spot every few seconds.
  useEffect(() => {
    if (prefersReducedMotion()) return;
    const timers: ReturnType<typeof setTimeout>[] = [];
    const step = (id: number) => {
      const p = petsRef.current.find(x => x.id === id);
      if (!p) return;
      const lying = p.mood === 'very_sick';
      if (!lying) {
        setSpots(prev => {
          const cur = prev[id];
          if (!cur) return prev;
          const range = p.sick ? 0.08 : 0.22;
          const x = Math.min(X_MAX, Math.max(X_MIN, cur.x + (Math.random() * 2 - 1) * range));
          const y = Math.min(Y_MAX, Math.max(Y_MIN, cur.y + (Math.random() * 2 - 1) * range * 0.5));
          const dist = Math.hypot(x - cur.x, (y - cur.y) * 2);
          const ms = Math.max(600, dist * (p.sick ? 22000 : 7000));
          const face: 1 | -1 = Math.abs(x - cur.x) < 0.01 ? cur.face : x > cur.x ? 1 : -1;
          timers.push(setTimeout(() => setSpots(pr => (pr[id] ? { ...pr, [id]: { ...pr[id], walking: false } } : pr)), ms));
          return { ...prev, [id]: { x, y, face, ms, walking: true } };
        });
      }
      const pause = (p.sick ? 6000 : 2500) + Math.random() * 4000;
      timers.push(setTimeout(() => step(id), pause));
    };
    petsRef.current.forEach((p, i) => timers.push(setTimeout(() => step(p.id), 400 + i * 700)));
    return () => timers.forEach(clearTimeout);
  }, [outKey]);

  const ordered = useMemo(
    () => [...out].sort((a, b) => (spots[a.id]?.y ?? 0) - (spots[b.id]?.y ?? 0)),
    [out, spots],
  );

  const label = (p: Pet) => t(`kid.farm.mood.${p.mood}`, { name: p.name });

  const tag = (p: Pet) => (
    <span className={s.tag} style={{ ['--tag' as string]: p.owner_color ? `var(--person-${p.owner_color})` : undefined }}>
      <span className={s.tagName}>
        {p.ribbon && <Rosette className={s.rosette} title={t('kid.board.ribbon')} />}
        {p.name}
      </span>
      {owners && p.owner_name && <span className={s.tagOwner}>{p.owner_name}</span>}
      {p.visiting && <span className={s.tagOwner}>{t('kid.farm.visiting', { family: p.family ?? '' })}</span>}
    </span>
  );

  return (
    <div className={clsx(s.farm, compact && s.compact, className)} role="group" aria-label={t('kid.farm.title')}>
      <FarmBackdrop />

      {hiding.length > 0 && (
        <ul className={s.shelters} aria-label={t('kid.farm.hidingTitle')}>
          {hiding.map(p => (
            <li key={p.id} className={clsx(s.shelterItem, p.user_id === meId && s.mine)}>
              <button
                type="button"
                className={s.shelterBtn}
                onClick={onSelect ? () => onSelect(p) : undefined}
                disabled={!onSelect}
                aria-label={label(p)}
                title={label(p)}
              >
                <Shelter className={s.shelter} peeking />
                {tag(p)}
              </button>
            </li>
          ))}
        </ul>
      )}

      {ordered.map(p => {
        const sp = spots[p.id];
        if (!sp) return null;
        const depth = (sp.y - Y_MIN) / (Y_MAX - Y_MIN);
        const badge = MOOD_BADGE[p.mood];
        return (
          <button
            key={p.id}
            type="button"
            className={clsx(s.animal, sp.walking && s.walking, p.sick && s.sick, p.mood === 'very_sick' && s.lying, p.user_id === meId && s.mine)}
            style={{
              left: `${sp.x * 100}%`,
              top: `${sp.y * 100}%`,
              zIndex: 10 + Math.round(depth * 50),
              transitionDuration: `${sp.ms}ms`,
              ['--depth' as string]: (0.85 + depth * 0.3).toFixed(2),
            }}
            onClick={onSelect ? () => onSelect(p) : undefined}
            disabled={!onSelect}
            aria-label={label(p)}
            title={label(p)}
          >
            {badge && <span className={s.badge} aria-hidden="true">{badge}</span>}
            <span className={s.body} style={{ transform: `scaleX(${sp.face})` }}>
              <FarmAnimal species={p.species} color={p.color} mood={p.mood} accessory={p.accessory} {...petPicture(p)} className={s.svg} />
            </span>
            {tag(p)}
          </button>
        );
      })}

      {pets.length === 0 && <p className={s.empty}>{t('kid.farm.empty')}</p>}
    </div>
  );
}
