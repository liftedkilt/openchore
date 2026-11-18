// Original side-view farm animals drawn in SVG, facing right (the scene flips
// them to walk left). Every species is its own little rig: a body, a big round
// head, four (or two) legs and a set of anchors (eyes, mouth, hat, collar) that
// the shared face and accessory components read, so every mood and accessory
// lands correctly on every animal. Flat, friendly children's-book style with
// soft outlines in a darker tint of each animal's own colour.
//
// Colours live in a per-species palette (one per coat colour) that the rig is
// merged with, so a black horse gets a black mane and a red cow a white face
// without any species drawing needing to know which coat it is wearing.
import { useId } from 'react';
import type { PetMood, PetSpecies } from '../../../types';
import s from './farm.module.css';

const INK = '#2b2522';
const LIGHT_INK = '#f6f1e8';

/** A short-lived face override shown while a pet is fed, played with or loved. */
export type PetReaction = 'eat' | 'play' | 'love';

/** Blend a hex colour towards white (0 = unchanged, 1 = white). */
function pale(hex: string, t: number): string {
  const n = parseInt(hex.slice(1), 16);
  const ch = (shift: number) => Math.round(((n >> shift) & 255) + (255 - ((n >> shift) & 255)) * t);
  return `#${[16, 8, 0].map(s => ch(s).toString(16).padStart(2, '0')).join('')}`;
}

interface Pt { x: number; y: number }

/** Every colour a species drawing can ask for. */
interface Palette {
  fur: string;        // main body colour
  shade: string;      // far legs, underside, inner ears, stripes, beard
  line: string;       // outline colour
  face: string;       // head colour (sheep's dark face)
  ink: string;        // eye/mouth ink for this face
  dark: string;       // horse mane + tail, dog ears, cow tail tuft, sheep far legs
  darkLine: string;   // outline for the dark parts
  muzzle: string;     // muzzle / snout
  nose?: string;      // nostrils / nose leather (defaults to the outline colour)
  horn: string;       // cow + goat horns
  inner?: string;     // inner ear (cat, pig; defaults to shade)
  hoof?: string;      // pig trotters (other hoofed species share one dark hoof colour)
  patch?: string;     // cow: the patch over the eye (or a blaze)
  spots?: string;     // cow + pig body spots; omit for a solid coat
}

interface Geometry {
  body: { x: number; y: number; rx: number; ry: number };
  head: { x: number; y: number; r: number };
  eyes: [Pt, Pt];
  mouth: Pt;
  cheek: Pt;
  legs: { x: number; y: number; len: number }[]; // hip point + length; index 0/3 swing together
  hat: { x: number; y: number; s: number };       // crown of the head, scale
  neck: { x: number; y: number; w: number };      // where a collar sits
}

type Rig = Geometry & Palette;

const RIGS: Record<PetSpecies, Geometry> = {
  horse: {
    body: { x: 40, y: 50, rx: 25, ry: 14 }, head: { x: 74, y: 27, r: 12 },
    eyes: [{ x: 70, y: 24 }, { x: 79, y: 25 }], mouth: { x: 89, y: 39 }, cheek: { x: 76, y: 32 },
    legs: [{ x: 24, y: 58, len: 18 }, { x: 31, y: 58, len: 18 }, { x: 50, y: 58, len: 18 }, { x: 57, y: 58, len: 18 }],
    hat: { x: 73, y: 15, s: 0.95 }, neck: { x: 68, y: 40, w: 13 },
  },
  dog: {
    body: { x: 41, y: 51, rx: 25, ry: 14 }, head: { x: 74, y: 31, r: 14 },
    eyes: [{ x: 69, y: 28 }, { x: 79, y: 28 }], mouth: { x: 84, y: 40 }, cheek: { x: 72, y: 36 },
    legs: [{ x: 24, y: 58, len: 18 }, { x: 31, y: 58, len: 18 }, { x: 50, y: 58, len: 18 }, { x: 57, y: 58, len: 18 }],
    hat: { x: 74, y: 17, s: 1 }, neck: { x: 66, y: 44, w: 14 },
  },
  cat: {
    body: { x: 42, y: 52, rx: 24, ry: 13 }, head: { x: 74, y: 32, r: 13 },
    eyes: [{ x: 69, y: 30 }, { x: 79, y: 30 }], mouth: { x: 80, y: 39 }, cheek: { x: 72, y: 37 },
    legs: [{ x: 26, y: 58, len: 18 }, { x: 33, y: 58, len: 18 }, { x: 50, y: 58, len: 18 }, { x: 57, y: 58, len: 18 }],
    hat: { x: 74, y: 19, s: 0.95 }, neck: { x: 66, y: 44, w: 13 },
  },
  chicken: {
    body: { x: 44, y: 53, rx: 21, ry: 15 }, head: { x: 68, y: 29, r: 12 },
    eyes: [{ x: 64, y: 26 }, { x: 73, y: 26 }], mouth: { x: 0, y: 0 }, cheek: { x: 70, y: 33 },
    legs: [{ x: 40, y: 66, len: 10 }, { x: 50, y: 66, len: 10 }],
    hat: { x: 68, y: 17, s: 0.9 }, neck: { x: 62, y: 41, w: 11 },
  },
  cow: {
    body: { x: 41, y: 50, rx: 27, ry: 15 }, head: { x: 74, y: 30, r: 14 },
    eyes: [{ x: 69, y: 27 }, { x: 79, y: 27 }], mouth: { x: 83, y: 41 }, cheek: { x: 70, y: 34 },
    legs: [{ x: 22, y: 58, len: 18 }, { x: 29, y: 58, len: 18 }, { x: 50, y: 58, len: 18 }, { x: 57, y: 58, len: 18 }],
    hat: { x: 74, y: 16, s: 1 }, neck: { x: 66, y: 44, w: 14 },
  },
  pig: {
    body: { x: 42, y: 52, rx: 26, ry: 15 }, head: { x: 74, y: 34, r: 14 },
    eyes: [{ x: 69, y: 31 }, { x: 79, y: 31 }], mouth: { x: 82, y: 45 }, cheek: { x: 70, y: 38 },
    legs: [{ x: 24, y: 60, len: 16 }, { x: 31, y: 60, len: 16 }, { x: 50, y: 60, len: 16 }, { x: 57, y: 60, len: 16 }],
    hat: { x: 74, y: 20, s: 1 }, neck: { x: 66, y: 47, w: 14 },
  },
  sheep: {
    body: { x: 42, y: 50, rx: 25, ry: 15 }, head: { x: 75, y: 33, r: 12 },
    eyes: [{ x: 71, y: 30 }, { x: 80, y: 30 }], mouth: { x: 80, y: 40 }, cheek: { x: 72, y: 37 },
    legs: [{ x: 26, y: 58, len: 18 }, { x: 33, y: 58, len: 18 }, { x: 50, y: 58, len: 18 }, { x: 57, y: 58, len: 18 }],
    hat: { x: 74, y: 17, s: 0.95 }, neck: { x: 67, y: 45, w: 12 },
  },
  goat: {
    body: { x: 41, y: 51, rx: 25, ry: 14 }, head: { x: 74, y: 30, r: 12 },
    eyes: [{ x: 70, y: 27 }, { x: 79, y: 27 }], mouth: { x: 85, y: 40 }, cheek: { x: 72, y: 34 },
    legs: [{ x: 24, y: 58, len: 18 }, { x: 31, y: 58, len: 18 }, { x: 50, y: 58, len: 18 }, { x: 57, y: 58, len: 18 }],
    hat: { x: 73, y: 18, s: 0.9 }, neck: { x: 67, y: 43, w: 13 },
  },
};

// ---------------------------------------------------------------------------
// Coat colours. The first palette of each species is the original art.

/** Build a palette from the main colours; the rest default to sensible parts. */
function coat(p: Partial<Palette> & Pick<Palette, 'fur' | 'shade' | 'line'>): Palette {
  return {
    face: p.fur, ink: INK, dark: p.shade, darkLine: p.line, muzzle: p.fur, horn: p.shade,
    ...p,
  };
}

