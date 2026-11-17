// Original side-view farm animals drawn in SVG, facing right (the scene flips
// them to walk left). Every species is its own little rig: a body, a big round
// head, four (or two) legs and a set of anchors (eyes, mouth, hat, collar) that
// the shared face and accessory components read, so every mood and accessory
// lands correctly on every animal. Flat, friendly children's-book style with
// soft outlines in a darker tint of each animal's own colour.
import React from 'react';
import type { PetItem, PetMood, PetSpecies } from '../../../types';

const INK = '#2b2522';

/** Blend a hex colour towards white (0 = unchanged, 1 = white). */
function pale(hex: string, t: number): string {
  const n = parseInt(hex.slice(1), 16);
  const ch = (shift: number) => Math.round(((n >> shift) & 255) + (255 - ((n >> shift) & 255)) * t);
  return `#${[16, 8, 0].map(s => ch(s).toString(16).padStart(2, '0')).join('')}`;
}

interface Pt { x: number; y: number }

interface Rig {
  fur: string;        // main body colour
  shade: string;      // far legs, underside, inner ears
  line: string;       // outline colour
  face: string;       // head colour (sheep's dark face)
  ink: string;        // eye/mouth ink for this face
  body: { x: number; y: number; rx: number; ry: number };
  head: { x: number; y: number; r: number };
  eyes: [Pt, Pt];
  mouth: Pt;
  cheek: Pt;
  legs: { x: number; y: number; len: number }[]; // hip point + length; index 0/3 swing together
  hat: { x: number; y: number; s: number };       // crown of the head, scale
  neck: { x: number; y: number; w: number };      // where a collar sits
}

