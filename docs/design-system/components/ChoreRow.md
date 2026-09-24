One chore: icon well, title, meta line and a check on the right. The same anatomy in every skin.

- **Props:**
  - `cat` (`essential`, `daily` or `bonus`), `icon` (a name from the Icon set), `title` and `meta` (a time window, a countdown or a status line).
  - `points`, plus flags: `urgent` sets the meta in `urgent`, `photo` adds a camera, `readAloud` adds a speaker button for young readers.
  - `state`: `todo`, `done`, `waiting` or `locked`.
- **States:**
  - `done` shows a filled check and muted text. Blocks also strikes the title through and presses the row flat.
  - `waiting` shows a dashed ring and the waiting line; the default is "Waiting for a grown-up".
  - `locked` shows a dashed outline, a lock and "Opens when everything else is done". Only Bonus locks.
- **Fills:** each skin gives rows its own fill: raised white cards in Sunroom, category-coloured blocks in Blocks, flat rows on hairlines in Tint. The consumer provides only data.
- Stack rows directly under a `CategoryHeader`. The gap between rows is a dial.