const COATS: Record<PetSpecies, Record<string, Palette>> = {
  horse: {
    chestnut: coat({ fur: '#c98f5e', shade: '#a97245', line: '#6b4327', dark: '#5c3a22', darkLine: '#4a2d19', muzzle: '#e4b58d' }),
    black: coat({ fur: '#44403e', shade: '#2f2b29', line: '#1a1716', ink: LIGHT_INK, dark: '#211e1d', darkLine: '#5d5753', muzzle: '#6f6761', nose: '#5d5753' }),
    grey: coat({ fur: '#cfcfd5', shade: '#aeaeb6', line: '#62626b', dark: '#f3f3f6', darkLine: '#9c9ca6', muzzle: '#8d8d95' }),
    palomino: coat({ fur: '#e6bd6e', shade: '#cc9c4a', line: '#8c6324', dark: '#fbf3dc', darkLine: '#c9b480', muzzle: '#f3dfbe' }),
  },
  dog: {
    golden: coat({ fur: '#e8b472', shade: '#cf9552', line: '#7a4f2a', dark: '#b8783f', muzzle: '#f8e2c3', nose: '#3a2a24' }),
    chocolate: coat({ fur: '#7f4d30', shade: '#633821', line: '#3b2113', ink: LIGHT_INK, dark: '#4f2c19', muzzle: '#b48968', nose: '#2b1a10' }),
    black: coat({ fur: '#403b39', shade: '#2d2927', line: '#171414', ink: LIGHT_INK, dark: '#242120', muzzle: '#c9a27c', nose: '#1a1514' }),
    white: coat({ fur: '#fbf7f0', shade: '#e7dfd2', line: '#a89d90', dark: '#dccbb3', muzzle: '#ffffff', nose: '#3a2a24' }),
  },
  cat: {
    ginger: coat({ fur: '#f0a455', shade: '#d6853a', line: '#8a4f1f', inner: '#f7c3cf', muzzle: '#fbe3c8', nose: '#e4788f' }),
    grey: coat({ fur: '#aaaab5', shade: '#85858f', line: '#4f4e58', inner: '#f7c3cf', muzzle: '#e9e8ee', nose: '#e4788f' }),
    black: coat({ fur: '#413c41', shade: '#2d292d', line: '#19171a', ink: LIGHT_INK, inner: '#8e7580', muzzle: '#746d75', nose: '#d4728a' }),
    white: coat({ fur: '#fcf9f4', shade: '#e8e1d7', line: '#aaa094', inner: '#f7c3cf', muzzle: '#ffffff', nose: '#e4788f' }),
  },
  chicken: {
    white: coat({ fur: '#fff8ea', shade: '#eadfc8', line: '#b8a27c' }),
    brown: coat({ fur: '#b9763c', shade: '#925829', line: '#5e3717' }),
    black: coat({ fur: '#3e3937', shade: '#2a2625', line: '#161313', ink: LIGHT_INK }),
    golden: coat({ fur: '#ecbb4c', shade: '#cc9630', line: '#8b6218' }),
  },
  cow: {
    holstein: coat({ fur: '#fffaf0', shade: '#eee4d2', line: '#6b5e56', dark: '#3d3532', horn: '#e8d7b1', muzzle: '#f4b6c2', nose: '#a86a78', patch: '#3d3532', spots: '#3d3532' }),
    jersey: coat({ fur: '#cb9b60', shade: '#ad7e47', line: '#6e4a23', dark: '#4a3322', horn: '#e8d7b1', muzzle: '#ead6b6', nose: '#4a3a30' }),
    black: coat({ fur: '#403a37', shade: '#2d2826', line: '#161312', ink: LIGHT_INK, dark: '#201d1b', horn: '#e8d7b1', muzzle: '#8d827b', nose: '#2a2422', patch: '#f4ede2' }),
    red: coat({ fur: '#b85d3b', shade: '#99492a', line: '#5f2a14', face: '#fdf5ea', dark: '#5f2a14', horn: '#e8d7b1', muzzle: '#f4b6c2', nose: '#a86a78', spots: '#fdf5ea' }),
  },
  pig: {
    pink: coat({ fur: '#f7b7c4', shade: '#ea98ab', line: '#b35f78', muzzle: '#f28fa6', nose: '#b0546d', hoof: '#b35f78' }),
    black: coat({ fur: '#4b4246', shade: '#363034', line: '#1f1b1d', ink: LIGHT_INK, inner: '#6d5f66', muzzle: '#857079', nose: '#2b2427', hoof: '#1f1b1d' }),
    ginger: coat({ fur: '#dc9060', shade: '#c27446', line: '#7e4522', muzzle: '#eaa37e', hoof: '#7e4522' }),
    spotted: coat({ fur: '#f7b7c4', shade: '#ea98ab', line: '#b35f78', muzzle: '#f28fa6', nose: '#b0546d', hoof: '#b35f78', spots: '#4b4246' }),
  },
  sheep: {
    white: coat({ fur: '#fbf8f1', shade: '#e6dfcf', line: '#a39a88', face: '#4d4543', ink: LIGHT_INK, dark: '#3a3331', darkLine: '#3a3331' }),
    cream: coat({ fur: '#f5e6c2', shade: '#e2cc98', line: '#a68e5a', face: '#4d4543', ink: LIGHT_INK, dark: '#3a3331', darkLine: '#3a3331' }),
    brown: coat({ fur: '#8f6c4c', shade: '#705238', line: '#443120', face: '#4d4543', ink: LIGHT_INK, dark: '#3a3331', darkLine: '#3a3331' }),
    black: coat({ fur: '#423d3b', shade: '#2e2a28', line: '#181515', face: '#a39b96', ink: INK, dark: '#3a3331', darkLine: '#3a3331' }),
  },
  goat: {
    white: coat({ fur: '#e9e0cf', shade: '#d3c6ab', line: '#7a6a55', horn: '#9b8a73', muzzle: '#f4ede0' }),
    tan: coat({ fur: '#d5ab72', shade: '#b68b4e', line: '#7b5629', horn: '#8c7961', muzzle: '#f0dbbb' }),
    brown: coat({ fur: '#825d3f', shade: '#64452c', line: '#3d2817', ink: LIGHT_INK, horn: '#6e5f4e', muzzle: '#bb966f' }),
    black: coat({ fur: '#3f3a38', shade: '#2b2726', line: '#161414', ink: LIGHT_INK, horn: '#8d847b', muzzle: '#837a74' }),
  },
};

/** Coat colour keys per species, in display order; the first is the default (the original art). */
export const COAT_COLORS: Record<PetSpecies, readonly string[]> = {
  horse: ['chestnut', 'black', 'grey', 'palomino'],
  dog: ['golden', 'chocolate', 'black', 'white'],
  cat: ['ginger', 'grey', 'black', 'white'],
  chicken: ['white', 'brown', 'black', 'golden'],
  cow: ['holstein', 'jersey', 'black', 'red'],
  pig: ['pink', 'black', 'ginger', 'spotted'],
  sheep: ['white', 'cream', 'brown', 'black'],
  goat: ['white', 'tan', 'brown', 'black'],
};

function resolveCoat(species: PetSpecies, color?: string): Palette {
  const coats = COATS[species] ?? COATS.dog;
  const keys = COAT_COLORS[species] ?? COAT_COLORS.dog;
  return (color && coats[color]) || coats[keys[0]];
}

/** Main body colour of a coat, for colour-picker swatches. */
export function coatSwatch(species: PetSpecies, color?: string): string {
  return resolveCoat(species, color).fur;
}

/** Apply the pale/sick tint to a rig's colours. */
function tint(r: Rig, t: number): Rig {
  if (t === 0) return r;
  return { ...r, fur: pale(r.fur, t), shade: pale(r.shade, t), line: pale(r.line, t * 0.6), face: pale(r.face, r.face === r.fur ? t : t * 0.4) };
}

/**
 * Outline props for a part drawn in `color` at width `w`. The attributes are
 * the soft tinted line; the inline style lets the skin's farm dials push it
 * towards the skin's own ink and thicken it (themes.css: --farm-ink,
 * --farm-ink-mix, --farm-line-k). With the dials at 0% / 1 the line is
 * exactly the soft one, so the other skins are untouched.
 */
function ln(color: string, w = 1.5) {
  return {
    stroke: color,
    strokeWidth: w,
    strokeLinejoin: 'round' as const,
    strokeLinecap: 'round' as const,
    style: {
      stroke: `color-mix(in srgb, var(--farm-ink, #000) var(--farm-ink-mix, 0%), ${color})`,
      strokeWidth: `calc(${w}px * var(--farm-line-k, 1))`,
    },
  };
}

// ---------------------------------------------------------------------------
// Face

function Eye({ p, mood, fur, line, ink }: { p: Pt; mood: PetMood; fur: string; line: string; ink: string }) {
  const r = 3.8;
  const { x, y } = p;
  if (mood === 'happy') {
    // Joyful squint: a thick upward arch.
    return <path d={`M${x - 3.6} ${y + 1.5} q3.6 -5 7.2 0`} stroke={ink} strokeWidth={2.2} fill="none" strokeLinecap="round" />;
  }
  if (mood === 'very_sick') {
    return <path d={`M${x - 3} ${y - 3} l6 6 M${x + 3} ${y - 3} l-6 6`} stroke={ink} strokeWidth={2} fill="none" strokeLinecap="round" />;
  }
  const hungry = mood === 'hungry';
  const px = x + (hungry ? 1.1 : 0.6);
  const py = y + (hungry ? 1 : 0.4);
  // Eyelid: a segment of the eye circle filled with fur, sitting on a chord.
  const lid = (kl: number, kr: number) => {
    const dl = r * Math.sqrt(1 - kl * kl);
    const dr = r * Math.sqrt(1 - kr * kr);
    return `M${x - dl} ${y + kl * r} A${r} ${r} 0 0 1 ${x + dr} ${y + kr * r} Z`;
  };
  return (
    <g>
      <circle cx={x} cy={y} r={r} fill="#fff" stroke={line} strokeWidth={1} />
      <circle cx={px} cy={py} r={hungry ? 2.4 : 2.1} fill={INK} />
      <circle cx={px + 0.8} cy={py - 0.9} r={0.75} fill="#fff" />
      {mood === 'bored' && (
        <g>
          <path d={lid(0.15, 0.15)} fill={fur} stroke={line} strokeWidth={1} />
          <path d={`M${x - 3.6} ${y + 0.6} h7.2`} stroke={ink} strokeWidth={1.4} strokeLinecap="round" />
        </g>
      )}
      {mood === 'sick' && (
        <g>
          <path d={lid(-0.3, 0.35)} fill={fur} stroke={line} strokeWidth={1} />
          <path d={`M${x - 3.6} ${y - 1.1} L${x + 3.5} ${y + 1.4}`} stroke={ink} strokeWidth={1.4} strokeLinecap="round" />
        </g>
      )}
    </g>
  );
}

