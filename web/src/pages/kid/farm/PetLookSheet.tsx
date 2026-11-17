// Pick a new animal and/or coat before redeeming a "new_animal" or
// "restyle" reward, with a live preview of how it'll look.
import { useState } from 'react';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import { PET_COLORS, PET_SPECIES, type Pet, type PetSpecies, type Reward } from '../../../types';
import { Sheet } from '../Sheet';
import { FarmAnimal } from './FarmAnimal';
import { colorLabel, petColor } from './petLook';
import k from '../kid.module.css';
import s from './petSheet.module.css';

interface PetLookSheetProps {
  pet: Pet;
  reward: Reward;
  busy?: boolean;
  onConfirm: (pick: { species: PetSpecies; color: string }) => void;
  onClose: () => void;
}

export function PetLookSheet({ pet, reward, busy, onConfirm, onClose }: PetLookSheetProps) {
  const { t } = useTranslation();
  const swap = reward.pet_action === 'new_animal';
  const current = { species: pet.species, color: petColor(pet.species, pet.color) };
  const [species, setSpecies] = useState<PetSpecies>(pet.species);
  const [color, setColor] = useState(current.color);
  const unchanged = species === current.species && color === current.color;

  const pickSpecies = (sp: PetSpecies) => {
    setSpecies(sp);
    setColor(PET_COLORS[sp][0]);
  };

  return (
    <Sheet title={swap ? t('kid.farm.pick.swapTitle') : t('kid.farm.pick.restyleTitle', { name: pet.name })} onClose={onClose} className={s.sheet}>
      <div className={s.pickPreview}>
        <FarmAnimal species={species} color={color} mood="happy" accessory={pet.accessory} className={s.pickArt} />
        <p className={s.hint}>{swap ? t('kid.farm.pick.swapLead', { name: pet.name }) : t('kid.farm.pick.restyleLead')}</p>
      </div>

      {swap && (
        <div className={s.species} role="radiogroup" aria-label={t('kid.farm.chooseAnimal')}>
          {PET_SPECIES.map(sp => (
            <button key={sp} type="button" role="radio" aria-checked={species === sp}
              className={clsx(s.speciesBtn, species === sp && s.speciesOn)} onClick={() => pickSpecies(sp)}>
              <FarmAnimal species={sp} mood="content" className={s.speciesArt} />
              <span>{t(`kid.farm.species.${sp}`)}</span>
            </button>
          ))}
        </div>
      )}

      <ColorPicker species={species} value={color} onChange={setColor} />

      <div className={k.sheetActions}>
        <button type="button" className={k.primaryAction} disabled={unchanged || busy} onClick={() => onConfirm({ species, color })}>
          {t('kid.farm.pick.confirm', { count: reward.effective_cost })}
        </button>
      </div>
    </Sheet>
  );
}

/** Coat swatches: the animal itself in each colour. */
export function ColorPicker({ species, value, onChange }: { species: PetSpecies; value: string; onChange: (c: string) => void }) {
  const { t } = useTranslation();
  return (
    <div className={s.colors} role="radiogroup" aria-label={t('kid.farm.coat')}>
      {PET_COLORS[species].map(c => (
        <button key={c} type="button" role="radio" aria-checked={value === c}
          className={clsx(s.colorBtn, value === c && s.speciesOn)} onClick={() => onChange(c)}>
          <FarmAnimal species={species} color={c} mood="content" className={s.colorArt} />
          <span>{colorLabel(t, c)}</span>
        </button>
      ))}
    </div>
  );
}
