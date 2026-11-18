// "Design an accessory": draw on a small transparent canvas (or start from a
// picture), choose where it's worn, see it on your animal, then redeem.
import { useCallback, useEffect, useRef, useState } from 'react';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import { PET_ITEM_SLOTS, type Pet, type PetItemSlot, type Reward } from '../../../types';
import { Sheet } from '../Sheet';
import { FarmAnimal, petPicture } from './FarmAnimal';
import k from '../kid.module.css';
import s from './petSheet.module.css';

const SIZE = 256;
const COLORS = ['#1d1b1a', '#ffffff', '#e2453c', '#f39c32', '#ffd166', '#5fbf5a', '#3c8fe0', '#8a5ae0', '#f59ac0', '#8a5a33'];
const BRUSHES = [6, 12, 24];

/**
 * The drawing trimmed to its own edges (plus a little padding), as a PNG
 * data URL, so it fills the accessory spot instead of sitting small in the
 * middle of an empty canvas. Returns '' when nothing is drawn.
 */
function trimmed(c: HTMLCanvasElement): string {
  const data = c.getContext('2d')?.getImageData(0, 0, SIZE, SIZE).data;
  if (!data) return '';
  let x0 = SIZE, y0 = SIZE, x1 = -1, y1 = -1;
  for (let y = 0; y < SIZE; y++) {
    for (let x = 0; x < SIZE; x++) {
      if (data[(y * SIZE + x) * 4 + 3] > 8) {
        if (x < x0) x0 = x;
        if (x > x1) x1 = x;
        if (y < y0) y0 = y;
        if (y > y1) y1 = y;
      }
    }
  }
  if (x1 < 0) return '';
  const pad = 4;
  x0 = Math.max(0, x0 - pad); y0 = Math.max(0, y0 - pad);
  x1 = Math.min(SIZE - 1, x1 + pad); y1 = Math.min(SIZE - 1, y1 + pad);
  const out = document.createElement('canvas');
  out.width = x1 - x0 + 1;
  out.height = y1 - y0 + 1;
  out.getContext('2d')?.drawImage(c, x0, y0, out.width, out.height, 0, 0, out.width, out.height);
  return out.toDataURL('image/png');
}

interface DesignSheetProps {
  pet: Pet;
  reward: Reward;
  busy?: boolean;
  onConfirm: (pick: { slot: PetItemSlot; image: string }) => void;
  onClose: () => void;
}