function Mouth({ p, mood, ink }: { p: Pt; mood: PetMood; ink: string }) {
  const { x, y } = p;
  const s = { stroke: ink, strokeWidth: 1.8, strokeLinecap: 'round' as const, fill: 'none' };
  switch (mood) {
    case 'happy':
      return (
        <g>
          <path d={`M${x - 4.5} ${y - 1} q4.5 7 9 0z`} fill={INK} />
          <path d={`M${x - 2} ${y + 1.6} q2 2.4 4 0 q-2 -1.6 -4 0z`} fill="#ef8aa0" />
        </g>
      );
    case 'content':
      return <path d={`M${x - 3.5} ${y} q3.5 3.5 7 0`} {...s} />;
    case 'hungry':
      return (
        <g>
          <ellipse cx={x} cy={y + 1} rx={2.8} ry={3.3} fill={INK} />
          <ellipse cx={x} cy={y + 2.6} rx={1.5} ry={1.3} fill="#ef8aa0" />
          <path d={`M${x + 3.2} ${y + 2} q1.6 2.5 0 4 q-1.6 -1.5 0 -4z`} fill="#8fc4f4" />
        </g>
      );
    case 'bored':
      return <path d={`M${x - 3} ${y + 0.5} h6`} {...s} />;
    default:
      return <path d={`M${x - 4} ${y + 0.5} q2 -2.4 4 0 t4 0`} {...s} />;
  }
}

/** Eyes for a reaction: squeezed shut (eat), wide and sparkling (play), hearts (love). */
function ReactionEye({ p, reaction, line, ink }: { p: Pt; reaction: PetReaction; line: string; ink: string }) {
  const { x, y } = p;
  switch (reaction) {
    case 'eat':
      return <path d={`M${x - 3.6} ${y + 1.5} q3.6 -5 7.2 0`} stroke={ink} strokeWidth={2.2} fill="none" strokeLinecap="round" />;
    case 'play':
      return (
        <g>
          <circle cx={x} cy={y} r={4.4} fill="#fff" stroke={line} strokeWidth={1} />
          <circle cx={x + 0.5} cy={y + 0.3} r={2.9} fill={INK} />
          <circle cx={x + 1.5} cy={y - 1} r={1.1} fill="#fff" />
          <circle cx={x - 0.8} cy={y + 1.4} r={0.55} fill="#fff" />
        </g>
      );
    case 'love':
      return (
        <path transform={`translate(${x} ${y + 0.6})`} d="M0 4.4 L-4.8 -0.6 A2.9 2.9 0 0 1 0 -3.6 A2.9 2.9 0 0 1 4.8 -0.6z"
          fill="#ff5c8a" stroke="#c23466" strokeWidth={1} strokeLinejoin="round" />
      );
  }
}

/** Mouths for a reaction. The chicken has no mouth anchor, so it gets a lower beak instead. */
function ReactionMouth({ r, species, reaction }: { r: Rig; species: PetSpecies; reaction: PetReaction }) {
  if (species === 'chicken') {
    const { x, y } = r.head;
    // Lower beak hinged at the top so the chew animation opens and closes it.
    return <path className={reaction === 'eat' ? 'farm-chew' : undefined} d={`M${x + 9} ${y + 2} l8.5 1 l-7.5 4.5z`} fill="#d98f2b" stroke="#c98427" strokeWidth={1} strokeLinejoin="round" />;
  }
  const { x, y } = r.mouth;
  switch (reaction) {
    case 'eat':
      return (
        <g>
          <path d={`M${x - 3.5} ${y - 1} h7`} stroke={r.ink} strokeWidth={1.6} strokeLinecap="round" />
          <g className="farm-chew">
            <path d={`M${x - 3.5} ${y - 1} q3.5 7 7 0z`} fill={INK} />
            <path d={`M${x - 1.6} ${y + 0.6} q1.6 2.2 3.2 0 q-1.6 -1.4 -3.2 0z`} fill="#ef8aa0" />
          </g>
        </g>
      );
    case 'play':
      return (
        <g>
          <path d={`M${x - 5} ${y - 1.5} q5 9 10 0z`} fill={INK} />
          <path d={`M${x - 2.4} ${y + 2} q2.4 3 4.8 0 q-2.4 -2 -4.8 0z`} fill="#ef8aa0" />
        </g>
      );
    case 'love':
      return <path d={`M${x - 3.5} ${y} q3.5 3.5 7 0`} stroke={r.ink} strokeWidth={1.8} strokeLinecap="round" fill="none" />;
  }
}

function Face({ r, mood, species, reaction }: { r: Rig; mood: PetMood; species: PetSpecies; reaction?: PetReaction }) {
  if (reaction) {
    const { x: cx, y: cy } = r.cheek;
    return (
      <g>
        {reaction === 'eat' && (
          <g className="farm-puff">
            <circle cx={cx - 1} cy={cy + 1.5} r={4.2} fill={r.face} stroke={r.line} strokeWidth={1} opacity={0.9} />
          </g>
        )}
        {reaction === 'love' && <circle cx={cx} cy={cy} r={3.4} fill="#f27d95" opacity={0.55} />}
        {reaction === 'love' && (
          // The animated class lives on the inner element so CSS transforms don't replace the placement.
          <g transform={`translate(${r.head.x + r.head.r + 1} ${r.head.y - r.head.r - 2}) rotate(12)`}>
            <path className="farm-float" d="M0 4.2 L-4.4 -0.5 A2.6 2.6 0 0 1 0 -3.4 A2.6 2.6 0 0 1 4.4 -0.5z" fill="#ff5c8a" stroke="#c23466" strokeWidth={0.9} strokeLinejoin="round" />
          </g>
        )}
        {reaction === 'play' && <circle cx={cx} cy={cy} r={2.8} fill="#f27d95" opacity={0.35} />}
        {r.eyes.map((p, i) => <ReactionEye key={i} p={p} reaction={reaction} line={r.line} ink={r.ink} />)}
        <ReactionMouth r={r} species={species} reaction={reaction} />
      </g>
    );
  }
  const sickly = mood === 'sick' || mood === 'very_sick';
  return (
    <g>
      {mood === 'happy' && <circle cx={r.cheek.x} cy={r.cheek.y} r={2.8} fill="#f27d95" opacity={0.45} />}
      {sickly && <circle cx={r.cheek.x} cy={r.cheek.y} r={3.2} fill="#8fcf7a" opacity={0.8} />}
      {r.eyes.map((p, i) => <Eye key={i} p={p} mood={mood} fur={r.face} line={r.line} ink={r.ink} />)}
      {r.mouth.x > 0 && <Mouth p={r.mouth} mood={mood} ink={r.ink} />}
    </g>
  );
}

// ---------------------------------------------------------------------------
// Species features, split into what goes behind the body/head and in front.

function Tail({ species, r, fast }: { species: PetSpecies; r: Rig; fast?: boolean }) {
  const { x, y, rx } = r.body;
  const bx = x - rx + 3;
  const wag = fast ? 'farm-tail farm-tail-fast' : 'farm-tail';
  switch (species) {
    case 'horse':
      return <path d={`M${bx + 2} ${y - 8} q-12 6 -11 26 q2 -4 5 -6 q1 4 4 6 q3 -14 6 -22z`} fill={r.dark} {...ln(r.darkLine)} />;
    case 'dog':
      return <path className={wag} d={`M${bx + 3} ${y - 5} q-9 -4 -12 -16`} stroke={r.shade} strokeWidth={5} fill="none" strokeLinecap="round" />;
    case 'cat':
      return (
        <g className={wag}>
          {/* Outline stroke: 4.5 of fur plus the outline on each side, so it thickens with --farm-line-k. */}
          <path d={`M${bx + 2} ${y + 2} q-13 -2 -11 -17 q1 -7 7 -5`} {...ln(r.line, 7)} style={{ ...ln(r.line).style, strokeWidth: 'calc(4.5px + 2.5px * var(--farm-line-k, 1))' }} fill="none" />
          <path d={`M${bx + 2} ${y + 2} q-13 -2 -11 -17 q1 -7 7 -5`} stroke={r.fur} strokeWidth={4.5} fill="none" strokeLinecap="round" />
        </g>
      );
    case 'chicken':
      return (
        <path d={`M${bx + 6} ${y - 4} q-8 -6 -8 -20 q5 5 8 10 q-2 -9 3 -16 q3 8 4 14 q1 -7 6 -11 q0 10 -2 20z`}
          fill={r.fur} {...ln(r.line)} />
      );
    case 'cow':
      return (
        <g>
          <path d={`M${bx + 2} ${y - 7} q-9 6 -8 24`} {...ln(r.line, 2.2)} fill="none" />
          <path d={`M${bx - 6} ${y + 15} q-3 3 0 7 q3 -3 3 -7z`} fill={r.dark} />
        </g>
      );
    case 'pig':
      return <path d={`M${bx + 2} ${y - 4} q-7 -5 -4 -9 q4 -2 3 2 q-1 3 -5 2`} {...ln(r.line, 2)} fill="none" />;
    case 'sheep':
      return <circle cx={bx - 1} cy={y - 6} r={5} fill={r.fur} {...ln(r.line)} />;
    case 'goat':
      return <path d={`M${bx + 3} ${y - 6} l-7 -8 q5 1 9 5z`} fill={r.fur} {...ln(r.line)} />;
  }
}

