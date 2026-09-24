import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { Download } from 'lucide-react';
import { api } from '../../api';
import ui from './ui.module.css';

const EXPORT_SECTIONS = ['users', 'chores', 'rewards', 'streak_rewards', 'settings'] as const;

export const ExportConfigSection: React.FC = () => {
  const { t } = useTranslation();
  const [selected, setSelected] = useState<Set<string>>(new Set(EXPORT_SECTIONS));
  const [exporting, setExporting] = useState(false);

  const toggle = (id: string) => {
    setSelected(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const handleExport = async () => {
    if (selected.size === 0) return;
    setExporting(true);
    try {
      const blob = await api.admin.exportConfig(Array.from(selected));
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'config.yaml';
      a.click();
      URL.revokeObjectURL(url);
    } catch (e) {
      console.error('Export failed', e);
    }
    setExporting(false);
  };

  return (
    <section className={clsx(ui.card, ui.section)}>
      <h3 className={ui.sectionTitle}>{t('admin.exportConfig.title')}</h3>
      <p className={ui.sectionDesc}>
        {t('admin.exportConfig.descriptionBefore')}<code>config.yaml</code>{t('admin.exportConfig.descriptionAfter')}
      </p>
      <fieldset className={ui.chips} style={{ border: 0, padding: 0, margin: 0 }}>
        <legend className={ui.srOnlyText}>{t('admin.exportConfig.sectionsLabel')}</legend>
        {EXPORT_SECTIONS.map(id => (
          <label key={id} className={ui.checkChip}>
            <input type="checkbox" checked={selected.has(id)} onChange={() => toggle(id)} />
            {t(`admin.exportConfig.sections.${id}`)}
          </label>
        ))}
      </fieldset>
      <div className={ui.actionsEnd}>
        <button type="button" className={ui.btnPrimary} onClick={handleExport} disabled={exporting || selected.size === 0}>
          <Download aria-hidden /> {exporting ? t('admin.exportConfig.exporting') : t('admin.exportConfig.download')}
        </button>
      </div>
    </section>
  );
};
