// The animal's reaction to pet care: food flies in and it munches, a ball
// bounces and it hops, a new look sparkles. Plays for a couple of seconds
// whenever `reaction` changes (a new key).
import { useEffect, useMemo, useState } from 'react';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import type { TFunction } from 'i18next';
import type { Pet, PetAction } from '../../../types';
import { FarmAnimal, type PetReaction as FaceReaction } from './FarmAnimal';
import s from './reaction.module.css';

export interface ReactionEvent {
  kind: PetAction | 'wear';
  key: number;
}

const DURATION = 2600;

/** Food / joy changes per action (keep in step with internal/pet Effect). */
const DELTAS: Partial<Record<PetAction, { food: number; joy: number }>> = {
  feed: { food: 40, joy: 5 },
  treat: { food: 15, joy: 20 },
  play: { food: -10, joy: 35 },
};

const PROP: Partial<Record<ReactionEvent['kind'], string>> = { feed: '🥕', treat: '🍎', play: '⚾' };
const BURST: Record<ReactionEvent['kind'], string[]> = {
  feed: ['✨', '😋', '💛'],
  treat: ['💖', '💕', '✨', '💖'],
  play: ['⭐', '✨', '💫', '🎉'],
  accessory: ['✨', '⭐', '✨', '💫'],
  design: ['🎨', '✨', '⭐', '✨'],
  wear: ['✨', '✨'],
  restyle: ['🎨', '🌈', '✨'],
  new_animal: ['💨', '✨', '💨', '⭐'],
};
const MOTION: Record<ReactionEvent['kind'], string> = {
  feed: s.munch, treat: s.munch, play: s.hop, accessory: s.pose, design: s.pose, wear: s.pose, restyle: s.pose, new_animal: s.poof,
};
const FACE: Record<ReactionEvent['kind'], FaceReaction> = {
  feed: 'eat', treat: 'eat', play: 'play', accessory: 'love', design: 'love', wear: 'love', restyle: 'love', new_animal: 'love',
};

interface ReactingAnimalProps {
  pet: Pet;
  reaction: ReactionEvent | null;
  /** Accessory to show instead of the worn one (try-on). */
  accessory?: string;
  className?: string;
  artClassName?: string;
}

export function ReactingAnimal({ pet, reaction, accessory, className, artClassName }: ReactingAnimalProps) {
  const { t } = useTranslation();
  const [active, setActive] = useState<ReactionEvent | null>(null);

  useEffect(() => {
    if (!reaction) return;
    setActive(reaction);
    const id = setTimeout(() => setActive(prev => (prev?.key === reaction.key ? null : prev)), DURATION);
    return () => clearTimeout(id);
  }, [reaction]);

  // Particle positions, fixed per reaction.
  const particles = useMemo(() => {
    if (!active) return [];
    return BURST[active.kind].concat(BURST[active.kind]).map((emoji, i) => ({
      emoji,
      x: 20 + ((i * 37 + active.key) % 60),
      delay: i * 120,
      drift: ((i * 53) % 40) - 20,
    }));
  }, [active]);

  const lying = pet.mood === 'very_sick';
  const delta = active && DELTAS[active.kind as PetAction];

  return (
    <div className={clsx(s.stage, className)}>
      <div className={clsx(s.body, active && !lying && MOTION[active.kind])} key={active?.key}>
        <FarmAnimal
          species={pet.species}
          color={pet.color}
          mood={pet.hiding ? 'bored' : pet.mood}
          accessory={accessory ?? pet.accessory}
          reaction={active && !lying ? FACE[active.kind] : undefined}
          className={artClassName}
        />
      </div>
      {active && PROP[active.kind] && (
        <span key={`p${active.key}`} className={clsx(s.prop, active.kind === 'play' ? s.ball : s.food)} aria-hidden="true">
          {PROP[active.kind]}
        </span>
      )}
      {particles.map((p, i) => (
        <span key={`${active?.key}-${i}`} className={s.particle} aria-hidden="true"
          style={{ left: `${p.x}%`, animationDelay: `${300 + p.delay}ms`, ['--drift' as string]: `${p.drift}px` }}>
          {p.emoji}
        </span>
      ))}
      {delta && (
        <span key={`d${active?.key}`} className={s.deltas} aria-live="polite">
          {delta.food !== 0 && <span>{t('kid.farm.react.food', { count: delta.food, sign: delta.food > 0 ? '+' : '' })}</span>}
          {delta.joy !== 0 && <span>{t('kid.farm.react.joy', { count: delta.joy, sign: delta.joy > 0 ? '+' : '' })}</span>}
        </span>
      )}
    </div>
  );
}

/** A short line for the reaction, e.g. "Biscuit loved the treat!". */
export function reactionLine(t: TFunction, pet: Pet, kind: ReactionEvent['kind']) {
  return t(`kid.farm.react.line.${kind}`, { name: pet.name });
}