/** Markings and parts drawn over the body but under the head. */
function BodyDetail({ species, r }: { species: PetSpecies; r: Rig }) {
  const { x, y, rx, ry } = r.body;
  switch (species) {
    case 'horse':
      return (
        <g>
          {/* neck with mane */}
          <path d={`M${x + 12} ${y - 10} Q${x + 20} ${y - 26} ${r.head.x - 2} ${r.head.y - 6} L${r.head.x + 6} ${r.head.y + 8} Q${x + 24} ${y} ${x + 18} ${y + 6}z`} fill={r.fur} {...ln(r.line)} />
          <path d={`M${r.head.x - 3} ${r.head.y - r.head.r - 1} q-9 2 -11 9 q-7 2 -8 9 q-6 3 -6 9 q-3 2 -2 6 L${x + 11} ${y - 9} Q${x + 20} ${y - 26} ${r.head.x - 1} ${r.head.y - 5}z`} fill={r.dark} {...ln(r.darkLine)} />
        </g>
      );
    case 'dog':
      return <ellipse cx={x - 6} cy={y - 4} rx={9} ry={6} fill={r.shade} opacity={0.6} />;
    case 'cat':
      return (
        <path d={`M${x - 10} ${y - 11} q2 6 0 12 M${x} ${y - 13} q2 7 0 13 M${x + 10} ${y - 11} q2 6 0 11`}
          stroke={r.shade} strokeWidth={2.6} fill="none" strokeLinecap="round" />
      );
    case 'chicken':
      return (
        <g>
          <path d={`M${x + 10} ${y - 14} L${r.head.x - 6} ${r.head.y - 2} L${r.head.x + 6} ${r.head.y + 6} L${x + 16} ${y - 2}z`} fill={r.fur} />
          <path d={`M${x - 6} ${y - 3} q10 -8 20 0 q-8 11 -20 0z`} fill={r.shade} {...ln(r.line)} />
        </g>
      );
    case 'cow':
      return (
        <g>
          {r.spots && <path d={`M${x - 10} ${y - 10} q10 -4 14 4 q-2 10 -12 7 q-5 -5 -2 -11z`} fill={r.spots} />}
          {r.spots && <path d={`M${x + 12} ${y + 2} q9 -3 11 5 q-7 6 -13 1z`} fill={r.spots} />}
          {r.spots && <path d={`M${x - 22} ${y + 4} q5 -3 7 4 q-5 4 -8 0z`} fill={r.spots} />}
          <ellipse cx={x - 8} cy={y + ry - 1} rx={6} ry={3.5} fill="#f4b6c2" />
        </g>
      );
    case 'pig': {
      const belly = <ellipse cx={x} cy={y + 6} rx={14} ry={6} fill={r.shade} opacity={0.45} />;
      if (!r.spots) return belly;
      return (
        <g>
          {belly}
          <g fill={r.spots}>
            <path d={`M${x - 12} ${y - 9} q9 -5 13 3 q-3 9 -12 6 q-5 -4 -1 -9z`} />
            <path d={`M${x + 9} ${y + 1} q8 -3 10 4 q-6 6 -12 2z`} />
            <path d={`M${x - 21} ${y + 6} q5 -4 8 2 q-4 5 -8 1z`} />
          </g>
        </g>
      );
    }
    case 'sheep':
      return (
        <g>
          {[[-18, -6], [-8, -12], [4, -13], [15, -9], [21, 0], [14, 9], [2, 12], [-11, 10], [-20, 5]].map(([dx, dy], i) => (
            <circle key={i} cx={x + dx} cy={y + dy} r={8.5} fill={r.fur} {...ln(r.line)} />
          ))}
          <ellipse cx={x} cy={y} rx={rx - 3} ry={ry - 3} fill={r.fur} />
        </g>
      );
    case 'goat':
      return <ellipse cx={x - 4} cy={y + 6} rx={14} ry={5} fill={r.shade} opacity={0.4} />;
  }
}

/** Ears, horns, combs: drawn before the head so they tuck behind hats. */
function HeadBack({ species, r }: { species: PetSpecies; r: Rig }) {
  const { x, y, r: hr } = r.head;
  switch (species) {
    case 'horse':
      return (
        <g>
          <path d={`M${x - 7} ${y - hr + 3} l-1 -9 l6 6z M${x + 2} ${y - hr + 1} l3 -8 l4 8z`} fill={r.fur} {...ln(r.line)} />
          <path d={`M${x - 4} ${y - hr + 1} q-4 4 -6 10 q4 -2 6 -5 q2 3 6 4 q-1 -6 -6 -9z`} fill={r.dark} {...ln(r.darkLine)} />
        </g>
      );
    case 'dog':
      return <path d={`M${x + 2} ${y - hr + 2} q8 -3 10 4 q-1 4 -4 5 q-2 -6 -6 -9z`} fill={r.dark} {...ln(r.line)} />;
    case 'cat':
      return (
        <g>
          <path d={`M${x - 10} ${y - 6} l-1 -13 l11 7z M${x + 3} ${y - 12} l9 -8 l1 13z`} fill={r.fur} {...ln(r.line)} />
          <path d={`M${x - 8} ${y - 8} l0 -7 l6 4z M${x + 5} ${y - 12} l5 -4 l0 7z`} fill={r.inner ?? r.shade} />
        </g>
      );
    case 'chicken':
      return (
        <path d={`M${x - 8} ${y - hr + 3} q0 -8 5 -6 q1 -7 6 -3 q3 -6 6 1 q2 -3 4 2 q-6 2 -21 6z`} fill="#e5473f" {...ln('#b7342d')} />
      );
    case 'cow':
      return (
        <g>
          <path d={`M${x - 7} ${y - hr + 3} q-3 -9 -9 -9`} stroke={r.horn} strokeWidth={4} fill="none" strokeLinecap="round" />
          <path d={`M${x + 7} ${y - hr + 3} q3 -9 9 -9`} stroke={r.horn} strokeWidth={4} fill="none" strokeLinecap="round" />
          <ellipse cx={x - hr - 2} cy={y - 2} rx={6} ry={3.5} fill={r.fur} {...ln(r.line)} />
        </g>
      );
    case 'pig':
      return (
        <g>
          <path d={`M${x - 10} ${y - 7} l-3 -11 l11 5z M${x + 4} ${y - 12} l8 -8 l2 11z`} fill={r.fur} {...ln(r.line)} />
          <path d={`M${x - 8} ${y - 9} l-1 -5 l5 3z M${x + 6} ${y - 12} l4 -4 l1 6z`} fill={r.inner ?? r.shade} />
        </g>
      );
    case 'sheep':
      return (
        <g>
          {[[-6, -9], [2, -11], [8, -8]].map(([dx, dy], i) => (
            <circle key={i} cx={x + dx} cy={y + dy} r={5.5} fill={r.fur} {...ln(r.line)} />
          ))}
          <ellipse cx={x - hr} cy={y + 1} rx={5} ry={3} fill={r.face} {...ln(r.darkLine)} />
        </g>
      );
    case 'goat':
      return (
        <g>
          <path d={`M${x - 5} ${y - hr + 2} q-4 -10 -12 -12`} stroke={r.horn} strokeWidth={3.5} fill="none" strokeLinecap="round" />
          <path d={`M${x + 2} ${y - hr + 1} q-1 -11 -8 -14`} stroke={r.horn} strokeWidth={3.5} fill="none" strokeLinecap="round" />
          <path d={`M${x - hr + 2} ${y - 2} q-9 -1 -9 6 q6 2 10 -2z`} fill={r.fur} {...ln(r.line)} />
        </g>
      );
  }
}

/** Muzzles, noses, whiskers: drawn on top of the head, under the face. */
function HeadFront({ species, r }: { species: PetSpecies; r: Rig }) {
  const { x, y, r: hr } = r.head;
  switch (species) {
    case 'horse':
      return (
        <g>
          <ellipse cx={x + 13} cy={y + 9} rx={10} ry={7.5} fill={r.muzzle} {...ln(r.line)} />
          <ellipse cx={x + 18} cy={y + 7} rx={1.4} ry={1.1} fill={r.nose ?? r.line} />
        </g>
      );
    case 'dog':
      return (
        <g>
          <ellipse cx={x + 9} cy={y + 8} rx={8.5} ry={6.5} fill={r.muzzle} {...ln(r.line)} />
          <ellipse cx={x + 14} cy={y + 5} rx={3} ry={2.4} fill={r.nose ?? r.line} />
          <path d={`M${x - 4} ${y - hr + 2} q-9 1 -12 10 q-2 9 4 13 q5 -3 5 -12 q0 -7 3 -11z`} fill={r.dark} {...ln(r.line)} />
        </g>
      );
    case 'cat':
      return (
        <g>
          <ellipse cx={x + 7} cy={y + 7} rx={6} ry={4.2} fill={r.muzzle} />
          <path d={`M${x + 5.5} ${y + 3.5} l3 0 l-1.5 2.2z`} fill={r.nose ?? r.line} />
          <path d={`M${x + 11} ${y + 4.5} l10 -2.5 M${x + 11} ${y + 7} l10 1.5 M${x + 3} ${y + 6} l-8 -1 M${x + 3} ${y + 8} l-8 1.5`}
            stroke={r.line} strokeWidth={0.9} strokeLinecap="round" />
          <path d={`M${x - 2} ${y - hr + 1} q2 4 0 7 M${x + 4} ${y - hr + 2} q2 4 0 7`} stroke={r.shade} strokeWidth={2} fill="none" strokeLinecap="round" />
        </g>
      );
    case 'chicken':
      return (
        <g>
          <path d={`M${x + 9} ${y - 1} l9 3 l-9 3z`} fill="#f2a93b" {...ln('#c98427')} />
          <path d={`M${x + 9} ${y + 5} q3 6 -1 8 q-3 -4 1 -8z`} fill="#e5473f" {...ln('#b7342d')} />
        </g>
      );
    case 'cow':
      return (
        <g>
          {r.patch && <path d={`M${x - 11} ${y - 9} q8 -4 12 3 q-1 8 -9 7 q-5 -4 -3 -10z`} fill={r.patch} />}
          <ellipse cx={x + 9} cy={y + 10} rx={10} ry={6.5} fill={r.muzzle} {...ln(r.line)} />
          <ellipse cx={x + 6} cy={y + 9.5} rx={1.4} ry={1.1} fill={r.nose ?? r.line} />
          <ellipse cx={x + 13} cy={y + 9.5} rx={1.4} ry={1.1} fill={r.nose ?? r.line} />
        </g>
      );
    case 'pig':
      return (
        <g>
          <ellipse cx={x + 11} cy={y + 6} rx={6.5} ry={5.5} fill={r.muzzle} {...ln(r.line)} />
          <ellipse cx={x + 9} cy={y + 6} rx={1.3} ry={1.7} fill={r.nose ?? r.line} />
          <ellipse cx={x + 13.5} cy={y + 6} rx={1.3} ry={1.7} fill={r.nose ?? r.line} />
        </g>
      );
    case 'sheep':
      return <ellipse cx={x + hr - 1} cy={y + 1} rx={5} ry={3} fill={r.face} {...ln(r.darkLine)} />;
    case 'goat':
      return (
        <g>
          <ellipse cx={x + 11} cy={y + 8} rx={8} ry={6} fill={r.muzzle} {...ln(r.line)} />
          <path d={`M${x + 7} ${y + 13} q2 8 4 10 q2 -4 3 -10z`} fill={r.shade} {...ln(r.line)} />
          <ellipse cx={x + 16} cy={y + 6} rx={1.3} ry={1} fill={r.nose ?? r.line} />
        </g>
      );
  }
}