const RIGS: Record<PetSpecies, Rig> = {
  horse: {
    fur: '#c98f5e', shade: '#a97245', line: '#6b4327', face: '#c98f5e', ink: INK,
    body: { x: 40, y: 50, rx: 25, ry: 14 }, head: { x: 74, y: 27, r: 12 },
    eyes: [{ x: 70, y: 24 }, { x: 79, y: 25 }], mouth: { x: 89, y: 39 }, cheek: { x: 76, y: 32 },
    legs: [{ x: 24, y: 58, len: 18 }, { x: 31, y: 58, len: 18 }, { x: 50, y: 58, len: 18 }, { x: 57, y: 58, len: 18 }],
    hat: { x: 73, y: 15, s: 0.95 }, neck: { x: 68, y: 40, w: 13 },
  },
  dog: {
    fur: '#e8b472', shade: '#cf9552', line: '#7a4f2a', face: '#e8b472', ink: INK,
    body: { x: 41, y: 51, rx: 25, ry: 14 }, head: { x: 74, y: 31, r: 14 },
    eyes: [{ x: 69, y: 28 }, { x: 79, y: 28 }], mouth: { x: 84, y: 40 }, cheek: { x: 72, y: 36 },
    legs: [{ x: 24, y: 58, len: 18 }, { x: 31, y: 58, len: 18 }, { x: 50, y: 58, len: 18 }, { x: 57, y: 58, len: 18 }],
    hat: { x: 74, y: 17, s: 1 }, neck: { x: 66, y: 44, w: 14 },
  },
  cat: {
    fur: '#f0a455', shade: '#d6853a', line: '#8a4f1f', face: '#f0a455', ink: INK,
    body: { x: 42, y: 52, rx: 24, ry: 13 }, head: { x: 74, y: 32, r: 13 },
    eyes: [{ x: 69, y: 30 }, { x: 79, y: 30 }], mouth: { x: 80, y: 39 }, cheek: { x: 72, y: 37 },
    legs: [{ x: 26, y: 58, len: 18 }, { x: 33, y: 58, len: 18 }, { x: 50, y: 58, len: 18 }, { x: 57, y: 58, len: 18 }],
    hat: { x: 74, y: 19, s: 0.95 }, neck: { x: 66, y: 44, w: 13 },
  },
  chicken: {
    fur: '#fff8ea', shade: '#eadfc8', line: '#b8a27c', face: '#fff8ea', ink: INK,
    body: { x: 44, y: 53, rx: 21, ry: 15 }, head: { x: 68, y: 29, r: 12 },
    eyes: [{ x: 64, y: 26 }, { x: 73, y: 26 }], mouth: { x: 0, y: 0 }, cheek: { x: 70, y: 33 },
    legs: [{ x: 40, y: 66, len: 10 }, { x: 50, y: 66, len: 10 }],
    hat: { x: 68, y: 17, s: 0.9 }, neck: { x: 62, y: 41, w: 11 },
  },
  cow: {
    fur: '#fffaf0', shade: '#eee4d2', line: '#6b5e56', face: '#fffaf0', ink: INK,
    body: { x: 41, y: 50, rx: 27, ry: 15 }, head: { x: 74, y: 30, r: 14 },
    eyes: [{ x: 69, y: 27 }, { x: 79, y: 27 }], mouth: { x: 83, y: 41 }, cheek: { x: 70, y: 34 },
    legs: [{ x: 22, y: 58, len: 18 }, { x: 29, y: 58, len: 18 }, { x: 50, y: 58, len: 18 }, { x: 57, y: 58, len: 18 }],
    hat: { x: 74, y: 16, s: 1 }, neck: { x: 66, y: 44, w: 14 },
  },
  pig: {
    fur: '#f7b7c4', shade: '#ea98ab', line: '#b35f78', face: '#f7b7c4', ink: INK,
    body: { x: 42, y: 52, rx: 26, ry: 15 }, head: { x: 74, y: 34, r: 14 },
    eyes: [{ x: 69, y: 31 }, { x: 79, y: 31 }], mouth: { x: 82, y: 45 }, cheek: { x: 70, y: 38 },
    legs: [{ x: 24, y: 60, len: 16 }, { x: 31, y: 60, len: 16 }, { x: 50, y: 60, len: 16 }, { x: 57, y: 60, len: 16 }],
    hat: { x: 74, y: 20, s: 1 }, neck: { x: 66, y: 47, w: 14 },
  },
  sheep: {
    fur: '#fbf8f1', shade: '#e6dfcf', line: '#a39a88', face: '#4d4543', ink: '#f6f1e8',
    body: { x: 42, y: 50, rx: 25, ry: 15 }, head: { x: 75, y: 33, r: 12 },
    eyes: [{ x: 71, y: 30 }, { x: 80, y: 30 }], mouth: { x: 80, y: 40 }, cheek: { x: 72, y: 37 },
    legs: [{ x: 26, y: 58, len: 18 }, { x: 33, y: 58, len: 18 }, { x: 50, y: 58, len: 18 }, { x: 57, y: 58, len: 18 }],
    hat: { x: 74, y: 17, s: 0.95 }, neck: { x: 67, y: 45, w: 12 },
  },
  goat: {
    fur: '#e9e0cf', shade: '#d3c6ab', line: '#7a6a55', face: '#e9e0cf', ink: INK,
    body: { x: 41, y: 51, rx: 25, ry: 14 }, head: { x: 74, y: 30, r: 12 },
    eyes: [{ x: 70, y: 27 }, { x: 79, y: 27 }], mouth: { x: 85, y: 40 }, cheek: { x: 72, y: 34 },
    legs: [{ x: 24, y: 58, len: 18 }, { x: 31, y: 58, len: 18 }, { x: 50, y: 58, len: 18 }, { x: 57, y: 58, len: 18 }],
    hat: { x: 73, y: 18, s: 0.9 }, neck: { x: 67, y: 43, w: 13 },
  },
};

/** Apply the pale/sick tint to a rig's colours. */
function tint(r: Rig, t: number): Rig {
  if (t === 0) return r;
  return { ...r, fur: pale(r.fur, t), shade: pale(r.shade, t), line: pale(r.line, t * 0.6), face: pale(r.face, r.face === r.fur ? t : t * 0.4) };
}

const LINE = { strokeWidth: 1.5, strokeLinejoin: 'round' as const, strokeLinecap: 'round' as const };

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

