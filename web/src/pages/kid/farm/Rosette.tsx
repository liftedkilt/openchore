// The winner's ribbon: a small rosette for whoever came first on last week's
// leaderboard, shown by their name on the board and on their animal's tag.
export function Rosette({ className, title }: { className?: string; title?: string }) {
  return (
    <svg viewBox="0 0 20 24" className={className} role={title ? 'img' : undefined} aria-label={title} aria-hidden={title ? undefined : true}>
      <path d="M6 13 L3 23 L8 20 L10 24 L12 20 L17 23 L14 13z" fill="#d6463a" stroke="#7d2f27" strokeWidth={1} strokeLinejoin="round" />
      <circle cx={10} cy={9} r={8} fill="#f6c443" stroke="#9a6b12" strokeWidth={1.2} />
      <circle cx={10} cy={9} r={4.6} fill="#fff3c4" stroke="#9a6b12" strokeWidth={0.8} />
      <path d="M10 6.2 l0.9 1.9 2 0.25 -1.5 1.4 0.4 2 -1.8 -1 -1.8 1 0.4 -2 -1.5 -1.4 2 -0.25z" fill="#d99a1c" />
    </svg>
  );
}
