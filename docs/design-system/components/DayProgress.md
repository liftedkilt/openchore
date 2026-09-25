The hero at the top of Today. It shows how much of the day is done, and each skin draws it its own way.

- **Props:** `items`, one per chore for the day: `{cat, done, at}`, where `at` is when a done chore was finished, as 0–1 across the day. Also `now` (0–1) and the `from` / `to` labels ("7 am", "9 pm").
- **Sunroom:** a sun arc from `from` to `to`. The sun sits at `now`, with a pin for each finished chore and "6 of 8" inside.
- **Blocks:** a row of category shapes that fill in as chores get done.
- **Tint:** one ring in the person's colour, with "6/8" in the middle.
- All three variants render, and the `--show-arc`, `--show-shapes` and `--show-ring` dials pick one. The written count is always present, so the graphic never carries the meaning alone.
