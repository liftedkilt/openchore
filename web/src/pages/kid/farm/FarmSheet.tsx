// The family farm, opened from Today: everyone's animals, plus your own
// animal's care — adopt one, feed / treat / play (pet-care rewards), and
// choose which accessory it wears.
import { useCallback, useEffect, useState } from 'react';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import { api, APIError } from '../../../api';
import { PET_COLORS, PET_SPECIES, type Pet, type PetItem, type PetPick, type PetSpecies, type Reward, type User } from '../../../types';
import { Sheet } from '../Sheet';
import type { KidData } from '../useKidData';
import { FarmAnimal } from './FarmAnimal';
import { FarmScene } from './FarmScene';
import { ColorPicker, PetLookSheet } from './PetLookSheet';
import { DesignSheet } from './DesignSheet';
import { ReactingAnimal, reactionLine, type ReactionEvent } from './PetReaction';
import { itemLabel, needsPick } from './petLook';
import { PetRewardThumb } from './PetRewardThumb';
import k from '../kid.module.css';
import s from './petSheet.module.css';

interface FarmSheetProps {
  user: User;
  data: KidData;
  onClose: () => void;
}

export function FarmSheet({ user, data, onClose }: FarmSheetProps) {
  const { t } = useTranslation();
  const pets = data.farm?.pets ?? [];
  const mine = data.myPet;
  const [selected, setSelected] = useState<Pet | null>(null);
  const other = selected && selected.user_id !== user.id ? pets.find(p => p.id === selected.id) ?? null : null;

  return (
    <Sheet title={t('kid.farm.title')} onClose={onClose} className={s.sheet}>
      <FarmScene pets={pets} meId={user.id} onSelect={setSelected} />
      {other && (
        <div className={s.otherCard} role="status">
          <FarmAnimal species={other.species} color={other.color} mood={other.mood} accessory={other.accessory} className={s.otherArt} />
          <p>
            <strong>{t('kid.farm.belongsTo', { name: other.name, owner: other.owner_name ?? '' })}</strong>
            <span>{t(`kid.farm.mood.${other.mood}`, { name: other.name })}</span>
          </p>
        </div>
      )}
      {mine ? <MyAnimal pet={mine} data={data} /> : <Adopt data={data} />}
    </Sheet>
  );
}