export function DesignSheet({ pet, reward, busy, onConfirm, onClose }: DesignSheetProps) {
  const { t } = useTranslation();
  const canvas = useRef<HTMLCanvasElement>(null);
  const drawing = useRef<{ x: number; y: number } | null>(null);
  const history = useRef<ImageData[]>([]);
  const [color, setColor] = useState(COLORS[2]);
  const [brush, setBrush] = useState(BRUSHES[1]);
  const [erase, setErase] = useState(false);
  const [slot, setSlot] = useState<PetItemSlot>('head');
  const [preview, setPreview] = useState('');
  const [empty, setEmpty] = useState(true);

  const ctx = () => canvas.current?.getContext('2d') ?? null;
  const refresh = useCallback(() => {
    const c = canvas.current;
    if (!c) return;
    const url = trimmed(c);
    setPreview(url);
    setEmpty(!url);
  }, []);
  useEffect(refresh, [refresh]);

  const snapshot = () => {
    const c = ctx();
    if (c) history.current = [...history.current.slice(-19), c.getImageData(0, 0, SIZE, SIZE)];
  };

  const point = (e: React.PointerEvent<HTMLCanvasElement>) => {
    const r = e.currentTarget.getBoundingClientRect();
    return { x: ((e.clientX - r.left) / r.width) * SIZE, y: ((e.clientY - r.top) / r.height) * SIZE };
  };

  const stroke = (from: { x: number; y: number }, to: { x: number; y: number }) => {
    const c = ctx();
    if (!c) return;
    c.globalCompositeOperation = erase ? 'destination-out' : 'source-over';
    c.strokeStyle = color;
    c.lineWidth = brush;
    c.lineCap = 'round';
    c.lineJoin = 'round';
    c.beginPath();
    c.moveTo(from.x, from.y);
    c.lineTo(to.x + 0.01, to.y);
    c.stroke();
  };

  const down = (e: React.PointerEvent<HTMLCanvasElement>) => {
    e.currentTarget.setPointerCapture(e.pointerId);
    snapshot();
    const p = point(e);
    drawing.current = p;
    stroke(p, p);
  };
  const move = (e: React.PointerEvent<HTMLCanvasElement>) => {
    if (!drawing.current) return;
    const p = point(e);
    stroke(drawing.current, p);
    drawing.current = p;
  };
  const up = () => {
    if (!drawing.current) return;
    drawing.current = null;
    refresh();
  };

  const undo = () => {
    const c = ctx();
    const last = history.current.pop();
    if (c && last) {
      c.putImageData(last, 0, 0);
      refresh();
    }
  };
  const clear = () => {
    snapshot();
    ctx()?.clearRect(0, 0, SIZE, SIZE);
    refresh();
  };

  // Start from a picture: fit it into the canvas.
  const upload = (file?: File) => {
    if (!file || !file.type.startsWith('image/')) return;
    const url = URL.createObjectURL(file);
    const img = new Image();
    img.onload = () => {
      const c = ctx();
      if (c) {
        snapshot();
        c.globalCompositeOperation = 'source-over';
        c.clearRect(0, 0, SIZE, SIZE);
        const scale = Math.min(SIZE / img.width, SIZE / img.height);
        const w = img.width * scale;
        const h = img.height * scale;
        c.drawImage(img, (SIZE - w) / 2, (SIZE - h) / 2, w, h);
        refresh();
      }
      URL.revokeObjectURL(url);
    };
    img.src = url;
  };

  return (
    <Sheet title={t('kid.farm.design.title')} onClose={onClose} className={s.sheet}>
      <p className={s.hint}>{t('kid.farm.design.lead')}</p>
      <div className={s.designRow}>
        <canvas
          ref={canvas}
          width={SIZE}
          height={SIZE}
          className={s.designCanvas}
          onPointerDown={down}
          onPointerMove={move}
          onPointerUp={up}
          onPointerCancel={up}
          aria-label={t('kid.farm.design.canvas')}
        />
        <div className={s.designSide}>
          <FarmAnimal
            species={pet.species}
            color={pet.color}
            mood="happy"
            accessory={empty ? pet.accessory : `img:${slot}:preview`}
            accessoryImageUrl={empty ? undefined : preview}
            {...petPicture(pet)}
            className={s.pickArt}
          />
          <div className={k.ownChips} role="radiogroup" aria-label={t('kid.farm.design.where')}>
            {PET_ITEM_SLOTS.map(sl => (
              <button key={sl} type="button" role="radio" aria-checked={slot === sl}
                className={clsx(s.chip, slot === sl && s.chipOn)} onClick={() => setSlot(sl)}>
                {t(`kid.farm.design.slot.${sl}`)}
              </button>
            ))}
          </div>
        </div>
      </div>

      <div className={s.designTools}>
        <div className={s.swatches} role="radiogroup" aria-label={t('kid.farm.design.colour')}>
          {COLORS.map(c => (
            <button key={c} type="button" role="radio" aria-checked={!erase && color === c} aria-label={c}
              className={clsx(s.swatch, !erase && color === c && s.swatchOn)} style={{ background: c }}
              onClick={() => { setColor(c); setErase(false); }} />
          ))}
        </div>
        <div className={k.ownChips}>
          {BRUSHES.map(b => (
            <button key={b} type="button" aria-pressed={brush === b}
              className={clsx(s.chip, brush === b && s.chipOn)} onClick={() => setBrush(b)}
              aria-label={t('kid.farm.design.brush', { size: b })}>
              <span className={s.brushDot} style={{ width: b / 1.5 + 4, height: b / 1.5 + 4 }} />
            </button>
          ))}
          <button type="button" aria-pressed={erase} className={clsx(s.chip, erase && s.chipOn)} onClick={() => setErase(!erase)}>
            {t('kid.farm.design.eraser')}
          </button>
          <button type="button" className={clsx(s.chip)} onClick={undo}>{t('kid.farm.design.undo')}</button>
          <button type="button" className={clsx(s.chip)} onClick={clear}>{t('kid.farm.design.clear')}</button>
          <label className={clsx(s.chip, s.uploadBtn)}>
            {t('kid.farm.design.upload')}
            <input type="file" accept="image/png,image/jpeg,image/gif" hidden onChange={e => upload(e.target.files?.[0])} />
          </label>
        </div>
      </div>

      <div className={k.sheetActions}>
        <button type="button" className={k.primaryAction} disabled={empty || busy}
          onClick={() => onConfirm({ slot, image: preview })}>
          {t('kid.farm.pick.confirm', { count: reward.effective_cost })}
        </button>
      </div>
    </Sheet>
  );
}
