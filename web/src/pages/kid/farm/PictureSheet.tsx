// "Use my own picture": pick a ready-made starter-pack animal or choose a PNG
// to show in place of the drawn animal, see it on the grass, then redeem. It
// can be one picture, or a strip of frames (stand, walk, walk, sick, lying
// down, happy) for a real walk and mood pictures. The starter kit (blank
// canvases, guides and examples) is linked here so one can be made in any art
// program. See docs/custom-animals.md.
import { useRef, useState } from 'react';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import type { Pet, PetMood, Reward } from '../../../types';
import { Sheet } from '../Sheet';
import { FarmAnimal, spriteFrameCount } from './FarmAnimal';
import { STARTER_PACK, type StarterPackAnimal, type StarterPackId } from './starterPack';
import k from '../kid.module.css';
import f from './farm.module.css';
import s from './petSheet.module.css';

const KIT_ONE = [
  ['kitBlank', '/sprites/blank-64.png'],
  ['kitLayer', '/sprites/guide-layer-64.png'],
  ['kitGuide', '/sprites/guide.png'],
  ['kitSample', '/sprites/sample-duck.png'],
] as const;
const KIT_STRIP = [
  ['kitBlank', '/sprites/blank-strip.png'],
  ['kitLayer', '/sprites/guide-layer-strip.png'],
  ['kitGuide', '/sprites/guide-strip.png'],
  ['kitSample', '/sprites/sample-duck-strip.png'],
] as const;

/** The moods a strip can have its own frame for, with the frame count that includes it. */
const MOOD_FRAMES: [PetMood, number][] = [['sick', 4], ['very_sick', 5], ['happy', 6]];

function blobToPngDataUrl(blob: Blob): Promise<string> {
  const pngBlob = blob.type === 'image/png' ? blob : new Blob([blob], { type: 'image/png' });
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      const url = typeof reader.result === 'string' ? reader.result : '';
      if (!url.startsWith('data:image/png;base64,')) {
        reject(new Error('not png data url'));
        return;
      }
      resolve(url);
    };
    reader.onerror = () => reject(new Error('read failed'));
    reader.readAsDataURL(pngBlob);
  });
}

interface PictureSheetProps {
  pet: Pet;
  reward: Reward;
  busy?: boolean;
  onConfirm: (pick: { image: string }) => void;
  onClose: () => void;
}

