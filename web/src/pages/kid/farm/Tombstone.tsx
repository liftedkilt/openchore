// A grave on the farm: where an animal that stayed sick too long is
// remembered (pet deaths, docs/pets.md). The animal's name is carved on the
// stone; the art reads the same --farm-* night filter as the rest of the
// field (see .graves in farm.module.css).
import s from './farm.module.css';

export function Tombstone({ name, className }: { name: string; className?: string }) {
  return (
    <span className={`${s.tombstone} ${className ?? ''}`}>
      <svg viewBox="0 0 60 70" className={s.tombstoneArt} aria-hidden="true">
        <ellipse cx={30} cy={66} rx={27} ry={3.5} fill="#1d1b1a" opacity={0.16} />
        {/* A mound of earth with a tuft of grass and a flower in front. */}
        <path d="M5 66 q25 -9 50 0z" fill="#7a5a3c" stroke="#5a4634" strokeWidth={1.2} strokeLinejoin="round" />
        <path d="M10 62 V22 a20 20 0 0 1 40 0 V62z" fill="#b9bcc0" stroke="#6d7177" strokeWidth={1.6} strokeLinejoin="round" />
        <path d="M14 60 V23 a16 16 0 0 1 32 0" fill="none" stroke="#d9dcdf" strokeWidth={1.4} opacity={0.8} />
        <path d="M18 9 q-2 6 1 10" stroke="#9a9ea4" strokeWidth={1} fill="none" opacity={0.7} />
        <path d="M8 64 q2 -6 4 0 M47 64 q2 -7 4 0 M50 64 q3 -5 5 0" stroke="#6fa84f" strokeWidth={2} fill="none" strokeLinecap="round" />
        <path d="M44 63 v-6" stroke="#4f8a3a" strokeWidth={1.4} strokeLinecap="round" />
        <circle cx={44} cy={56} r={2.6} fill="#f2c94c" stroke="#b8872a" strokeWidth={0.8} />
      </svg>
      <span className={s.tombstoneText}>
        <span className={s.tombstoneRip}>RIP</span>
        <span className={s.tombstoneName}>{name}</span>
      </span>
    </span>
  );
}

/** A grave's YYYY-MM-DD date written out for the reader's language. */
export function graveDate(day: string, lang: string): string {
  return new Date(`${day}T12:00:00`).toLocaleDateString(lang, { day: 'numeric', month: 'long', year: 'numeric' });
}
