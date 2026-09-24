OpenChore is one family app that each person can wear in their own skin. There are three skins: **Sunroom** (warm, soft, editorial), **Blocks** (bold, graphic, tactile) and **Tint** (calm, dark, lit in your own colour). They are the three themes of this system. Every screen is built once from the same components, and a skin only changes how it looks. A fourth theme, **House** (with **House Dark**), is nobody's skin. It is the neutral frame for screens the whole family shares. Use the theme switcher on this page to see every token and card in each theme.

## The rule that keeps it one app

A skin may change **how things look**. It may never change **what things are, where they sit or how they work**.

| Always the same, in every skin | A skin may change |
| --- | --- |
| The three tabs: Today, Week, Rewards | Colour: every colour token |
| Chore row anatomy: icon well, title, meta line, check on the right | Typefaces, display weight, italics |
| One tap on the check finishes a chore | Corner radii, outlines, shadows ("lift") |
| Category names and shapes: **Must do** ● · **Every day** ■ · **Bonus** ★ | The hero on Today: sun arc, shape row or ring (`DayProgress`) |
| Voice and copy, word for word | The celebration's decoration (`Celebration`) |
| Icons: one line set | Icon stroke weight |
| Locked bonus, waiting-for-a-grown-up and done states | Whether Tint's person colour leads the screen |

Two things carry across every skin:

1. **Each person's colour.** Every person picks one of `person-coral`, `person-mint`, `person-butter` or `person-sky` (the implementation adds `person-rose`, `person-leaf`, `person-lilac` and `person-sand` for big families). It is their avatar fill everywhere and their bar in every family view. On a Tint screen that belongs to them, it also becomes the accent, the progress ring and the done check. Each skin tunes the colour's lightness: pastel in Sunroom, saturated in Blocks, bright on dark in Tint. The hue stays the same, so Lily is mint wherever she appears.
2. **The category shapes.** Must do is a circle, Every day a rounded square and Bonus a star, in every skin. House and Tint keep the category colours monochrome on purpose, so the shape alone has to carry the meaning. That also keeps categories readable for colour-blind kids.

## House: the shared frame

Any screen that shows more than one person uses **House**. That covers the family picker, the family week, the wall display and every grown-up screen. House takes a little from each skin and none of their signatures, so no skin reads as the default:

- **Neutral ground:** `surface` is a warm grey between Sunroom's cream and Blocks' white, with soft `lift` and no outlines.
- **One neutral typeface:** Onest, a friendly grotesk that belongs to none of the three skins.
- **People are the only colour.** Categories are ink, and the accent is ink. The four person colours carry everything, including the logo mark: four overlapping dots, one per colour.
- **Each person appears in their own skin.** On the family picker, every person gets a "door" of the same size, radius and layout (avatar, name, what's left, their day). Each door is drawn in that person's skin, and their own `DayProgress` hero shows inside it. The frame is neutral, so each door previews where that person's screen will take them. See **Screens → FamilyPicker**.
- **Mechanics:** a door is just an element with `data-theme` and `data-person` inside the House frame. Components never test the theme, so every door renders correctly next to the others.

Once someone taps their door, the app opens in their skin.

### House Dark

House is the only theme with a dark mode, because it is the only one that runs on devices nobody chose a skin for: parents' phones, the shared tablet and the wall display. Each skin keeps one mode. A kid who wants a dark app picks Tint.

- **When it's on:** personal devices follow their system light/dark setting. Shared devices follow a schedule (the mockup switches at 7:30 pm) so the hallway tablet doesn't glow at bedtime.
- **What changes:** only the token values. House Dark shares every House dial and component. `surface` becomes a warm charcoal (`#1a1917`), deliberately warmer than Tint's blue-black, so a Tint door still reads as Tint. Ink and accent become a warm off-white.
- **What stays:** the four person colours keep their House values, which are bright enough for both grounds, so everyone looks the same by day and by night.
- **Doors at night:** a skin's door inside House Dark is dimmed (brightness 86%, saturation 90%) so a cream Sunroom or white Blocks door doesn't glare. This is the first piece of the planned evening adjustment for skins.
- Every text pair holds 4.5:1 in House Dark, and checks and outlines hold 3:1. See **Screens → FamilyPicker** for day and night side by side.

## Choosing a skin

- A kid picks their skin in their own settings. A parent can set or lock it. No skin is the default. A new profile picks one at setup, with suggestions by age: Blocks under 8, Sunroom or Tint from 8. Graduating to a new skin can be a small moment, not a hidden setting. House is never offered as a personal skin.
- The skin is stored per profile, replacing today's `theme` field (`default`, `quest`, `galaxy`, `forest`).

## How a theme is built

- **Tokens** (`tokens.json`) hold every colour and shadow per theme under shared names: `surface`, `ink`, `accent`, `row-essential`, `lift`, and so on. Components only ever use these names.
- **Dials** (the `:root` / `[data-theme]` blocks at the top of `components/bundle.css`) hold what a token file can't: font stacks, display weight and italics, outline width, radii, check size, and which hero and celebration decoration show. A new skin is a new token column plus one dial block.
- **Components never test which theme they are in.** No selector reads `[data-theme="x"] .thing`. That's what lets two skins sit side by side on one page, as in the cards here.
- **Put `data-theme` and `data-person` on the same screen root** for a person's own screen. That pair is what lets Tint's person colour lead: `[data-theme="tint"][data-person]` re-points `accent`, `highlight` and `done` to `--you`.

## Voice and content

- Talk to the kid as "you". Use their first name once per screen, in the greeting ("Afternoon, *Lily*.").
- Use sentence case, except for the uppercase category headers (`label`).
- Keep chore titles short and start them with a verb: "Feed the cats", "Tidy your room", "Water the garden".
- Give times as a window or a countdown ("3:00 – 5:00 pm", "44 min left", "Before 9:00"). Never make a kid work out a deadline.
- Praise the work in one line ("Nailed it!", "Looks great, Lily").
- Explain every lock: "Opens when everything else is done", "Waiting for Mom to check", "35 more".
- Say "points" in full, or "pts" in a meta line. A balance is a number beside a star.
- No emoji in the interface. Chores get an icon from the line set.

## Visual foundations

- **Spacing:** a 4px base. Screen side margins are `space-5`, card padding `space-4`, section gaps `space-5`, and devices on a canvas sit `space-8` apart.
- **Type:** one shared scale (`hero`, `numeral`, `title`, `body`, `meta`, `label`), with faces set by the dials:
  - House: Onest throughout.
  - Sunroom: Fraunces with the SOFT axis for display, and Figtree for text.
  - Blocks: Bricolage Grotesque, heavy.
  - Tint: Instrument Sans.
  - All faces come from Google Fonts.
- **Surfaces:** pages sit on `surface`, cards and rows on the row tokens, and tracks and empty tiles on `surface-sunk`. `lift` gives a soft shadow in House and Sunroom, a hard 4px ink offset in Blocks and nothing in Tint.
- **Outlines:** only Blocks draws outlines, at 2.5px in `line`. In Blocks, a chore that is still to do stands up on its shadow. When it's done it is pressed flat: it moves down and right by the shadow's offset and the shadow goes.
- **Accessibility:**
  - Every text pair in the token notes holds 4.5:1 in all three themes. Checks, rings and outlines hold 3:1.
  - Two graphic-only exceptions are noted on their tokens: Sunroom's `highlight` arc (always beside a written count) and Blocks' `done` mint (always inside an ink outline).
  - Status never relies on colour. Done has a tick and muted text, waiting a dashed ring and words, locked a lock icon and a sentence.
  - Targets are at least 44px.
- **Motion:** not specified yet. Planned: a press-down on the check, the celebration flowing in from the check's position, and Sunroom's sun sliding along the arc as the day passes.

## Iconography

One custom line set on a 24px grid with round caps and joins, drawn inline from the bundle (`Icon`, see the Icon card). The stroke is set by the `--icon-stroke` dial: 1.75 in House, Sunroom and Tint, and 2 in Blocks. It covers chore objects (bed, tooth, paw, dish, book, shirt, sprout, toy, fish, piano, table), reward objects (bike, rocket, film, bowl, cone, screen, moon), UI (home, calendar, gift, check, lock, clock, camera, sound, chevrons, plus, people) and the flame, star and spark. It replaces the emoji the current app uses for chore icons.