function Adopt({ data }: { data: KidData }) {
  const { t } = useTranslation();
  const [species, setSpecies] = useState<PetSpecies | null>(null);
  const [color, setColor] = useState('');
  const [name, setName] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const adopt = async () => {
    if (!species || !name.trim()) return;
    setBusy(true);
    setError(null);
    try {
      await api.pets.adopt(name.trim(), species, color || undefined);
      await data.loadExtras();
    } catch (e) {
      setError(e instanceof APIError ? e.message : t('kid.farm.adoptFailed'));
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className={s.section} aria-labelledby="farm-adopt">
      <h3 id="farm-adopt" className={s.heading}>{t('kid.farm.adoptTitle')}</h3>
      <p className={k.helpLine}>{t('kid.farm.adoptLead')}</p>
      <div className={s.species} role="radiogroup" aria-label={t('kid.farm.chooseAnimal')}>
        {PET_SPECIES.map(sp => (
          <button
            key={sp}
            type="button"
            role="radio"
            aria-checked={species === sp}
            className={clsx(s.speciesBtn, species === sp && s.speciesOn)}
            onClick={() => { setSpecies(sp); setColor(PET_COLORS[sp][0]); }}
          >
            <FarmAnimal species={sp} color={species === sp ? color : undefined} mood="happy" className={s.speciesArt} />
            <span>{t(`kid.farm.species.${sp}`)}</span>
          </button>
        ))}
      </div>
      {species && (
        <>
          <h4 className={s.subheading}>{t('kid.farm.coat')}</h4>
          <ColorPicker species={species} value={color} onChange={setColor} />
        </>
      )}
      <label className={s.nameRow}>
        <span>{t('kid.farm.nameLabel')}</span>
        <input
          className={k.input}
          value={name}
          maxLength={24}
          onChange={e => setName(e.target.value)}
          placeholder={t('kid.farm.namePlaceholder')}
        />
      </label>
      {error && <p className={k.errorLine} role="alert">{error}</p>}
      <div className={k.sheetActions}>
        <button type="button" className={k.primaryAction} disabled={!species || !name.trim() || busy} onClick={adopt}>
          {t('kid.farm.adopt')}
        </button>
      </div>
    </section>
  );
}

function Meter({ label, value, tone }: { label: string; value: number; tone: 'health' | 'food' | 'joy' }) {
  return (
    <div className={s.meter}>
      <span className={s.meterLabel}>{label}</span>
      <span className={s.meterTrack} role="meter" aria-label={label} aria-valuemin={0} aria-valuemax={100} aria-valuenow={value}>
        <span className={clsx(s.meterFill, s[tone], value < 30 && s.low)} style={{ width: `${value}%` }} />
      </span>
    </div>
  );
}

function MyAnimal({ pet, data }: { pet: Pet; data: KidData }) {
  const { t } = useTranslation();
  const [rewards, setRewards] = useState<Reward[] | null>(null);
  const [renaming, setRenaming] = useState(false);
  const [name, setName] = useState(pet.name);
  const [busy, setBusy] = useState(false);
  const [picking, setPicking] = useState<Reward | null>(null);
  const [tryOn, setTryOn] = useState<string | null>(null);
  const [reaction, setReaction] = useState<ReactionEvent | null>(null);
  const react = (kind: ReactionEvent['kind']) => setReaction({ kind, key: Date.now() });
  const balance = data.points?.balance ?? 0;

  const loadRewards = useCallback(() => {
    api.rewards.list().then(setRewards).catch(() => setRewards([]));
  }, []);
  useEffect(loadRewards, [loadRewards]);

  const care = (rewards ?? []).filter(r => r.pet_action && r.pet_action !== 'accessory');
  const shopItems = (rewards ?? []).filter(r => r.pet_action === 'accessory' && r.pet_item && !pet.items.includes(r.pet_item));

  const redeem = async (r: Reward, pick?: PetPick) => {
    if (needsPick(r) && !pick) {
      setPicking(r);
      return;
    }
    if (await data.redeem(r, pick)) {
      setPicking(null);
      setTryOn(null);
      if (r.pet_action) react(r.pet_action);
      loadRewards();
    }
  };

  const wear = async (item: PetItem | '') => {
    setBusy(true);
    try {
      await api.pets.update({ accessory: item });
      await data.loadExtras();
      if (item) react('wear');
    } catch (e) {
      data.showToast(e instanceof APIError ? e.message : t('kid.farm.saveFailed'));
    } finally {
      setBusy(false);
    }
  };

  const rename = async () => {
    if (!name.trim()) return;
    setBusy(true);
    try {
      await api.pets.update({ name: name.trim() });
      await data.loadExtras();
      setRenaming(false);
    } catch (e) {
      data.showToast(e instanceof APIError ? e.message : t('kid.farm.saveFailed'));
    } finally {
      setBusy(false);
    }
  };

  const hint = pet.hiding ? t('kid.farm.hint.hiding', { name: pet.name })
    : pet.sick ? t('kid.farm.hint.sick', { name: pet.name })
    : pet.mood === 'hungry' ? t('kid.farm.hint.hungry', { name: pet.name })
    : pet.mood === 'bored' ? t('kid.farm.hint.bored', { name: pet.name })
    : t('kid.farm.hint.ok', { name: pet.name });

  return (
    <section className={s.section} aria-labelledby="farm-mine">
      <div className={s.mineHead}>
        <ReactingAnimal pet={pet} reaction={reaction} accessory={tryOn ?? undefined} artClassName={s.mineArt} />
        <div className={s.mineText}>
          {renaming ? (
            <div className={k.addRow}>
              <input className={k.input} value={name} maxLength={24} onChange={e => setName(e.target.value)} aria-label={t('kid.farm.nameLabel')} />
              <button type="button" className={k.linkBtn} onClick={rename} disabled={busy || !name.trim()}>{t('common.save')}</button>
            </div>
          ) : (
            <h3 id="farm-mine" className={s.heading}>
              {pet.name}
              <button type="button" className={s.renameBtn} onClick={() => setRenaming(true)}>{t('kid.farm.rename')}</button>
            </h3>
          )}
          <p className={s.moodLine} aria-live="polite">
            {reaction ? reactionLine(t, pet, reaction.kind) : t(`kid.farm.mood.${pet.mood}`, { name: pet.name })}
          </p>
          <p className={s.hint}>{hint}</p>
        </div>
      </div>

      <div className={s.meters}>
        <Meter label={t('kid.farm.health')} value={pet.health} tone="health" />
        <Meter label={t('kid.farm.food')} value={pet.fullness} tone="food" />
        <Meter label={t('kid.farm.joy')} value={pet.joy} tone="joy" />
      </div>

      <h4 className={s.subheading}>{t('kid.farm.careTitle')}</h4>
      {rewards && care.length === 0 && <p className={k.helpLine}>{t('kid.farm.noCare')}</p>}
      <ul className={s.careList}>
        {care.map(r => {
          const blocked = r.pet_action === 'play' && !pet.can_play;
          const short = balance < r.effective_cost;
          return (
            <li key={r.id}>
              <button
                type="button"
                className={s.careBtn}
                disabled={blocked || short || data.redeemingId === r.id}
                onClick={() => redeem(r)}
                title={blocked ? (pet.hiding ? t('kid.farm.hint.hiding', { name: pet.name }) : t('kid.farm.tooSick', { name: pet.name })) : undefined}
              >
                <PetRewardThumb pet={pet} reward={r} className={s.careThumb} />
                <span className={s.careName}>{r.name}</span>
                <span className={s.careCost}>{t('kid.rewards.cost', { count: r.effective_cost })}</span>
              </button>
            </li>
          );
        })}
      </ul>

      <h4 className={s.subheading}>{t('kid.farm.wardrobe')}</h4>
      <div className={k.ownChips} role="group" aria-label={t('kid.farm.wardrobe')}>
        <button type="button" className={clsx(k.smallBtn, k.ownChip, !pet.accessory && k.ownChipOn)} aria-pressed={!pet.accessory} disabled={busy} onClick={() => wear('')}>
          {t('kid.farm.item.none')}
        </button>
        {pet.items.map(it => (
          <button key={it} type="button" className={clsx(k.smallBtn, k.ownChip, s.wardrobeChip, pet.accessory === it && k.ownChipOn)} aria-pressed={pet.accessory === it} disabled={busy} onClick={() => wear(it)}>
            <FarmAnimal species={pet.species} color={pet.color} mood="content" accessory={it} className={s.wardrobeArt} />
            {itemLabel(t, it)}
          </button>
        ))}
      </div>

      {shopItems.length > 0 && (
        <>
          <h4 className={s.subheading}>{t('kid.farm.shop')}</h4>
          <p className={s.hint}>{t('kid.farm.tryOnHint')}</p>
          <ul className={s.careList}>
            {shopItems.map(r => (
              <li key={r.id}>
                <div className={clsx(s.shopItem, tryOn === r.pet_item && s.speciesOn)}>
                  <button type="button" className={s.tryOnBtn} aria-pressed={tryOn === r.pet_item}
                    onClick={() => setTryOn(tryOn === r.pet_item ? null : r.pet_item ?? null)}>
                    <FarmAnimal species={pet.species} color={pet.color} mood="happy" accessory={r.pet_item} className={s.careArt} />
                    <span className={s.careName}>{r.name}</span>
                  </button>
                  <button type="button" className={s.buyBtn}
                    disabled={balance < r.effective_cost || data.redeemingId === r.id} onClick={() => redeem(r)}>
                    {t('kid.rewards.cost', { count: r.effective_cost })}
                  </button>
                </div>
              </li>
            ))}
          </ul>
        </>
      )}

      {picking && picking.pet_action === 'design' && (
        <DesignSheet pet={pet} reward={picking} busy={data.redeemingId === picking.id}
          onConfirm={pick => redeem(picking, pick)} onClose={() => setPicking(null)} />
      )}
      {picking && picking.pet_action !== 'design' && (
        <PetLookSheet pet={pet} reward={picking} busy={data.redeemingId === picking.id}
          onConfirm={pick => redeem(picking, pick)} onClose={() => setPicking(null)} />
      )}
    </section>
  );
}
