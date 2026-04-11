import React, { useState } from 'react';
import { api } from '../../api';
import styles from '../../pages/AdminDashboard.module.css';
import { Save } from 'lucide-react';

const EXPORT_SECTIONS = [
  { id: 'users', label: 'Users' },
  { id: 'chores', label: 'Chores & Schedules' },
  { id: 'rewards', label: 'Rewards' },
  { id: 'streak_rewards', label: 'Streak Rewards' },
  { id: 'settings', label: 'Settings' },
];

export const ExportConfigSection: React.FC = () => {
  const [selected, setSelected] = useState<Set<string>>(new Set(EXPORT_SECTIONS.map(s => s.id)));
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
    <div className={styles.form} style={{ marginTop: '1.5rem' }}>
      <div className={styles.formHeader}>
        <h3>Export Configuration</h3>
      </div>
      <p className={styles.sectionDesc}>
        Download a <code>config.yaml</code> reflecting the current database state. Use this to bootstrap a fresh instance.
      </p>
      <div className={styles.chipRow} style={{ marginBottom: '1rem' }}>
        {EXPORT_SECTIONS.map(s => (
          <label key={s.id} className={styles.chipLabel}>
            <input type="checkbox" checked={selected.has(s.id)} onChange={() => toggle(s.id)} />
            {s.label}
          </label>
        ))}
      </div>
      <div className={styles.formActions}>
        <button className={styles.btnPrimary} onClick={handleExport} disabled={exporting || selected.size === 0}>
          <Save size={16} /> {exporting ? 'Exporting...' : 'Download config.yaml'}
        </button>
      </div>
    </div>
  );
};