// ---------------------------------------------------------------------------
// Accessories

/** Built-in accessory keys: hats, then face, then neck. */
export const BUILTIN_ITEMS: readonly string[] = [
  'straw_hat', 'cowboy_hat', 'top_hat', 'party_hat', 'beanie', 'wizard_hat', 'pirate_hat', 'crown', 'flower_crown', 'flower', 'bow',
  'sunglasses', 'round_glasses', 'heart_glasses',
  'bandana', 'scarf', 'bowtie', 'bell', 'medal',
];

export type AccessorySlot = 'head' | 'face' | 'neck';

export type ParsedAccessory =
  | { kind: 'builtin'; item: string }
  | { kind: 'emoji'; slot: AccessorySlot; emoji: string }
  | { kind: 'image'; slot: AccessorySlot; id: string; url: string };

const EMOJI_RE = /^e:(head|face|neck):(.+)$/su;
const IMAGE_RE = /^img:(head|face|neck):([a-f0-9]{16,64}|preview)$/;

/**
 * A picture address is only ever one of ours: an upload under /uploads/pets,
 * or a PNG data URL made in this page (a drawing or a chosen file being
 * previewed). Anything else is dropped rather than put in the page.
 */
const SAFE_IMAGE_RE = /^(?:\/uploads\/pets\/[a-f0-9]{16,64}\.png|data:image\/png;base64,[A-Za-z0-9+/=]+)$/;
export function safeImageHref(url: string | undefined): string | undefined {
  return url && SAFE_IMAGE_RE.test(url) ? url : undefined;
}

/** Where an uploaded accessory image is served from. */
export function accessoryImageUrl(id: string): string {
  return `/uploads/pets/${id}.png`;
}

/**
 * Decode an accessory string: a built-in key, `e:<slot>:<emoji>` or
 * `img:<slot>:<id>` (a hex upload id, or `preview` for a live drawing passed
 * through FarmAnimal's accessoryImageUrl). Anything malformed or unknown is
 * null and renders nothing.
 */
export function parseAccessory(a?: string): ParsedAccessory | null {
  if (!a) return null;
  if (BUILTIN_ITEMS.includes(a)) return { kind: 'builtin', item: a };
  const img = IMAGE_RE.exec(a);
  if (img) return { kind: 'image', slot: img[1] as AccessorySlot, id: img[2], url: accessoryImageUrl(img[2]) };
  const em = EMOJI_RE.exec(a);
  if (em) {
    const emoji = em[2].trim();
    // One emoji (possibly a ZWJ/flag/skin-tone sequence) or a very short label.
    if (emoji.length === 0 || emoji.length > 16) return null;
    return { kind: 'emoji', slot: em[1] as AccessorySlot, emoji };
  }
  return null;
}

/** A little four-point sparkle, centred on the origin. */
function Star({ x, y, r, fill }: { x: number; y: number; r: number; fill: string }) {
  const k = r * 0.3;
  return <path d={`M${x} ${y - r} L${x + k} ${y - k} L${x + r} ${y} L${x + k} ${y + k} L${x} ${y + r} L${x - k} ${y + k} L${x - r} ${y} L${x - k} ${y - k}z`} fill={fill} />;
}

/** A five-petal flower, centred on the origin. */
function Flower({ x, y, petal, heart, r }: { x: number; y: number; petal: string; heart: string; r: number }) {
  return (
    <g>
      {[0, 72, 144, 216, 288].map(a => (
        <circle key={a} cx={x + r * 1.4 * Math.cos(((a - 90) * Math.PI) / 180)} cy={y + r * 1.4 * Math.sin(((a - 90) * Math.PI) / 180)} r={r} fill={petal} />
      ))}
      <circle cx={x} cy={y} r={r * 0.85} fill={heart} />
    </g>
  );
}

/** Centre + size of each accessory slot, for emoji and image accessories. */
function slotBox(slot: AccessorySlot, r: Rig): { x: number; y: number; size: number } {
  switch (slot) {
    case 'head': {
      const size = r.head.r * 1.9 * r.hat.s;
      // Sit on the crown: the glyph's bottom edge just below the hat anchor.
      return { x: r.hat.x, y: r.hat.y + 3 - size * 0.45, size };
    }
    case 'face': {
      const [a, b] = r.eyes;
      return { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2, size: r.head.r * 1.7 };
    }
    case 'neck':
      return { x: r.neck.x, y: r.neck.y + 6, size: r.head.r * 1.2 };
  }
}

function EmojiAccessory({ slot, emoji, r }: { slot: AccessorySlot; emoji: string; r: Rig }) {
  const { x, y, size } = slotBox(slot, r);
  return (
    <text x={x} y={y} fontSize={size} textAnchor="middle" dominantBaseline="central" pointerEvents="none"
      fontFamily="'Noto Color Emoji', 'Apple Color Emoji', 'Segoe UI Emoji', 'Twemoji Mozilla', sans-serif">
      {emoji}
    </text>
  );
}

function ImageAccessory({ slot, url, r }: { slot: AccessorySlot; url: string; r: Rig }) {
  const { x, y, size } = slotBox(slot, r);
  // Drawings are trimmed to their own edges, so anchor by slot: a hat sits on
  // the crown, glasses centre on the eyes, neckwear hangs from the collar.
  const align = slot === 'head' ? 'xMidYMax' : slot === 'neck' ? 'xMidYMin' : 'xMidYMid';
  const href = safeImageHref(url);
  if (!href) return null;
  return <image href={href} x={x - size / 2} y={y - size / 2} width={size} height={size} preserveAspectRatio={`${align} meet`} pointerEvents="none" />;
}

