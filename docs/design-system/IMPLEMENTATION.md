# Implementing the redesign

This folder is the reference for the OpenChore redesign. It is exported from the design-system artifact:

- `README.md` is the brand book: skins, House, voice, foundations.
- `tokens.json` holds every colour and shadow per theme, plus spacing, radii and type.
- `reference-components.css` holds the theme **dials** and the component styles.
- `reference-components.js` holds the reference React components (plain `createElement`, icons inlined).
- `components/*.md` are the per-component guidelines.
- `screens/*.png` are rendered mockups. `screens/*.html` are their sources.
- `icons.svg.html` is the line icon set as SVG `<symbol>`s.

The reference code is a spec, not production code. Port it to typed React + CSS in `web/src/design/`. Match the rendered screens, and keep the rules in the brand book.

## Decisions

1. **Skins live in `users.theme`.** The values are `sunroom`, `blocks` and `tint`. Migration `017` maps the old values: `default`→`sunroom`, `quest`→`blocks`, `galaxy`→`tint`, `forest`→`sunroom`.
   - The API accepts only the new values.
   - The config seeder also accepts the legacy names and maps them.
   - An empty theme resolves on the client by age: under 8 → `blocks`, otherwise `sunroom`.
2. **Each person has a colour in `users.color`.** It is a key, not a hex value.
   - **Palette:** the design has four colours (`coral`, `mint`, `butter`, `sky`). The implementation extends this to eight, adding `rose`, `leaf`, `lilac` and `sand`, so that big families don't repeat colours.
   - **Values:** each colour needs a value per theme (`house`, `house-dark`, `sunroom`, `blocks`, `tint`). The initial on each must hold 4.5:1 against `on-person`.
   - **Existing users:** migration `017` assigns colours by nearest match to their old `line_color`, otherwise round-robin by id.
   - **Endpoints:** `PUT /api/users/{id}/color` lets a user set their own colour. Admins set it through `PUT /api/users/{id}`.
   - `line_color` stays in the API for compatibility. The UI stops using it; charts use the person colour.
3. **Themes are data attributes.** Write `data-theme` and `data-person` together on a person's own screen root (see `SkinScope`). Shared screens (picker, ambient, admin, reports) use `HouseScope`, which picks `house` or `house-dark`:
   - Personal (persistent / OIDC) sessions follow `prefers-color-scheme`.
   - Shared-device sessions and the ambient display follow a schedule: dark from 19:30 to 07:00.
4. **Components never test which theme they are in.** No `[data-theme="x"] .thing` selectors. Every difference between themes is a dial (a CSS custom property set on `[data-theme]`) or a token. This is what lets several skins render side by side (the picker's doors).
5. **Fonts are self-hosted** through `@fontsource-variable/*` so the PWA works offline: Fraunces (with the SOFT axis), Figtree, Bricolage Grotesque, Instrument Sans and Onest.
6. **Icons:** the line set in `icons.svg.html` becomes an `Icon` component. Chore icons in the database are free text, mostly emoji.
   - A known icon name renders the line icon.
   - A known emoji maps to a line icon (🐱→`paw`, 🛏️→`bed`, 🪥→`tooth`, …).
   - Anything else falls back to a generic icon for its category.
7. **Words:** the categories are **Must do / Every day / Bonus** in every skin. The per-theme labels and greetings in `THEME_CONFIG` go away. Sounds and celebration decoration may still vary by skin. Every new string goes through i18n, in both `en` and `de`.
8. **Business rules are unchanged.** Every points change still writes `point_transactions`. Bonus still only pays out when every Must do and Every day chore is complete. Parents still take part.

## Phases

- **Phase 1 (in parallel):**
  - **Backend:** migration, model, store, API, seed data, Go tests, and the `types.ts` / `api.ts` additions.
  - **Frontend foundation:** `web/src/design/` with tokens, dials, fonts, icons, components, scopes and a dev-only `/design` gallery.
- **Phase 2 (in parallel, after phase 1 merges):**
  - **Shared entry:** the picker and PIN pad in House, plus the setup wizard's skin and colour choice.
  - **Kid screens:** Today, Week, Rewards, the celebration, and photo upload, in the person's skin.
  - **Shared and parent screens:** the ambient wall display, admin and reports, in House / House Dark.

## Rules for every change

- Run `go test ./...`, `cd web && npx tsc --noEmit && npx vitest run && npm run build`.
- e2e (Playwright) uses fixed ports 8080/5173 and is run centrally after merging. Keep the hooks the specs rely on, or update the specs in the same change. The hooks are class names containing `choreCard`, `aria-label="Mark complete"` / `"Mark incomplete"`, and visible text such as `pts`. Never skip or delete a test.
- Contrast: text 4.5:1 and meaningful graphics 3:1 in every theme. Check new colour values with a script, not by eye.
