# OpenChore website

The promo site published to GitHub Pages by `.github/workflows/pages.yml`.
Plain HTML and CSS, no build step. Screenshots are not duplicated here: the
workflow copies `docs/screenshots/` into `site/screenshots/` when it deploys.

Preview locally with `make site` (copies the screenshots, then serves on
http://localhost:8000).

Fonts are Onest and Fraunces (SIL Open Font License, see `fonts/`), served
from this site rather than a font CDN.
