// "Use my own picture": choose a PNG to show in place of the drawn animal,
// see it on the grass, then redeem. The starter kit (a blank canvas, a guide
// and an example) is linked here so a picture can be made in any art program.
// See docs/custom-animals.md.
import { useState } from 'react';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import type { Pet, Reward } from '../../../types';
import { Sheet } from '../Sheet';
import { FarmAnimal } from './FarmAnimal';
import k from '../kid.module.css';
import s from './petSheet.module.css';

const KIT = [
  ['kitBlank', '/sprites/blank-64.png'],
  ['kitLayer', '/sprites/guide-layer-64.png'],
  ['kitGuide', '/sprites/guide.png'],
  ['kitSample', '/sprites/sample-duck.png'],
] as const;

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
      setError('');
      setImage(typeof reader.result === 'string' ? reader.result : '');
    };
    reader.readAsDataURL(file);
  };

  return (
    <Sheet title={t('kid.farm.picture.title')} onClose={onClose}>
      <p className={k.helpLine}>{t('kid.farm.picture.lead')}</p>

      <div className={s.pickPreview}>
        <div className={s.pictureGrass} aria-label={t('kid.farm.picture.preview')}>
          <FarmAnimal
            species={pet.species}
            color={pet.color}
            mood="happy"
            accessory={pet.accessory}
            sprite={image || pet.sprite}
            spritePreview={!!image}
            className={s.pickArt}
          />
        </div>
        <label className={clsx(s.chip, s.uploadBtn)}>
          {image ? t('kid.farm.picture.change') : t('kid.farm.picture.choose')}
          <input type="file" accept="image/png" hidden onChange={e => choose(e.target.files?.[0])} />
        </label>
        {error && <p className={k.helpLine} role="alert">{error}</p>}
      </div>

      <p className={k.helpLine}>{t('kid.farm.picture.rules')}</p>
      <p className={k.helpLine}>{t('kid.farm.picture.kit')}</p>
      <div className={k.ownChips}>
        {KIT.map(([key, href]) => (
          <a key={key} className={s.chip} href={href} download>{t(`kid.farm.picture.${key}`)}</a>
        ))}
      </div>

      <div className={k.sheetActions}>
        <button type="button" className={k.primaryAction} disabled={!image || busy} onClick={() => onConfirm({ image })}>
          {t('kid.farm.pick.confirm', { count: reward.effective_cost })}
        </button>
      </div>
    </Sheet>
  );
}