function Face({ r, mood }: { r: Rig; mood: PetMood }) {
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

function Tail({ species, r }: { species: PetSpecies; r: Rig }) {
  const { x, y, rx } = r.body;
  const bx = x - rx + 3;
  switch (species) {
    case 'horse':
      return <path d={`M${bx + 2} ${y - 8} q-12 6 -11 26 q2 -4 5 -6 q1 4 4 6 q3 -14 6 -22z`} fill="#5c3a22" stroke="#4a2d19" {...LINE} />;
    case 'dog':
      return <path className="farm-tail" d={`M${bx + 3} ${y - 5} q-9 -4 -12 -16`} stroke={r.shade} strokeWidth={5} fill="none" strokeLinecap="round" />;
    case 'cat':
      return (
        <g className="farm-tail">
          <path d={`M${bx + 2} ${y + 2} q-13 -2 -11 -17 q1 -7 7 -5`} stroke={r.line} strokeWidth={7} fill="none" strokeLinecap="round" />
          <path d={`M${bx + 2} ${y + 2} q-13 -2 -11 -17 q1 -7 7 -5`} stroke={r.fur} strokeWidth={4.5} fill="none" strokeLinecap="round" />
        </g>
      );
    case 'chicken':
      return (
        <path d={`M${bx + 6} ${y - 4} q-8 -6 -8 -20 q5 5 8 10 q-2 -9 3 -16 q3 8 4 14 q1 -7 6 -11 q0 10 -2 20z`}
          fill={r.fur} stroke={r.line} {...LINE} />
      );
    case 'cow':
      return (
        <g>
          <path d={`M${bx + 2} ${y - 7} q-9 6 -8 24`} stroke={r.line} strokeWidth={2.2} fill="none" strokeLinecap="round" />
          <path d={`M${bx - 6} ${y + 15} q-3 3 0 7 q3 -3 3 -7z`} fill="#3d3532" />
        </g>
      );
    case 'pig':
      return <path d={`M${bx + 2} ${y - 4} q-7 -5 -4 -9 q4 -2 3 2 q-1 3 -5 2`} stroke={r.line} strokeWidth={2} fill="none" strokeLinecap="round" />;
    case 'sheep':
      return <circle cx={bx - 1} cy={y - 6} r={5} fill={r.fur} stroke={r.line} {...LINE} />;
    case 'goat':
      return <path d={`M${bx + 3} ${y - 6} l-7 -8 q5 1 9 5z`} fill={r.fur} stroke={r.line} {...LINE} />;
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
          <path d={`M${x + 12} ${y - 10} Q${x + 20} ${y - 26} ${r.head.x - 2} ${r.head.y - 6} L${r.head.x + 6} ${r.head.y + 8} Q${x + 24} ${y} ${x + 18} ${y + 6}z`} fill={r.fur} stroke={r.line} {...LINE} />
          <path d={`M${r.head.x - 3} ${r.head.y - r.head.r - 1} q-9 2 -11 9 q-7 2 -8 9 q-6 3 -6 9 q-3 2 -2 6 L${x + 11} ${y - 9} Q${x + 20} ${y - 26} ${r.head.x - 1} ${r.head.y - 5}z`} fill="#5c3a22" stroke="#4a2d19" {...LINE} />
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
          <path d={`M${x - 6} ${y - 3} q10 -8 20 0 q-8 11 -20 0z`} fill={r.shade} stroke={r.line} {...LINE} />
        </g>
      );
    case 'cow':
      return (
        <g>
          <path d={`M${x - 10} ${y - 10} q10 -4 14 4 q-2 10 -12 7 q-5 -5 -2 -11z`} fill="#3d3532" />
          <path d={`M${x + 12} ${y + 2} q9 -3 11 5 q-7 6 -13 1z`} fill="#3d3532" />
          <path d={`M${x - 22} ${y + 4} q5 -3 7 4 q-5 4 -8 0z`} fill="#3d3532" />
          <ellipse cx={x - 8} cy={y + ry - 1} rx={6} ry={3.5} fill="#f4b6c2" />
        </g>
      );
    case 'pig':
      return <ellipse cx={x} cy={y + 6} rx={14} ry={6} fill={r.shade} opacity={0.45} />;
    case 'sheep':
      return (
        <g>
          {[[-18, -6], [-8, -12], [4, -13], [15, -9], [21, 0], [14, 9], [2, 12], [-11, 10], [-20, 5]].map(([dx, dy], i) => (
            <circle key={i} cx={x + dx} cy={y + dy} r={8.5} fill={r.fur} stroke={r.line} {...LINE} />
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
          <path d={`M${x - 7} ${y - hr + 3} l-1 -9 l6 6z M${x + 2} ${y - hr + 1} l3 -8 l4 8z`} fill={r.fur} stroke={r.line} {...LINE} />
          <path d={`M${x - 4} ${y - hr + 1} q-4 4 -6 10 q4 -2 6 -5 q2 3 6 4 q-1 -6 -6 -9z`} fill="#5c3a22" stroke="#4a2d19" {...LINE} />
        </g>
      );
    case 'dog':
      return <path d={`M${x + 2} ${y - hr + 2} q8 -3 10 4 q-1 4 -4 5 q-2 -6 -6 -9z`} fill="#b8783f" stroke={r.line} {...LINE} />;
    case 'cat':
      return (
        <g>
          <path d={`M${x - 10} ${y - 6} l-1 -13 l11 7z M${x + 3} ${y - 12} l9 -8 l1 13z`} fill={r.fur} stroke={r.line} {...LINE} />
          <path d={`M${x - 8} ${y - 8} l0 -7 l6 4z M${x + 5} ${y - 12} l5 -4 l0 7z`} fill="#f7c3cf" />
        </g>
      );
    case 'chicken':
      return (
        <path d={`M${x - 8} ${y - hr + 3} q0 -8 5 -6 q1 -7 6 -3 q3 -6 6 1 q2 -3 4 2 q-6 2 -21 6z`} fill="#e5473f" stroke="#b7342d" {...LINE} />
      );
    case 'cow':
      return (
        <g>
          <path d={`M${x - 7} ${y - hr + 3} q-3 -9 -9 -9`} stroke="#e8d7b1" strokeWidth={4} fill="none" strokeLinecap="round" />
          <path d={`M${x + 7} ${y - hr + 3} q3 -9 9 -9`} stroke="#e8d7b1" strokeWidth={4} fill="none" strokeLinecap="round" />
          <ellipse cx={x - hr - 2} cy={y - 2} rx={6} ry={3.5} fill={r.fur} stroke={r.line} {...LINE} />
        </g>
      );
    case 'pig':
      return (
        <g>
          <path d={`M${x - 10} ${y - 7} l-3 -11 l11 5z M${x + 4} ${y - 12} l8 -8 l2 11z`} fill={r.fur} stroke={r.line} {...LINE} />
          <path d={`M${x - 8} ${y - 9} l-1 -5 l5 3z M${x + 6} ${y - 12} l4 -4 l1 6z`} fill={r.shade} />
        </g>
      );
    case 'sheep':
      return (
        <g>
          {[[-6, -9], [2, -11], [8, -8]].map(([dx, dy], i) => (
            <circle key={i} cx={x + dx} cy={y + dy} r={5.5} fill={r.fur} stroke={r.line} {...LINE} />
          ))}
          <ellipse cx={x - hr} cy={y + 1} rx={5} ry={3} fill={r.face} stroke="#3a3331" {...LINE} />
        </g>
      );
    case 'goat':
      return (
        <g>
          <path d={`M${x - 5} ${y - hr + 2} q-4 -10 -12 -12`} stroke="#9b8a73" strokeWidth={3.5} fill="none" strokeLinecap="round" />
          <path d={`M${x + 2} ${y - hr + 1} q-1 -11 -8 -14`} stroke="#9b8a73" strokeWidth={3.5} fill="none" strokeLinecap="round" />
          <path d={`M${x - hr + 2} ${y - 2} q-9 -1 -9 6 q6 2 10 -2z`} fill={r.fur} stroke={r.line} {...LINE} />
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
          <ellipse cx={x + 13} cy={y + 9} rx={10} ry={7.5} fill="#e4b58d" stroke={r.line} {...LINE} />
          <ellipse cx={x + 18} cy={y + 7} rx={1.4} ry={1.1} fill={r.line} />
        </g>
      );
    case 'dog':
      return (
        <g>
          <ellipse cx={x + 9} cy={y + 8} rx={8.5} ry={6.5} fill="#f8e2c3" stroke={r.line} {...LINE} />
          <ellipse cx={x + 14} cy={y + 5} rx={3} ry={2.4} fill="#3a2a24" />
          <path d={`M${x - 4} ${y - hr + 2} q-9 1 -12 10 q-2 9 4 13 q5 -3 5 -12 q0 -7 3 -11z`} fill="#b8783f" stroke={r.line} {...LINE} />
        </g>
      );
    case 'cat':
      return (
        <g>
          <ellipse cx={x + 7} cy={y + 7} rx={6} ry={4.2} fill="#fbe3c8" />
          <path d={`M${x + 5.5} ${y + 3.5} l3 0 l-1.5 2.2z`} fill="#e4788f" />
          <path d={`M${x + 11} ${y + 4.5} l10 -2.5 M${x + 11} ${y + 7} l10 1.5 M${x + 3} ${y + 6} l-8 -1 M${x + 3} ${y + 8} l-8 1.5`}
            stroke={r.line} strokeWidth={0.9} strokeLinecap="round" />
          <path d={`M${x - 2} ${y - hr + 1} q2 4 0 7 M${x + 4} ${y - hr + 2} q2 4 0 7`} stroke={r.shade} strokeWidth={2} fill="none" strokeLinecap="round" />
        </g>
      );
    case 'chicken':
      return (
        <g>
          <path d={`M${x + 9} ${y - 1} l9 3 l-9 3z`} fill="#f2a93b" stroke="#c98427" {...LINE} />
          <path d={`M${x + 9} ${y + 5} q3 6 -1 8 q-3 -4 1 -8z`} fill="#e5473f" stroke="#b7342d" {...LINE} />
        </g>
      );
    case 'cow':
      return (
        <g>
          <path d={`M${x - 11} ${y - 9} q8 -4 12 3 q-1 8 -9 7 q-5 -4 -3 -10z`} fill="#3d3532" />
          <ellipse cx={x + 9} cy={y + 10} rx={10} ry={6.5} fill="#f4b6c2" stroke={r.line} {...LINE} />
          <ellipse cx={x + 6} cy={y + 9.5} rx={1.4} ry={1.1} fill="#a86a78" />
          <ellipse cx={x + 13} cy={y + 9.5} rx={1.4} ry={1.1} fill="#a86a78" />
        </g>
      );
    case 'pig':
      return (
        <g>
          <ellipse cx={x + 11} cy={y + 6} rx={6.5} ry={5.5} fill="#f28fa6" stroke={r.line} {...LINE} />
          <ellipse cx={x + 9} cy={y + 6} rx={1.3} ry={1.7} fill="#b0546d" />
          <ellipse cx={x + 13.5} cy={y + 6} rx={1.3} ry={1.7} fill="#b0546d" />
        </g>
      );
    case 'sheep':
      return <ellipse cx={x + hr - 1} cy={y + 1} rx={5} ry={3} fill={r.face} stroke="#3a3331" {...LINE} />;
    case 'goat':
      return (
        <g>
          <ellipse cx={x + 11} cy={y + 8} rx={8} ry={6} fill="#f4ede0" stroke={r.line} {...LINE} />
          <path d={`M${x + 7} ${y + 13} q2 8 4 10 q2 -4 3 -10z`} fill={r.shade} stroke={r.line} {...LINE} />
          <ellipse cx={x + 16} cy={y + 6} rx={1.3} ry={1} fill={r.line} />
        </g>
      );
  }
}

// ---------------------------------------------------------------------------
// Accessories

function Accessory({ item, r }: { item: PetItem; r: Rig }) {
  const { x: hx, y: hy, s } = r.hat;
  const { x: nx, y: ny, w: nw } = r.neck;
  switch (item) {
    case 'straw_hat':
      return (
        <g transform={`translate(${hx} ${hy}) scale(${s})`}>
          <ellipse cx={0} cy={1} rx={17} ry={4.5} fill="#f0cf74" stroke="#b08a3a" {...LINE} />
          <path d="M-8 1 q-1 -11 8 -11 q9 0 8 11z" fill="#f6dd8c" stroke="#b08a3a" {...LINE} />
          <path d="M-8 -1 q8 2 16 0" stroke="#e2453c" strokeWidth={2.4} fill="none" />
        </g>
      );
    case 'cowboy_hat':
      return (
        <g transform={`translate(${hx} ${hy}) scale(${s})`}>
          <path d="M-18 -1 q2 7 18 6 q16 1 18 -6 q-6 5 -18 3 q-12 2 -18 -3z" fill="#9a6336" stroke="#5e3a1e" {...LINE} />
          <path d="M-9 2 q-1 -13 4 -13 q3 3 6 0 q6 -1 6 13z" fill="#b6773f" stroke="#5e3a1e" {...LINE} />
          <path d="M-9 1 h18" stroke="#5e3a1e" strokeWidth={2} />
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
    case 'bow':
      return (
        <g transform={`translate(${hx + 7 * s} ${hy + 1}) scale(${s}) rotate(-15)`}>
          <path d="M-1 0 q-7 -8 -10 -3 q0 7 10 3z M1 0 q7 -8 10 -3 q0 7 -10 3z" fill="#f05a7a" stroke="#a83350" {...LINE} />
          <path d="M-1 1 q-6 7 -8 4 M1 1 q6 7 8 4" stroke="#a83350" strokeWidth={1.2} fill="none" />
          <circle cx={0} cy={0} r={2.6} fill="#f05a7a" stroke="#a83350" {...LINE} />
        </g>
      );
    case 'sunglasses':
      return (
        <g>
          <path d={`M${r.eyes[0].x - 5} ${r.eyes[0].y - 1} H${r.eyes[1].x + 5}`} stroke={INK} strokeWidth={1.5} />
          <path d={`M${r.eyes[0].x - 5} ${r.eyes[0].y - 1} l-6 1`} stroke={INK} strokeWidth={1.5} strokeLinecap="round" />
          {r.eyes.map((p, i) => (
            <g key={i}>
              <rect x={p.x - 4.8} y={p.y - 3.5} width={9.6} height={7.2} rx={3} fill="#2a2a33" stroke={INK} strokeWidth={1} />
              <path d={`M${p.x - 3} ${p.y - 1.5} l3 -1`} stroke="#8fb0d8" strokeWidth={1.2} strokeLinecap="round" opacity={0.9} />
            </g>
          ))}
        </g>
      );
    case 'bandana':
      return (
        <g>
          <path d={`M${nx - nw} ${ny - 2} q${nw} 6 ${nw * 2} 0 l-2 4 q${-nw + 2} 5 ${-nw * 2 + 4} 0z`} fill="#e2453c" stroke="#a52e28" {...LINE} />
          <path d={`M${nx - 1} ${ny + 3} l-5 10 l10 -4z`} fill="#e2453c" stroke="#a52e28" {...LINE} />
          {[[-6, 1], [0, 3], [6, 1], [-3, 8], [1, 9]].map(([dx, dy], i) => (
            <circle key={i} cx={nx + dx} cy={ny + dy} r={0.9} fill="#fff" opacity={0.9} />
          ))}
        </g>
      );
    case 'scarf':
      return (
        <g>
          <path d={`M${nx - nw} ${ny - 2} q${nw} 6 ${nw * 2} 0 l0 5 q${-nw} 6 ${-nw * 2} 0z`} fill="#6fa8dc" stroke="#3f6f9e" {...LINE} />
          <path d={`M${nx - nw + 1} ${ny + 2} l-9 11 l7 2 l5 -11z`} fill="#4f86bd" stroke="#3f6f9e" {...LINE} />
          <path d={`M${nx - nw - 8} ${ny + 13} l-1 3 M${nx - nw - 5} ${ny + 14} l-1 3 M${nx - nw - 2} ${ny + 15} l-1 3`} stroke="#3f6f9e" strokeWidth={1.2} strokeLinecap="round" />
          <path d={`M${nx - nw + 3} ${ny + 1} q${nw - 3} 3 ${nw * 2 - 6} -1`} stroke="#fff" strokeWidth={1.2} fill="none" opacity={0.6} />
        </g>
      );
    case 'bell':
      return (
        <g>
          <path d={`M${nx - nw} ${ny - 2} q${nw} 7 ${nw * 2} 0`} stroke="#8a5a33" strokeWidth={3} fill="none" strokeLinecap="round" />
          <path d={`M${nx - 4} ${ny + 8} q0 -6 4 -6 q4 0 4 6 l1.5 1.5 h-11z`} fill="#ffd166" stroke="#b08a3a" {...LINE} />
          <circle cx={nx} cy={ny + 10.5} r={1.3} fill="#b08a3a" />
        </g>
      );
  }
}

// ---------------------------------------------------------------------------

export interface FarmAnimalProps {
  species: PetSpecies;
  mood: PetMood;
  accessory?: PetItem | '';
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
  const fill = species === 'sheep' ? (far ? '#3a3331' : r.face) : far ? r.shade : r.fur;
  const hoof = species === 'cow' || species === 'goat' || species === 'horse' || species === 'pig' || species === 'sheep';
  return (
    <g>
      <rect x={hip.x - 3.5} y={hip.y} width={7} height={len} rx={3} fill={fill} stroke={r.line} strokeWidth={1.2} />
      {hoof && <path d={`M${hip.x - 3.5} ${hip.y + len - 4} h7 v2 a3 3 0 0 1 -3 2 h-1 a3 3 0 0 1 -3 -2z`} fill={species === 'pig' ? '#b35f78' : '#4a3f38'} />}
    </g>
  );
}

/** One farm animal, 100 x 80, facing right, feet on the ground at y = 76. */
export function FarmAnimal({ species, mood, accessory, className, title }: FarmAnimalProps) {
  const base = RIGS[species] ?? RIGS.dog;
  const lying = mood === 'very_sick';
  const r = tint(base, lying ? 0.35 : mood === 'sick' ? 0.18 : 0);
  const { body, head } = r;
  // Lying down: drop the standing figure onto its belly, keeping the feet line.
  const pose = lying ? `translate(0 76) scale(1 0.9) translate(0 -${body.y + body.ry + 1})` : undefined;
  // Diagonal pairs swing together: far-back + near-front, near-back + far-front.
  const legClass = (i: number) => (i === 0 || i === 3 ? 'farm-leg-a' : 'farm-leg-b');

  return (
    <svg viewBox="0 0 100 80" className={className} role={title ? 'img' : undefined} aria-label={title} aria-hidden={title ? undefined : true}>
      <ellipse cx={body.x + 6} cy={76} rx={body.rx + 8} ry={3.5} fill="#1d1b1a" opacity={0.14} />
      <g transform={pose}>
        <Tail species={species} r={r} />
        {!lying && r.legs.map((l, i) => (i === 0 || i === 2) && (
          <g key={i} className={legClass(i)}><Leg species={species} r={r} hip={l} len={l.len} far /></g>
        ))}
        {lying && species !== 'chicken' && (
          <g>
            <ellipse cx={body.x - body.rx + 8} cy={body.y + body.ry - 1} rx={7} ry={3.5} fill={r.shade} stroke={r.line} strokeWidth={1.2} />
            <ellipse cx={body.x + body.rx - 6} cy={body.y + body.ry - 1} rx={8} ry={3.5} fill={r.fur} stroke={r.line} strokeWidth={1.2} />
          </g>
        )}
        <ellipse cx={body.x} cy={body.y} rx={body.rx} ry={body.ry} fill={r.fur} stroke={r.line} {...LINE} />
        <BodyDetail species={species} r={r} />
        {!lying && r.legs.map((l, i) => (i === 1 || i === 3) && (
          <g key={i} className={legClass(i)}><Leg species={species} r={r} hip={l} len={l.len} far={false} /></g>
        ))}
        {species !== 'chicken' && species !== 'sheep' && (
          <ellipse cx={body.x + 2} cy={body.y + body.ry - 4} rx={body.rx * 0.55} ry={4} fill="#fff" opacity={species === 'cow' ? 0 : 0.18} />
        )}
        <HeadBack species={species} r={r} />
        <circle cx={head.x} cy={head.y} r={head.r} fill={r.face} stroke={species === 'sheep' ? '#3a3331' : r.line} {...LINE} />
        <path d={`M${head.x - head.r * 0.55} ${head.y - head.r * 0.6} q${head.r * 0.5} -${head.r * 0.35} ${head.r} 0`} stroke="#fff" strokeWidth={1.6} fill="none" opacity={0.35} strokeLinecap="round" />
        <HeadFront species={species} r={r} />
        <Face r={r} mood={mood} />
        {accessory ? <Accessory item={accessory} r={r} /> : null}
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
      <path d="M12 30 h56 v36 h-56z" fill={wall} stroke="#8a6a44" strokeWidth={1.5} strokeLinejoin="round" />
      <path d="M12 42 h56 M12 54 h56" stroke="#8a6a44" strokeWidth={1} opacity={0.45} />
      <path d="M26 66 v-20 q14 -13 28 0 v20z" fill="#2a211c" stroke="#5a4634" strokeWidth={1.5} />
      <path d="M4 31 L40 5 L76 31 Z" fill="#c4553f" stroke="#7d2f27" strokeWidth={1.5} strokeLinejoin="round" />
      <path d="M22 18 l18 -13 l18 13" stroke="#e07a62" strokeWidth={2} fill="none" strokeLinecap="round" opacity={0.7} />
      <path d="M2 32 h76" stroke="#7d2f27" strokeWidth={3} strokeLinecap="round" />
      <circle cx={40} cy={22} r={4} fill="#fff6d6" stroke="#7d2f27" strokeWidth={1.2} />
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