export function PictureSheet({ pet, reward, busy, onConfirm, onClose }: PictureSheetProps) {
  const { t } = useTranslation();
  const [image, setImage] = useState('');
  const [frames, setFrames] = useState(1);
  const [error, setError] = useState('');
  const [selectedPack, setSelectedPack] = useState<StarterPackId | null>(null);
  const [loadingPack, setLoadingPack] = useState<StarterPackId | null>(null);
  const reqSeq = useRef(0);

  const choosePack = async (animal: StarterPackAnimal) => {
    const seq = ++reqSeq.current;
    setLoadingPack(animal.id);
    setError('');
    try {
      const resp = await fetch(animal.stripUrl);
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      const blob = await resp.blob();
      const dataUrl = await blobToPngDataUrl(blob);
      if (seq !== reqSeq.current) return;
      setSelectedPack(animal.id);
      setFrames(animal.frames);
      setImage(dataUrl);
    } catch {
      if (seq !== reqSeq.current) return;
      setSelectedPack(null);
      setImage('');
      setError(t('kid.farm.picture.packError'));
    } finally {
      if (seq === reqSeq.current) setLoadingPack(null);
    }
  };

  const choose = (file?: File) => {
    if (!file) return;
    const seq = ++reqSeq.current;
    setSelectedPack(null);
    setLoadingPack(null);
    if (file.type !== 'image/png') {
      setImage('');
      setError(t('kid.farm.picture.notPng'));
      return;
    }
    const reader = new FileReader();
    reader.onload = () => {
      if (seq !== reqSeq.current) return;
      const url = typeof reader.result === 'string' ? reader.result : '';
      const probe = new Image();
      probe.onload = () => {
        if (seq !== reqSeq.current) return;
        setError('');
        setFrames(spriteFrameCount(probe.naturalWidth, probe.naturalHeight));
        setImage(url);
      };
      probe.onerror = () => {
        if (seq !== reqSeq.current) return;
        setImage('');
        setError(t('kid.farm.picture.notPng'));
      };
      probe.src = url;
    };
    reader.readAsDataURL(file);
  };

  // What's on show: the chosen picture, or the one already in use.
  const sprite = image || pet.sprite;
  const count = image ? frames : pet.sprite_frames ?? 1;
  const look = { species: pet.species, color: pet.color, sprite, spritePreview: !!image, spriteFrames: count };

  return (
    <Sheet title={t('kid.farm.picture.title')} onClose={onClose} className={s.sheet}>
      <p className={k.helpLine}>{t('kid.farm.picture.lead')}</p>

      <div className={s.pickPreview}>
        {/* Walking, so a strip's walk frames can be seen. */}
        <div className={clsx(s.pictureGrass, sprite && count > 1 && f.walking)} aria-label={t('kid.farm.picture.preview')}>
          <FarmAnimal {...look} mood="content" accessory={pet.accessory} className={s.pickArt} />
        </div>
        {image && (
          <p className={k.helpLine}>
            {count > 1 ? t('kid.farm.picture.frames', { count }) : t('kid.farm.picture.oneFrame')}
          </p>
        )}
        {sprite && count > 3 && (
          <div className={s.pictureMoods}>
            {MOOD_FRAMES.filter(([, need]) => count >= need).map(([mood]) => (
              <figure key={mood}>
                <FarmAnimal {...look} mood={mood} />
                <figcaption>{t(`kid.farm.picture.mood.${mood}`)}</figcaption>
              </figure>
            ))}
          </div>
        )}
        {error && <p className={k.helpLine} role="alert">{error}</p>}
      </div>

      <h3 className={s.subheading}>{t('kid.farm.picture.packTitle')}</h3>
      <p className={k.helpLine}>{t('kid.farm.picture.packLead')}</p>
      <div className={s.species} role="radiogroup" aria-label={t('kid.farm.picture.packTitle')}>
        {STARTER_PACK.map(animal => (
          <button
            key={animal.id}
            type="button"
            role="radio"
            aria-checked={selectedPack === animal.id}
            aria-busy={loadingPack === animal.id || undefined}
            disabled={busy}
            className={clsx(s.speciesBtn, selectedPack === animal.id && s.speciesOn)}
            onClick={() => { void choosePack(animal); }}
          >
            <svg viewBox="0.006 0 0.988 1" className={s.packArt} aria-hidden="true">
              <image
                href={animal.stripUrl}
                x={0}
                y={0}
                width={animal.frames}
                height={1}
                preserveAspectRatio="none"
                style={{ imageRendering: 'pixelated' }}
              />
            </svg>
            <span>{t(animal.labelKey)}</span>
          </button>
        ))}
      </div>

      <h3 className={s.subheading}>{t('kid.farm.picture.uploadTitle')}</h3>
      <div className={s.pictureUpload}>
        <label className={clsx(s.chip, s.uploadBtn)}>
          {image && !selectedPack ? t('kid.farm.picture.change') : t('kid.farm.picture.choose')}
          <input type="file" accept="image/png" hidden onChange={e => choose(e.target.files?.[0])} />
        </label>
      </div>

      <p className={k.helpLine}>{t('kid.farm.picture.rules')}</p>
      {([['kitOne', KIT_ONE], ['kitStrip', KIT_STRIP]] as const).map(([title, kit]) => (
        <div key={title} className={s.pictureKit}>
          <p className={k.helpLine}>{t(`kid.farm.picture.${title}`)}</p>
          <div className={k.ownChips}>
            {kit.map(([key, href]) => (
              <a key={href} className={s.chip} href={href} download>{t(`kid.farm.picture.${key}`)}</a>
            ))}
          </div>
        </div>
      ))}

      <div className={k.sheetActions}>
        <button
          type="button"
          className={k.primaryAction}
          disabled={!image || busy || !!loadingPack}
          onClick={() => onConfirm({ image })}
        >
          {t('kid.farm.pick.confirm', { count: reward.effective_cost })}
        </button>
      </div>
    </Sheet>
  );
}
