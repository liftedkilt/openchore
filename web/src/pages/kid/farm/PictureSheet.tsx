// "Use my own picture": choose a PNG to show in place of the drawn animal,
// see it on the grass, then redeem. It can be one picture, or a strip of
// frames (stand, walk, walk, sick, lying down, happy) for a real walk and
// mood pictures. The starter kit (blank canvases, guides and examples) is
// linked here so one can be made in any art program.
// See docs/custom-animals.md.
import { useState } from 'react';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import type { Pet, PetMood, Reward } from '../../../types';
import { Sheet } from '../Sheet';
import { FarmAnimal, spriteFrameCount } from './FarmAnimal';
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

  const choose = (file?: File) => {
    if (!file) return;
    if (file.type !== 'image/png') {
      setImage('');
      setError(t('kid.farm.picture.notPng'));
      return;
    }
    const reader = new FileReader();
    reader.onload = () => {
      const url = typeof reader.result === 'string' ? reader.result : '';
      const probe = new Image();
      probe.onload = () => {
        setError('');
        setFrames(spriteFrameCount(probe.naturalWidth, probe.naturalHeight));
        setImage(url);
      };
      probe.onerror = () => { setImage(''); setError(t('kid.farm.picture.notPng')); };
      probe.src = url;
    };
    reader.readAsDataURL(file);
  };

  // What's on show: the chosen picture, or the one already in use.
  const sprite = image || pet.sprite;
  const count = image ? frames : pet.sprite_frames ?? 1;
  const look = { species: pet.species, color: pet.color, sprite, spritePreview: !!image, spriteFrames: count };

  return (
    <Sheet title={t('kid.farm.picture.title')} onClose={onClose}>
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
        <label className={clsx(s.chip, s.uploadBtn)}>
          {image ? t('kid.farm.picture.change') : t('kid.farm.picture.choose')}
          <input type="file" accept="image/png" hidden onChange={e => choose(e.target.files?.[0])} />
        </label>
        {error && <p className={k.helpLine} role="alert">{error}</p>}
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
        <button type="button" className={k.primaryAction} disabled={!image || busy} onClick={() => onConfirm({ image })}>
          {t('kid.farm.pick.confirm', { count: reward.effective_cost })}
        </button>
      </div>
    </Sheet>
  );
}
