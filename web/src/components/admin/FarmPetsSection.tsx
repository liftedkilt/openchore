// Settings → Farm pets: turn the optional farm pets on or off. The first
// time they're turned on, a starter set of pet-care rewards is added.
import React, { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { api, APIError } from '../../api';
import type { Pet, PetGrave } from '../../types';
import { FarmAnimal, petPicture } from '../../pages/kid/farm/FarmAnimal';
import { Tombstone } from '../../pages/kid/farm/Tombstone';
import { FormMessage } from './FormMessage';
import ui from './ui.module.css';
import fp from './FarmPetsSection.module.css';

export const FarmPetsSection: React.FC = () => {
  const { t } = useTranslation();
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [pets, setPets] = useState<Pet[]>([]);
  const [graves, setGraves] = useState<PetGrave[]>([]);
  // Pet deaths: 0 = animals never die; otherwise days in a row sick.
  const [deathDays, setDeathDays] = useState(0);
  const [daysDraft, setDaysDraft] = useState('7');
  const [confirmingGrave, setConfirmingGrave] = useState<number | null>(null);
  const [confirming, setConfirming] = useState<number | null>(null);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  const load = useCallback(() => {
    api.pets.farm().then(f => {
      setEnabled(f.enabled);
      setPets(f.pets);
      setGraves(f.graves ?? []);
      const d = f.death_days ?? 0;
      setDeathDays(d);
      if (d > 0) setDaysDraft(String(d));
    }).catch(() => setEnabled(false));
  }, []);
  useEffect(load, [load]);

  const remove = async (p: Pet) => {
    if (confirming !== p.id) {
      setConfirming(p.id);
      return;
    }
    setConfirming(null);
    try {
      await api.pets.remove(p.user_id);
      setMessage({ type: 'success', text: t('admin.settingsTab.pets.removed', { name: p.name, owner: p.owner_name ?? '' }) });
      load();
    } catch {
      setMessage({ type: 'error', text: t('admin.settingsTab.pets.failed') });
    }
  };

  const approvePicture = async (p: Pet, approved: boolean) => {
    try {
      await api.pets.approvePicture(p.user_id, approved, p.sprite);
      setMessage({ type: 'success', text: t(approved ? 'admin.settingsTab.pets.approved' : 'admin.settingsTab.pets.unapproved', { name: p.name }) });
      load();
    } catch (e) {
      // 409: the kid changed the picture while this page was open.
      setMessage({ type: 'error', text: e instanceof APIError && e.status === 409 ? t('admin.settingsTab.pets.pictureChanged') : t('admin.settingsTab.pets.failed') });
      load();
    }
  };

  const removePicture = async (p: Pet) => {
    try {
      await api.pets.removePicture(p.user_id);
      setMessage({ type: 'success', text: t('admin.settingsTab.pets.pictureRemoved', { name: p.name }) });
      load();
    } catch {
      setMessage({ type: 'error', text: t('admin.settingsTab.pets.failed') });
    }
  };

  const saveDeathDays = async (n: number) => {
    setSaving(true);
    setMessage(null);
    try {
      const res = await api.pets.setDeathDays(n);
      setDeathDays(res.death_days);
      setMessage({ type: 'success', text: res.death_days > 0 ? t('admin.settingsTab.pets.deathsOn', { count: res.death_days }) : t('admin.settingsTab.pets.deathsOff') });
      load();
    } catch {
      setMessage({ type: 'error', text: t('admin.settingsTab.pets.failed') });
    }
    setSaving(false);
  };

  const removeGrave = async (g: PetGrave) => {
    if (confirmingGrave !== g.id) {
      setConfirmingGrave(g.id);
      return;
    }
    setConfirmingGrave(null);
    try {
      await api.pets.removeGrave(g.id);
      setMessage({ type: 'success', text: t('admin.settingsTab.pets.graveRemoved', { name: g.name }) });
      load();
    } catch {
      setMessage({ type: 'error', text: t('admin.settingsTab.pets.failed') });
    }
  };

  const draftDays = Number(daysDraft);
  const draftValid = Number.isInteger(draftDays) && draftDays >= 2 && draftDays <= 60;

  const toggle = async (on: boolean) => {
    setSaving(true);
    setMessage(null);
    try {
      const res = await api.pets.setEnabled(on);
      setEnabled(res.enabled);
      load();
      setMessage({
        type: 'success',
        text: res.rewards_added > 0
          ? t('admin.settingsTab.pets.addedRewards', { count: res.rewards_added })
          : on ? t('admin.settingsTab.pets.on') : t('admin.settingsTab.pets.off'),
      });
    } catch {
      setMessage({ type: 'error', text: t('admin.settingsTab.pets.failed') });
    }
    setSaving(false);
  };

  return (
    <section className={clsx(ui.card, ui.section)}>
      <h3 className={ui.sectionTitle}>{t('admin.settingsTab.pets.title')}</h3>
      <p className={ui.sectionDesc}>{t('admin.settingsTab.pets.description')}</p>
      <label className={ui.check}>
        <input
          type="checkbox"
          checked={!!enabled}
          disabled={enabled === null || saving}
          onChange={e => toggle(e.target.checked)}
        />
        <span className={ui.checkLabel}>{t('admin.settingsTab.pets.toggle')}</span>
      </label>
      {enabled && (
        <div className={fp.deaths}>
          <label className={ui.check}>
            <input
              type="checkbox"
              checked={deathDays > 0}
              disabled={saving || (deathDays === 0 && !draftValid)}
              onChange={e => saveDeathDays(e.target.checked ? draftDays : 0)}
            />
            <span className={ui.checkLabel}>{t('admin.settingsTab.pets.deathsToggle')}</span>
          </label>
          <div className={fp.deathDays}>
            <label htmlFor="pet-death-days">{t('admin.settingsTab.pets.deathDaysLabel')}</label>
            <input
              id="pet-death-days"
              className={clsx(ui.input, fp.daysInput)}
              type="number"
              inputMode="numeric"
              min={2}
              max={60}
              value={daysDraft}
              onChange={e => setDaysDraft(e.target.value)}
            />
            <span>{t('admin.settingsTab.pets.deathDaysUnit')}</span>
            {deathDays > 0 && draftValid && draftDays !== deathDays && (
              <button type="button" className={ui.btnGhost} disabled={saving} onClick={() => saveDeathDays(draftDays)}>
                {t('common.save')}
              </button>
            )}
          </div>
          <p className={ui.help}>{t('admin.settingsTab.pets.deathsHelp')}</p>
        </div>
      )}
      {enabled && pets.length > 0 && (
        <ul className={ui.list}>
          {pets.map(p => (
            <li key={p.id} className={fp.row}>
              <FarmAnimal species={p.species} color={p.color} mood={p.mood} accessory={p.accessory} {...petPicture(p)} className={fp.thumb} />
              <span className={fp.text}>
                <strong>{p.name}</strong> · {p.owner_name} · {t(`kid.farm.mood.${p.mood}`, { name: p.name })}
              </span>
              <button type="button" className={confirming === p.id ? ui.btnDanger : ui.btnGhost} onClick={() => remove(p)}>
                {confirming === p.id ? t('admin.settingsTab.pets.confirmRemove') : t('admin.settingsTab.pets.remove')}
              </button>
              {p.sprite && (
                <div className={fp.picture}>
                  <label className={ui.check} title={t('admin.settingsTab.pets.approveTitle')}>
                    <input type="checkbox" checked={!!p.sprite_approved} onChange={e => approvePicture(p, e.target.checked)} />
                    <span className={ui.checkLabel}>{t('admin.settingsTab.pets.approve')}</span>
                  </label>
                  <button type="button" className={ui.btnGhost} onClick={() => removePicture(p)}>
                    {t('admin.settingsTab.pets.removePicture')}
                  </button>
                </div>
              )}
            </li>
          ))}
        </ul>
      )}
      {enabled && graves.length > 0 && (
        <>
          <h4 className={fp.subTitle}>{t('admin.settingsTab.pets.gravesTitle')}</h4>
          <ul className={ui.list}>
            {graves.map(g => (
              <li key={g.id} className={fp.row}>
                <Tombstone name={g.name} className={fp.graveThumb} />
                <span className={fp.text}>
                  <strong>{g.name}</strong> · {g.owner_name} · {t('admin.settingsTab.pets.diedOn', { date: g.died_on })}
                </span>
                <button type="button" className={confirmingGrave === g.id ? ui.btnDanger : ui.btnGhost} onClick={() => removeGrave(g)}>
                  {confirmingGrave === g.id ? t('admin.settingsTab.pets.confirmRemove') : t('admin.settingsTab.pets.removeGrave')}
                </button>
              </li>
            ))}
          </ul>
        </>
      )}
      <FormMessage msg={message} />
    </section>
  );
};