function Builtin({ item, r }: { item: string; r: Rig }) {
  const { x: hx, y: hy, s } = r.hat;
  const { x: nx, y: ny, w: nw } = r.neck;
  const [e0, e1] = r.eyes;
  const strap = `M${nx - nw} ${ny - 2} q${nw} 7 ${nw * 2} 0`;
  switch (item) {
    case 'straw_hat':
      return (
        <g transform={`translate(${hx} ${hy}) scale(${s})`}>
          <ellipse cx={0} cy={1} rx={17} ry={4.5} fill="#f0cf74" {...ln('#b08a3a')} />
          <path d="M-8 1 q-1 -11 8 -11 q9 0 8 11z" fill="#f6dd8c" {...ln('#b08a3a')} />
          <path d="M-8 -1 q8 2 16 0" stroke="#e2453c" strokeWidth={2.4} fill="none" />
        </g>
      );
    case 'cowboy_hat':
      return (
        <g transform={`translate(${hx} ${hy}) scale(${s})`}>
          <path d="M-18 -1 q2 7 18 6 q16 1 18 -6 q-6 5 -18 3 q-12 2 -18 -3z" fill="#9a6336" {...ln('#5e3a1e')} />
          <path d="M-9 2 q-1 -13 4 -13 q3 3 6 0 q6 -1 6 13z" fill="#b6773f" {...ln('#5e3a1e')} />
          <path d="M-9 1 h18" stroke="#5e3a1e" strokeWidth={2} />
        </g>
      );
    case 'top_hat':
      return (
        <g transform={`translate(${hx} ${hy}) scale(${s})`}>
          <ellipse cx={0} cy={2} rx={15} ry={3.6} fill="#2e2c38" {...ln('#15141b')} />
          <path d="M-9 2 l-0.5 -14 q9.5 -2.5 19 0 l-0.5 14z" fill="#3b3947" {...ln('#15141b')} />
          <path d="M-9.3 -2 q9.3 2.6 18.6 0 v4.2 q-9.3 2.4 -18.6 0z" fill="#c23b4a" stroke="#15141b" strokeWidth={1} />
          <path d="M-6 -10 v8" stroke="#fff" strokeWidth={1.4} strokeLinecap="round" opacity={0.18} />
        </g>
      );
    case 'party_hat':
      return (
        <g transform={`translate(${hx + 2 * s} ${hy + 1}) scale(${s}) rotate(10)`}>
          <path d="M-8 2 L0 -15 L8 2z" fill="#ff7ab3" {...ln('#b03a6e')} />
          <path d="M-5.6 -3 L5.6 -3 M-3 -8.5 L3 -8.5" stroke="#8fc4f4" strokeWidth={2.2} strokeLinecap="round" />
          <path d="M-8 2 q8 -3 16 0" stroke="#ffe66d" strokeWidth={2.2} fill="none" strokeLinecap="round" />
          <circle cx={0} cy={-15.5} r={3} fill="#ffe66d" {...ln('#b08a3a')} />
        </g>
      );
    case 'beanie':
      return (
        <g transform={`translate(${hx} ${hy}) scale(${s})`}>
          <path d="M-13 3 q0 -15 13 -15 q13 0 13 15z" fill="#7a5cc7" {...ln('#4b3680')} />
          <path d="M-5 -9 q-1 6 -2 12 M0 -11 v14 M5 -9 q1 6 2 12" stroke="#fff" strokeWidth={1.4} strokeLinecap="round" opacity={0.22} />
          <path d="M-13 3 q13 -3.5 26 0 v4 q-13 3 -26 0z" fill="#9b7fe0" {...ln('#4b3680')} />
          <path d="M-9 1.5 v4 M-4.5 0.8 v4 M0 0.5 v4 M4.5 0.8 v4 M9 1.5 v4" stroke="#4b3680" strokeWidth={1} strokeLinecap="round" opacity={0.5} />
          <circle cx={0} cy={-13} r={3.6} fill="#f3ecff" {...ln('#4b3680')} />
        </g>
      );
    case 'wizard_hat':
      return (
        <g transform={`translate(${hx} ${hy}) scale(${s})`}>
          <ellipse cx={0} cy={2} rx={16} ry={4} fill="#3b48c2" {...ln('#26307f')} />
          <path d="M-9 2 Q-5 -6 -3 -13 Q-1 -17 8 -18 Q2 -15 3 -11 Q6 -4 9 2z" fill="#4a5be0" {...ln('#26307f')} />
          <path d="M-9 2 q9 -3 18 0" stroke="#ffd34e" strokeWidth={2} fill="none" strokeLinecap="round" />
          <Star x={-1} y={-6} r={2.4} fill="#ffe66d" />
          <Star x={3.5} y={-1.5} r={1.6} fill="#ffe66d" />
          <Star x={-5} y={-1} r={1.3} fill="#ffe66d" />
        </g>
      );
    case 'pirate_hat':
      return (
        <g transform={`translate(${hx} ${hy}) scale(${s})`}>
          <path d="M-19 3 q-1 -11 10 -13 q9 -3 18 0 q11 2 10 13 q-9 -5 -19 -4 q-10 -1 -19 4z" fill="#2e2c38" {...ln('#15141b')} />
          <path d="M-18 2.5 q9 -4.5 18 -3.5 q9 -1 18 3.5" stroke="#f5c542" strokeWidth={1.4} fill="none" strokeLinecap="round" />
          <circle cx={0} cy={-5.5} r={3} fill="#fff8ea" />
          <path d="M-0.6 -3 h1.2 v1.6 h-1.2z" fill="#fff8ea" />
          <circle cx={-1.1} cy={-6} r={0.8} fill="#2e2c38" />
          <circle cx={1.1} cy={-6} r={0.8} fill="#2e2c38" />
          <path d="M-5 -1.5 l10 -5.5 M-5 -7 l10 5.5" stroke="#fff8ea" strokeWidth={1.3} strokeLinecap="round" />
        </g>
      );
    case 'crown':
      return (
        <g transform={`translate(${hx} ${hy}) scale(${s})`}>
          <path d="M-11 4 L-12.5 -8 L-6 -2.5 L0 -11 L6 -2.5 L12.5 -8 L11 4z" fill="#f5c542" {...ln('#a9781c')} />
          <path d="M-11 0.5 h22 v3.5 h-22z" fill="#ffd966" stroke="#a9781c" strokeWidth={1} />
          <circle cx={-12.5} cy={-8} r={1.7} fill="#ef4d6a" stroke="#a9781c" strokeWidth={0.7} />
          <circle cx={0} cy={-11} r={1.9} fill="#5fa8f5" stroke="#a9781c" strokeWidth={0.7} />
          <circle cx={12.5} cy={-8} r={1.7} fill="#ef4d6a" stroke="#a9781c" strokeWidth={0.7} />
          <circle cx={0} cy={2.2} r={1.6} fill="#6fcf6f" stroke="#a9781c" strokeWidth={0.7} />
        </g>
      );
    case 'flower_crown':
      return (
        <g transform={`translate(${hx} ${hy}) scale(${s})`}>
          <path d="M-15 5 q15 -9 30 0" stroke="#5f9e4a" strokeWidth={2.6} fill="none" strokeLinecap="round" />
          <path d="M-8 0 q-5 -5 -1 -8 q4 3 1 8z M9 0 q5 -5 1 -8 q-4 3 -1 8z" fill="#7bb562" stroke="#4d8a3a" strokeWidth={0.8} />
          {[[-13, 3, '#f58bb4'], [-4, -1.5, '#ffd34e'], [5, -1.5, '#8fc4f4'], [14, 3, '#f58bb4']].map(([dx, dy, c], i) => (
            <g key={i}>
              {[0, 72, 144, 216, 288].map(a => (
                <circle key={a} cx={Number(dx) + 2.8 * Math.cos((a * Math.PI) / 180)} cy={Number(dy) + 2.8 * Math.sin((a * Math.PI) / 180)} r={2} fill={String(c)} />
              ))}
              <circle cx={Number(dx)} cy={Number(dy)} r={1.7} fill="#fff6d6" />
            </g>
          ))}
        </g>
      );
    case 'flower':
      return (
        <g transform={`translate(${hx - 7 * s} ${hy + 4}) scale(${s})`}>
          <path d="M2 2 q6 -1 8 4 q-6 2 -8 -4z" fill="#7bb562" stroke="#4d8a3a" strokeWidth={0.8} />
          <Flower x={0} y={0} r={2.7} petal="#ff8fb8" heart="#ffe66d" />
        </g>
      );
    case 'bow':
      return (
        <g transform={`translate(${hx + 7 * s} ${hy + 1}) scale(${s}) rotate(-15)`}>
          <path d="M-1 0 q-7 -8 -10 -3 q0 7 10 3z M1 0 q7 -8 10 -3 q0 7 -10 3z" fill="#f05a7a" {...ln('#a83350')} />
          <path d="M-1 1 q-6 7 -8 4 M1 1 q6 7 8 4" stroke="#a83350" strokeWidth={1.2} fill="none" />
          <circle cx={0} cy={0} r={2.6} fill="#f05a7a" {...ln('#a83350')} />
        </g>
      );
    case 'sunglasses':
      return (
        <g>
          <path d={`M${e0.x - 5} ${e0.y - 1} H${e1.x + 5}`} stroke={INK} strokeWidth={1.5} />
          <path d={`M${e0.x - 5} ${e0.y - 1} l-6 1`} stroke={INK} strokeWidth={1.5} strokeLinecap="round" />
          {r.eyes.map((p, i) => (
            <g key={i}>
              <rect x={p.x - 4.8} y={p.y - 3.5} width={9.6} height={7.2} rx={3} fill="#2a2a33" stroke={INK} strokeWidth={1} />
              <path d={`M${p.x - 3} ${p.y - 1.5} l3 -1`} stroke="#8fb0d8" strokeWidth={1.2} strokeLinecap="round" opacity={0.9} />
            </g>
          ))}
        </g>
      );
    case 'round_glasses':
      return (
        <g>
          <path d={`M${e0.x - 5} ${e0.y - 0.5} l-6 1.2`} stroke="#b8862b" strokeWidth={1.4} strokeLinecap="round" />
          <path d={`M${e0.x + 5} ${e0.y - 1} q${(e1.x - e0.x) / 2 - 5} -2.5 ${e1.x - e0.x - 10} 0`} stroke="#b8862b" strokeWidth={1.4} fill="none" />
          {r.eyes.map((p, i) => (
            <g key={i}>
              <circle cx={p.x} cy={p.y} r={5} fill="#cfe6ff" opacity={0.3} />
              <circle cx={p.x} cy={p.y} r={5} fill="none" stroke="#b8862b" strokeWidth={1.5} />
            </g>
          ))}
        </g>
      );
    case 'heart_glasses':
      return (
        <g>
          <path d={`M${e0.x - 5} ${e0.y - 1} l-6 1.2`} stroke="#b8275a" strokeWidth={1.4} strokeLinecap="round" />
          <path d={`M${e0.x + 4} ${e0.y - 1.5} L${e1.x - 4} ${e1.y - 1.5}`} stroke="#b8275a" strokeWidth={1.4} />
          {r.eyes.map((p, i) => (
            <path key={i} transform={`translate(${p.x} ${p.y + 0.5})`} d="M0 4.6 L-5 -0.6 A3 3 0 0 1 0 -3.8 A3 3 0 0 1 5 -0.6z"
              fill="#ff5c8a" opacity={0.85} stroke="#b8275a" strokeWidth={1.2} strokeLinejoin="round" />
          ))}
        </g>
      );
    case 'bandana':
      return (
        <g>
          <path d={`M${nx - nw} ${ny - 2} q${nw} 6 ${nw * 2} 0 l-2 4 q${-nw + 2} 5 ${-nw * 2 + 4} 0z`} fill="#e2453c" {...ln('#a52e28')} />
          <path d={`M${nx - 1} ${ny + 3} l-5 10 l10 -4z`} fill="#e2453c" {...ln('#a52e28')} />
          {[[-6, 1], [0, 3], [6, 1], [-3, 8], [1, 9]].map(([dx, dy], i) => (
            <circle key={i} cx={nx + dx} cy={ny + dy} r={0.9} fill="#fff" opacity={0.9} />
          ))}
        </g>
      );
    case 'scarf':
      return (
        <g>
          <path d={`M${nx - nw} ${ny - 2} q${nw} 6 ${nw * 2} 0 l0 5 q${-nw} 6 ${-nw * 2} 0z`} fill="#6fa8dc" {...ln('#3f6f9e')} />
          <path d={`M${nx - nw + 1} ${ny + 2} l-9 11 l7 2 l5 -11z`} fill="#4f86bd" {...ln('#3f6f9e')} />
          <path d={`M${nx - nw - 8} ${ny + 13} l-1 3 M${nx - nw - 5} ${ny + 14} l-1 3 M${nx - nw - 2} ${ny + 15} l-1 3`} stroke="#3f6f9e" strokeWidth={1.2} strokeLinecap="round" />
          <path d={`M${nx - nw + 3} ${ny + 1} q${nw - 3} 3 ${nw * 2 - 6} -1`} stroke="#fff" strokeWidth={1.2} fill="none" opacity={0.6} />
        </g>
      );
    case 'bowtie':
      return (
        <g>
          <path d={strap} stroke={INK} strokeWidth={1.4} fill="none" strokeLinecap="round" />
          <g transform={`translate(${nx} ${ny + 4})`}>
            <path d="M-1.5 0 l-8 -5 q-1.5 5 0 10z M1.5 0 l8 -5 q1.5 5 0 10z" fill="#e2453c" {...ln('#9a2b24')} />
            <rect x={-2.2} y={-2.8} width={4.4} height={5.6} rx={1.2} fill="#e2453c" stroke="#9a2b24" strokeWidth={1.2} />
          </g>
        </g>
      );
    case 'bell':
      return (
        <g>
          <path d={strap} stroke="#8a5a33" strokeWidth={3} fill="none" strokeLinecap="round" />
          <path d={`M${nx - 4} ${ny + 8} q0 -6 4 -6 q4 0 4 6 l1.5 1.5 h-11z`} fill="#ffd166" {...ln('#b08a3a')} />
          <circle cx={nx} cy={ny + 10.5} r={1.3} fill="#b08a3a" />
        </g>
      );
    case 'medal':
      return (
        <g>
          <path d={strap} stroke="#2f62b3" strokeWidth={3.4} fill="none" strokeLinecap="round" />
          <path d={`M${nx - 4} ${ny + 2} l4 7 l4 -7z`} fill="#e2453c" stroke="#9a2b24" strokeWidth={1} strokeLinejoin="round" />
          <circle cx={nx} cy={ny + 10} r={4.6} fill="#ffd166" {...ln('#b08a3a')} />
          <Star x={nx} y={ny + 10} r={2.6} fill="#b08a3a" />
        </g>
      );
    default:
      return null;
  }
}

