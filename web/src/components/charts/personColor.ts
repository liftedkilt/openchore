import { isPersonColor } from '../../design/types';

/**
 * A CSS colour for a person's series: their `--person-<key>` token, which
 * resolves to the right value in whatever theme the chart sits in. People
 * without a colour fall back to muted ink.
 */
export function personColorVar(color: string | null | undefined): string {
  return isPersonColor(color) ? `var(--person-${color})` : 'var(--ink-muted)';
}

/**
 * Spread label positions so none sit closer than `gap`, keeping them between
 * `min` and `max`. Returns the new positions in the input order.
 */
export function spreadLabels(ys: number[], gap: number, min: number, max: number): number[] {
  const order = ys.map((y, i) => ({ y, i })).sort((a, b) => a.y - b.y);
  const out = order.map((o) => Math.max(min, o.y));
  for (let k = 1; k < out.length; k++) out[k] = Math.max(out[k], out[k - 1] + gap);
  if (out.length && out[out.length - 1] > max) {
    out[out.length - 1] = max;
    for (let k = out.length - 2; k >= 0; k--) out[k] = Math.min(out[k], out[k + 1] - gap);
  }
  const result = new Array<number>(ys.length);
  order.forEach((o, k) => { result[o.i] = out[k]; });
  return result;
}
