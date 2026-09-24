import { isIconName, type IconName } from './icons';
import type { Cat } from './types';

// Chore and reward icons in the database are free text, mostly emoji.
// A known icon name renders that icon, a known emoji maps to its line icon,
// and anything else falls back to a generic icon for the category.

const EMOJI: Record<string, IconName> = {};
function map(icon: IconName, emoji: string) {
  for (const e of emoji.split(' ')) if (e) EMOJI[normalize(e)] = icon;
}

/** Strip variation selectors and skin-tone modifiers so 🛏 and 🛏️ match. */
function normalize(s: string): string {
  return s.trim().replace(/[\uFE0E\uFE0F\u{1F3FB}-\u{1F3FF}]/gu, '');
}

map('paw', '🐱 🐈 🐈‍⬛ 🐶 🐕 🐩 🦮 🐕‍🦺 🐾 🐹 🐰 🐇 🐭 🐁 🐦 🦜 🐤 🐴 🐎');
map('fish', '🐟 🐠 🐡 🦈 🐢 🐸');
map('bed', '🛏️ 🛌');
map('tooth', '🪥 🦷 😁');
map('dish', '🍽️ 🥄 🧽 🫧 🥛 🫖');
map('table', '🍴 🪑');
map('bowl', '🥣 🍳 🥘 🍲 🍝 🥗');
map('book', '📚 📖 📓 📔 📕 📗 📘 📙 📒 📝 ✏️ ✍️ 🖍️ 📐');
map('shirt', '👕 👚 👖 🧦 🧺 👗 👔 🩳 🧥 👟');
map('sprout', '🌱 🪴 🌿 🌻 🌷 🌸 🌼 🍀 🌳 🌲 💐 🍂 🍃');
map('toy', '🧸 🪀 🧩 🎲 🧱 🪁 🚂 🎨');
map('piano', '🎹 🎵 🎶 🎼 🎸 🎻 🎺 🎷 🥁 🪕');
map('bin', '🗑️ ♻️ 🚮');
map('broom', '🧹 🧼 🪣 🧴 🫙 🪒');
map('bike', '🚲 🛴 🛹');
map('rocket', '🚀');
map('film', '🎬 🎥 🍿 🎞️');
map('screen', '📺 🎮 🕹️ 💻 📱 🖥️');
map('cone', '🍦 🍨 🍧 🍭 🍬 🍫 🍩 🍪 🧁');
map('moon', '🌙 🌛 🌜 😴 💤');
map('gift', '🎁 🎀');
map('star', '⭐ 🌟 🏆 🥇 🏅');
map('flame', '🔥');
map('spark', '✨ 🎉 🎊 💫');
map('clock', '⏰ ⏱️ 🕐 ⌛ ⏳');
map('camera', '📷 📸');
map('home', '🏠 🏡 🚪');
map('cal', '📅 🗓️ 📆');
map('lock', '🔒 🔐');
map('people', '👪 🤝 👨‍👩‍👧 👨‍👩‍👧‍👦 👫 👬 👭 🫂');
map('sound', '🔊 🔉 🎤');
map('check', '✅ ✔️ ☑️');

// One emoji, including ZWJ sequences, variation selectors and skin tones.
const FIRST_EMOJI = /^\p{Extended_Pictographic}[\uFE0E\uFE0F\u{1F3FB}-\u{1F3FF}]*(?:\u200D\p{Extended_Pictographic}[\uFE0E\uFE0F\u{1F3FB}-\u{1F3FF}]*)*/u;

/** The generic icon for each category, used when nothing else matches. */
export const CATEGORY_ICON: Record<Cat, IconName> = {
  essential: 'home',
  daily: 'cal',
  bonus: 'spark',
};

/**
 * Resolve a DB chore/reward `icon` string to a line icon name.
 * Known icon names pass through (case-insensitive), known emoji map to a line
 * icon, and anything else falls back to the category's generic icon.
 */
export function resolveChoreIcon(icon: string | null | undefined, cat: Cat = 'daily'): IconName {
  const raw = (icon ?? '').trim();
  if (raw) {
    const lower = raw.toLowerCase();
    if (isIconName(lower)) return lower;
    const hit = EMOJI[normalize(raw)];
    if (hit) return hit;
    // Several emoji or an emoji with trailing text: try the first grapheme.
    const first = FIRST_EMOJI.exec(raw);
    if (first) {
      const firstHit = EMOJI[normalize(first[0])];
      if (firstHit) return firstHit;
    }
  }
  return CATEGORY_ICON[cat] ?? CATEGORY_ICON.daily;
}