function Accessory({ accessory, imageUrl, r }: { accessory?: string; imageUrl?: string; r: Rig }) {
  const a = parseAccessory(accessory);
  if (!a) return null;
  if (a.kind === 'builtin') return <Builtin item={a.item} r={r} />;
  if (a.kind === 'emoji') return <EmojiAccessory slot={a.slot} emoji={a.emoji} r={r} />;
  return <ImageAccessory slot={a.slot} url={imageUrl || a.url} r={r} />;
}

// ---------------------------------------------------------------------------

/**
 * A person's own animal picture (docs/custom-animals.md). It is drawn in a
 * square with its bottom edge on the ground line, and accessories go at
 * fixed spots given for a 64 x 64 picture: the top of the head, the eyes and
 * the neck. Keep in step with web/scripts/gen-sprite-kit.py.
 */
const SPRITE_BOX = { x: 14, y: 4, size: 72 };
const SPRITE_ANCHORS = { hat: { x: 46, y: 8 }, eyes: { x: 46, y: 19 }, neck: { x: 39, y: 32 } };
const spritePt = (p: Pt): Pt => ({ x: SPRITE_BOX.x + (p.x * SPRITE_BOX.size) / 64, y: SPRITE_BOX.y + (p.y * SPRITE_BOX.size) / 64 });
/** Where accessories sit on a picture, as a rig the accessory code understands. */
function spriteRig(): Rig {
  const hat = spritePt(SPRITE_ANCHORS.hat);
  const eyes = spritePt(SPRITE_ANCHORS.eyes);
  const neck = spritePt(SPRITE_ANCHORS.neck);
  return {
    ...RIGS.dog, ...resolveCoat('dog'),
    head: { x: eyes.x, y: eyes.y + 1, r: 12 },
    eyes: [{ x: eyes.x - 5, y: eyes.y }, { x: eyes.x + 5, y: eyes.y }],
    hat: { x: hat.x, y: hat.y, s: 1 },
    neck: { x: neck.x, y: neck.y - 6, w: 13 },
  };
}

/** The frames of a picture strip, left to right. Keep in step with model.PetSpriteFrames. */
const SPRITE_FRAMES = ['stand', 'walk1', 'walk2', 'sick', 'lying', 'happy'] as const;
type SpriteFrame = typeof SPRITE_FRAMES[number];

/** How many frames a w x h picture holds: 2 to 6 square frames side by side, else one. The server uses the same rule. */
export function spriteFrameCount(w: number, h: number): number {
  return h > 0 && w % h === 0 && w / h >= 2 && w / h <= SPRITE_FRAMES.length ? w / h : 1;
}

/** The URL of a person's own animal picture. */
export const spriteUrl = (id: string) => accessoryImageUrl(id);

export interface FarmAnimalProps {
  species: PetSpecies;
  mood: PetMood;
  /** The person's own picture (an upload id, or a preview URL with spritePreview) shown in place of the drawn animal. */
  sprite?: string;
  /** Treat `sprite` as a ready URL (a live preview before upload). */
  spritePreview?: boolean;
  /** Frames in the picture (see SPRITE_FRAMES); 1 or unset = a single picture. */
  spriteFrames?: number;
  /** Coat colour key from COAT_COLORS[species]; unknown or empty → the default coat. */
  color?: string;
  /** A built-in item key, `e:<head|face|neck>:<emoji>` or `img:<head|face|neck>:<id>`. */
  accessory?: string;
  /** Live preview of a kid's drawing: used instead of the upload URL for an `img:` accessory. */
  accessoryImageUrl?: string;
  /** A short-lived face override (eating, playing, being loved); ignored while lying down. */
  reaction?: PetReaction;
  className?: string;
  title?: string;
}

function Leg({ species, r, hip, len, far }: { species: PetSpecies; r: Rig; hip: Pt; len: number; far: boolean }) {
  if (species === 'chicken') {
    return (
      <g stroke="#f2a93b" strokeWidth={2.2} fill="none" strokeLinecap="round" strokeLinejoin="round">
        <path d={`M${hip.x} ${hip.y} v${len}`} />
        <path d={`M${hip.x - 4} ${hip.y + len + 1} l4 -1 l4.5 0 M${hip.x} ${hip.y + len} l-1 2 M${hip.x} ${hip.y + len} l2.5 2`} stroke="#d98f2b" strokeWidth={2.4} />
      </g>
    );
  }
  const fill = species === 'sheep' ? (far ? r.dark : r.face) : far ? r.shade : r.fur;
  const hoof = species === 'cow' || species === 'goat' || species === 'horse' || species === 'pig' || species === 'sheep';
  return (
    <g>
      <rect x={hip.x - 3.5} y={hip.y} width={7} height={len} rx={3} fill={fill} {...ln(r.line, 1.2)} />
      {hoof && <path d={`M${hip.x - 3.5} ${hip.y + len - 4} h7 v2 a3 3 0 0 1 -3 2 h-1 a3 3 0 0 1 -3 -2z`} fill={species === 'pig' ? r.hoof ?? r.line : '#4a3f38'} />}
    </g>
  );
}

