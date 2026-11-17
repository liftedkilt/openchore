// Settings → Farm pets: turn the optional farm pets on or off. The first
// time they're turned on, a starter set of pet-care rewards is added.
import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { api } from '../../api';
import { FormMessage } from './FormMessage';
import ui from './ui.module.css';

export const FarmPetsSection: React.FC = () => {
  const { t } = useTranslation();
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  useEffect(() => {
    api.pets.farm().then(f => setEnabled(f.enabled)).catch(() => setEnabled(false));
  }, []);

  const toggle = async (on: boolean) => {
    setSaving(true);
    setMessage(null);
    try {
      const res = await api.pets.setEnabled(on);
      setEnabled(res.enabled);
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
      <FormMessage msg={message} />
    </section>
  );
};
