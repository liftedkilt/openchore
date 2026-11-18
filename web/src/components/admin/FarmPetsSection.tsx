// Settings → Farm pets: turn the optional farm pets on or off. The first
// time they're turned on, a starter set of pet-care rewards is added.
import React, { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { api } from '../../api';
import type { Pet } from '../../types';
import { FarmAnimal } from '../../pages/kid/farm/FarmAnimal';
import { FormMessage } from './FormMessage';
import ui from './ui.module.css';
import fp from './FarmPetsSection.module.css';

export const FarmPetsSection: React.FC = () => {
  const { t } = useTranslation();
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [pets, setPets] = useState<Pet[]>([]);
  const [confirming, setConfirming] = useState<number | null>(null);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  const load = useCallback(() => {
    api.pets.farm().then(f => { setEnabled(f.enabled); setPets(f.pets); }).catch(() => setEnabled(false));
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
      {enabled && pets.length > 0 && (
        <ul className={ui.list}>
          {pets.map(p => (
            <li key={p.id} className={fp.row}>
              <FarmAnimal species={p.species} color={p.color} mood={p.mood} accessory={p.accessory} className={fp.thumb} />
              <span className={fp.text}>
                <strong>{p.name}</strong> · {p.owner_name} · {t(`kid.farm.mood.${p.mood}`, { name: p.name })}
              </span>
              <button type="button" className={confirming === p.id ? ui.btnDanger : ui.btnGhost} onClick={() => remove(p)}>
                {confirming === p.id ? t('admin.settingsTab.pets.confirmRemove') : t('admin.settingsTab.pets.remove')}
              </button>
            </li>
          ))}
        </ul>
      )}
      <FormMessage msg={message} />
    </section>
  );
};