/** One farm animal, 100 x 80, facing right, feet on the ground at y = 76. */
export function FarmAnimal({ species, mood, color, accessory, accessoryImageUrl, reaction, sprite, spritePreview, spriteFrames, className, title }: FarmAnimalProps) {
  const tintId = `farm-sick-${useId().replace(/[^a-zA-Z0-9_-]/g, '')}`;
  if (sprite) {
    const n = Math.max(1, Math.min(SPRITE_FRAMES.length, spriteFrames ?? 1));
    const has = (f: SpriteFrame) => SPRITE_FRAMES.indexOf(f) < n;
    const lying = mood === 'very_sick';
    const sick = lying || mood === 'sick';
    const { x, y, size } = SPRITE_BOX;
    const href = safeImageHref(spritePreview ? sprite : spriteUrl(sprite));
    if (!href) return null;
    // The frame shown while standing still. A strip with a frame for the
    // mood uses it; otherwise the standing frame is tinted green when sick
    // and squashed flat when too sick to stand.
    const rest: SpriteFrame = lying && has('lying') ? 'lying'
      : sick && has('sick') ? 'sick'
      : !sick && (mood === 'happy' || reaction) && has('happy') ? 'happy'
      : 'stand';
    const tinted = sick && rest === 'stand';
    const squashed = lying && rest !== 'lying';
    // Walking swaps between the two walk frames (the first walk frame and
    // the standing one if there are only two). Sick animals just shuffle.
    const walks = !sick && n > 1;
    const g = lying ? 0.55 : 0.35;
    const frame = (f: SpriteFrame, className?: string) => (
      // A window one frame wide onto the strip, pulled in a hair at the sides
      // so the edge of the neighbouring frame can't bleed through.
      <svg x={x} y={y} width={size} height={size} preserveAspectRatio="xMidYMax meet" className={className}
        viewBox={n > 1 ? `${SPRITE_FRAMES.indexOf(f) + 0.006} 0 0.988 1` : '0 0 1 1'}>
        <image href={href} x={0} y={0} width={n} height={1} preserveAspectRatio={n > 1 ? 'none' : 'xMidYMax meet'}
          pointerEvents="none" style={{ imageRendering: 'pixelated' }} filter={tinted ? `url(#${tintId})` : undefined} />
      </svg>
    );
    return (
      <svg viewBox="0 0 100 80" overflow="visible" className={className} role={title ? 'img' : undefined} aria-label={title} aria-hidden={title ? undefined : true}>
        <ellipse cx={50} cy={76} rx={26} ry={3.5} fill="#1d1b1a" opacity={0.14} />
        {tinted && (
          <defs>
            {/* Pull every colour part-way toward a sickly green; see-through pixels stay see-through. */}
            <filter id={tintId} colorInterpolationFilters="sRGB" x="0" y="0" width="1" height="1">
              <feColorMatrix type="matrix" values={[
                1 - g, 0, 0, 0, 0.42 * g,
                0, 1 - g, 0, 0, 0.72 * g,
                0, 0, 1 - g, 0, 0.30 * g,
                0, 0, 0, 1, 0,
              ].join(' ')} />
            </filter>
          </defs>
        )}
        <g transform={squashed ? 'translate(0 76) scale(1 0.6) translate(0 -76)' : undefined}>
          {frame(rest, walks ? 'farm-frame-rest' : undefined)}
          {walks && frame('walk1', 'farm-frame-walk-a')}
          {walks && frame(n > 2 ? 'walk2' : 'stand', 'farm-frame-walk-b')}
          {rest !== 'lying' && <Accessory accessory={accessory} imageUrl={accessoryImageUrl} r={spriteRig()} />}
        </g>
      </svg>
    );
  }
  const base: Rig = { ...(RIGS[species] ?? RIGS.dog), ...resolveCoat(species, color) };
  const lying = mood === 'very_sick';
  const react = lying ? undefined : reaction;
  const r = tint(base, lying ? 0.35 : mood === 'sick' ? 0.18 : 0);
  const { body, head } = r;
  // Lying down: drop the standing figure onto its belly, keeping the feet line.
  const pose = lying ? `translate(0 76) scale(1 0.9) translate(0 -${body.y + body.ry + 1})` : undefined;
  // Diagonal pairs swing together: far-back + near-front, near-back + far-front.
  const legClass = (i: number) => (i === 0 || i === 3 ? 'farm-leg-a' : 'farm-leg-b');

  return (
    <svg viewBox="0 0 100 80" overflow="visible" className={className} role={title ? 'img' : undefined} aria-label={title} aria-hidden={title ? undefined : true}>
      <ellipse cx={body.x + 6} cy={76} rx={body.rx + 8} ry={3.5} fill="#1d1b1a" opacity={0.14} />
      <g transform={pose}>
        <Tail species={species} r={r} fast={react === 'play'} />
        {!lying && r.legs.map((l, i) => (i === 0 || i === 2) && (
          <g key={i} className={legClass(i)}><Leg species={species} r={r} hip={l} len={l.len} far /></g>
        ))}
        {lying && species !== 'chicken' && (
          <g>
            <ellipse cx={body.x - body.rx + 8} cy={body.y + body.ry - 1} rx={7} ry={3.5} fill={r.shade} {...ln(r.line, 1.2)} />
            <ellipse cx={body.x + body.rx - 6} cy={body.y + body.ry - 1} rx={8} ry={3.5} fill={r.fur} {...ln(r.line, 1.2)} />
          </g>
        )}
        <ellipse cx={body.x} cy={body.y} rx={body.rx} ry={body.ry} fill={r.fur} {...ln(r.line)} />
        <BodyDetail species={species} r={r} />
        {!lying && r.legs.map((l, i) => (i === 1 || i === 3) && (
          <g key={i} className={legClass(i)}><Leg species={species} r={r} hip={l} len={l.len} far={false} /></g>
        ))}
        {species !== 'chicken' && species !== 'sheep' && (
          <ellipse cx={body.x + 2} cy={body.y + body.ry - 4} rx={body.rx * 0.55} ry={4} fill="#fff" opacity={species === 'cow' ? 0 : 0.18} />
        )}
        <HeadBack species={species} r={r} />
        <circle cx={head.x} cy={head.y} r={head.r} fill={r.face} {...ln(species === 'sheep' ? r.darkLine : r.line)} />
        <path d={`M${head.x - head.r * 0.55} ${head.y - head.r * 0.6} q${head.r * 0.5} -${head.r * 0.35} ${head.r} 0`} stroke="#fff" strokeWidth={1.6} fill="none" opacity={0.35} strokeLinecap="round" />
        <HeadFront species={species} r={r} />
        <Face r={r} mood={mood} species={species} reaction={react} />
        <Accessory accessory={accessory} imageUrl={accessoryImageUrl} r={r} />
      </g>
    </svg>
  );
}

/** A small wooden hutch with a dark doorway; a hiding animal's eyes peek out. */
export function Shelter({ color, className, peeking }: { color?: string; className?: string; peeking?: boolean }) {
  const wall = color ?? '#e8c98f';
  return (
    <svg viewBox="0 0 80 70" className={className} aria-hidden="true">
      <ellipse cx={40} cy={67} rx={36} ry={3} fill="#1d1b1a" opacity={0.14} />
      <path d="M12 30 h56 v36 h-56z" fill={wall} {...ln('#8a6a44')} />
      <path d="M12 42 h56 M12 54 h56" stroke="#8a6a44" strokeWidth={1} opacity={0.45} />
      <path d="M26 66 v-20 q14 -13 28 0 v20z" fill="#2a211c" {...ln('#5a4634')} />
      <path d="M4 31 L40 5 L76 31 Z" fill="#c4553f" {...ln('#7d2f27')} />
      <path d="M22 18 l18 -13 l18 13" stroke="#e07a62" strokeWidth={2} fill="none" strokeLinecap="round" opacity={0.7} />
      <path d="M2 32 h76" {...ln('#7d2f27', 3)} />
      <circle cx={40} cy={22} r={4} fill="#fff6d6" {...ln('#7d2f27', 1.2)} />
      <path d="M6 66 q3 -6 5 0 M70 66 q3 -7 5 0" stroke="#6fa84f" strokeWidth={2} fill="none" strokeLinecap="round" />
      {peeking && (
        <g className="farm-peek">
          <circle cx={34} cy={56} r={3} fill="#fff" />
          <circle cx={46} cy={56} r={3} fill="#fff" />
          <circle cx={34.7} cy={56.4} r={1.6} fill="#1d1b1a" />
          <circle cx={46.7} cy={56.4} r={1.6} fill="#1d1b1a" />
        </g>
      )}
    </svg>
  );
}

// ---------------------------------------------------------------------------
// Scenery

/** Stars as [left %, top %, size px]: kept to the sky, never stretched. */
const STARS: readonly [number, number, number][] = [
  [6, 7, 2], [14, 18, 3], [21, 5, 2], [27, 26, 2], [33, 11, 3], [44, 4, 2], [49, 21, 2], [55, 9, 3],
  [61, 27, 2], [66, 4, 2], [72, 16, 3], [78, 7, 2], [86, 22, 2], [91, 6, 3], [96, 15, 2], [39, 30, 2],
];

/**
 * The farm's scenery: sky extras (stars, moon or sun), two rolling hills, the
 * barn and the fence. Every colour and every theme-dependent part comes from
 * the --farm-* dials in themes.css, so the same markup is a sunny pastel farm,
 * a dusk farm or a bold flat farm depending on the skin around it.
 * Render it first inside the scene root (s.farm).
 */
export function FarmBackdrop() {
  const posts = `farm-posts-${useId().replace(/[^a-zA-Z0-9_-]/g, '')}`;
  return (
    <>
      <div className={s.sky} aria-hidden="true">
        {STARS.map(([x, y, d], i) => (
          <span key={i} className={s.star} style={{ left: `${x}%`, top: `${y}%`, width: d, height: d, animationDelay: `${(i * 0.83) % 5}s` }} />
        ))}
        <svg className={s.moon} viewBox="0 0 28 28">
          <path d="M17 3 A11 11 0 1 0 25 20 A9 9 0 0 1 17 3z" />
          <circle className={s.crater} cx={9} cy={15} r={1.8} />
          <circle className={s.crater} cx={13} cy={21} r={1.2} />
        </svg>
        <svg className={s.sun} viewBox="0 0 40 40">
          <path className={s.sunRays} d="M20 1 v6 M20 33 v6 M1 20 h6 M33 20 h6 M6.6 6.6 l4.2 4.2 M29.2 29.2 l4.2 4.2 M6.6 33.4 l4.2 -4.2 M29.2 10.8 l4.2 -4.2" />
          <circle cx={20} cy={20} r={9.5} />
        </svg>
      </div>
      <svg className={s.backdrop} viewBox="0 0 400 200" preserveAspectRatio="none" aria-hidden="true">
        <path className={s.hillFar} d="M0 92 Q80 70 170 88 T400 80 V200 H0z" />
        <path className={s.ridge} d="M-2 92 Q80 70 170 88 T402 80" />
        <path className={s.hillNear} d="M0 104 Q120 88 240 104 T400 100 V200 H0z" />
        <path className={s.ridge} d="M-2 104 Q120 88 240 104 T402 100" />
      </svg>
      <svg className={s.barn} viewBox="0 0 120 100" preserveAspectRatio="xMaxYMax meet" aria-hidden="true">
        <rect className={s.barnWall} x={18} y={42} width={84} height={56} />
        <path className={s.barnRoof} d="M10 44 L60 10 L110 44z" />
        <path className={s.barnDoor} d="M44 98 V62 H76 V98z" />
        <path className={s.barnTrim} d="M44 98 V62 H76 V98z M44 62 L76 98 M76 62 L44 98" />
        <rect className={s.barnWindow} x={52} y={24} width={16} height={12} />
        <g className={s.barnLamps}>
          <rect className={s.barnWindow} x={25} y={60} width={12} height={12} rx={1.5} />
          <rect className={s.barnWindow} x={83} y={60} width={12} height={12} rx={1.5} />
          <path className={s.barnMullion} d="M31 60 v12 M25 66 h12 M89 60 v12 M83 66 h12" />
        </g>
      </svg>
      <svg className={s.fence} aria-hidden="true">
        <defs>
          <pattern id={posts} width={34} height={34} patternUnits="userSpaceOnUse">
            <line className={s.postEdge} x1={6} y1={8} x2={6} y2={30} />
            <line className={s.post} x1={6} y1={8} x2={6} y2={30} />
          </pattern>
        </defs>
        <rect x={0} y={0} width="100%" height={34} fill={`url(#${posts})`} />
        <line className={s.railEdge} x1={-10} y1={15.5} x2="110%" y2={15.5} />
        <line className={s.railEdge} x1={-10} y1={23.5} x2="110%" y2={23.5} />
        <line className={s.rail} x1={-10} y1={15.5} x2="110%" y2={15.5} />
        <line className={s.rail} x1={-10} y1={23.5} x2="110%" y2={23.5} />
      </svg>
    </>
  );
}
