import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../AuthContext';
import { api } from '../api';
import styles from './SetupWizard.module.css';
import { UserPlus, Check, ArrowRight, Sparkles, Trash2, Palette } from 'lucide-react';

type Step = 'welcome' | 'children' | 'themes' | 'chores' | 'finish';

const THEMES = [
  { id: 'default', nameKey: 'themeClassicBlue', color: '#3b82f6' },
  { id: 'quest', nameKey: 'themeQuestAdventure', color: '#f59e0b' },
  { id: 'galaxy', nameKey: 'themeGalaxyPurple', color: '#8b5cf6' },
  { id: 'forest', nameKey: 'themeNatureForest', color: '#10b981' },
];

const CHORE_PRESETS = [
  { title: 'Brush Teeth', icon: '🪥', category: 'required', points: 5 },
  { title: 'Make Bed', icon: '🛏️', category: 'core', points: 10 },
  { title: 'Clean Room', icon: '🧹', category: 'core', points: 20 },
  { title: 'Feed Pet', icon: '🐾', category: 'required', points: 5 },
  { title: 'Set Table', icon: '🍽️', category: 'core', points: 10 },
  { title: 'Read 20 Mins', icon: '📚', category: 'bonus', points: 15 },
];

export const SetupWizard: React.FC = () => {
  const { t } = useTranslation();
  const [step, setStep] = useState<Step>('welcome');
  const [children, setChildren] = useState<{ name: string; age?: number; theme: string; id?: number }[]>([]);
  const [newName, setNewName] = useState('');
  const [selectedPresets, setSelectedPresets] = useState<number[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const navigate = useNavigate();
  const { setUser } = useAuth();

  const addChild = () => {
    if (!newName.trim()) return;
    setChildren([...children, { name: newName, theme: 'default' }]);
    setNewName('');
  };

  const removeChild = (index: number) => {
    setChildren(children.filter((_, i) => i !== index));
  };

  const updateChildTheme = (index: number, theme: string) => {
    const newChildren = [...children];
    newChildren[index].theme = theme;
    setChildren(newChildren);
  };

  const handleFinish = async () => {
    setLoading(true);
    setError('');
    try {
      const result = await api.setup({
        children: children.map(c => ({ name: c.name, theme: c.theme })),
        chores: selectedPresets.map(idx => {
          const preset = CHORE_PRESETS[idx];
          return {
            title: preset.title,
            icon: preset.icon,
            category: preset.category,
            points_value: preset.points,
          };
        }),
      });

      // Store admin user for subsequent authenticated requests
      setUser(result.admin);

      setStep('finish');
    } catch (err) {
      console.error(err);
      setError(t('setup.errorSetupFailed'));
    } finally {
      setLoading(false);
    }
  };

  const renderStep = () => {
    switch (step) {
      case 'welcome':
        return (
          <div className={styles.stepContent}>
            <div className={styles.iconCircle}><Sparkles size={48} /></div>
            <h1>{t('setup.welcomeTitle')}</h1>
            <p>{t('setup.welcomeDescription')}</p>
            <button className={styles.primaryBtn} onClick={() => setStep('children')}>
              {t('setup.getStarted')} <ArrowRight size={20} />
            </button>
          </div>
        );

      case 'children':
        return (
          <div className={styles.stepContent}>
            <h1>{t('setup.childrenTitle')}</h1>
            <p>{t('setup.childrenDescription')}</p>

            <div className={styles.inputRow}>
              <input
                type="text"
                placeholder={t('setup.childNamePlaceholder')}
                value={newName}
                onChange={e => setNewName(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && addChild()}
              />
              <button className={styles.addBtn} onClick={addChild} disabled={!newName.trim()}>
                <UserPlus size={20} /> {t('setup.addButton')}
              </button>
            </div>

            <div className={styles.list}>
              {children.map((c, i) => (
                <div key={i} className={styles.listItem}>
                  <span>{c.name}</span>
                  <button onClick={() => removeChild(i)} className={styles.removeBtn}>
                    <Trash2 size={18} />
                  </button>
                </div>
              ))}
            </div>

            <div className={styles.navBtns}>
              <button
                className={styles.primaryBtn}
                disabled={children.length === 0}
                onClick={() => setStep('themes')}
              >
                {t('setup.next')} <ArrowRight size={20} />
              </button>
            </div>
          </div>
        );

      case 'themes':
        return (
          <div className={styles.stepContent}>
            <h1>{t('setup.themesTitle')}</h1>
            <p>{t('setup.themesDescription')}</p>
            
            <div className={styles.themeGrid}>
              {children.map((c, i) => (
                <div key={i} className={styles.themeCard}>
                  <span className={styles.childName}>{c.name}</span>
                  <div className={styles.themeOptions}>
                    {THEMES.map(theme => (
                      <button
                        key={theme.id}
                        className={`${styles.themeOption} ${c.theme === theme.id ? styles.activeTheme : ''}`}
                        style={{ backgroundColor: theme.color }}
                        onClick={() => updateChildTheme(i, theme.id)}
                        title={t(`setup.${theme.nameKey}`)}
                      >
                        {c.theme === theme.id && <Check size={16} color="white" />}
                      </button>
                    ))}
                  </div>
                </div>
              ))}
            </div>

            <div className={styles.navBtns}>
              <button className={styles.secondaryBtn} onClick={() => setStep('children')}>{t('setup.back')}</button>
              <button className={styles.primaryBtn} onClick={() => setStep('chores')}>
                {t('setup.next')} <ArrowRight size={20} />
              </button>
            </div>
          </div>
        );

      case 'chores':
        return (
          <div className={styles.stepContent}>
            <h1>{t('setup.choresTitle')}</h1>
            <p>{t('setup.choresDescription')}</p>
            
            <div className={styles.presetGrid}>
              {CHORE_PRESETS.map((p, i) => (
                <button 
                  key={i} 
                  className={`${styles.presetCard} ${selectedPresets.includes(i) ? styles.activePreset : ''}`}
                  onClick={() => {
                    if (selectedPresets.includes(i)) {
                      setSelectedPresets(selectedPresets.filter(idx => idx !== i));
                    } else {
                      setSelectedPresets([...selectedPresets, i]);
                    }
                  }}
                >
                  <span className={styles.presetIcon}>{p.icon}</span>
                  <span className={styles.presetTitle}>{p.title}</span>
                  <span className={styles.presetTag} data-category={p.category}>{p.category}</span>
                </button>
              ))}
            </div>

            {error && <p style={{ color: '#ef4444', fontSize: '0.9rem', marginTop: '0.5rem' }}>{error}</p>}

            <div className={styles.navBtns}>
              <button className={styles.secondaryBtn} onClick={() => setStep('themes')}>{t('setup.back')}</button>
              <button className={styles.primaryBtn} onClick={handleFinish} disabled={loading}>
                {loading ? t('setup.settingUp') : t('setup.finishSetup')}
              </button>
            </div>
          </div>
        );

      case 'finish':
        return (
          <div className={styles.stepContent}>
            <div className={styles.iconCircle} style={{ backgroundColor: '#10b981' }}><Check size={48} color="white" /></div>
            <h1>{t('setup.finishTitle')}</h1>
            <p>{t('setup.finishDescription')}</p>
            <button className={styles.primaryBtn} onClick={() => navigate('/login')}>
              {t('setup.goToLogin')}
            </button>
          </div>
        );
    }
  };

  return (
    <div className={styles.container}>
      <div className={styles.card}>
        <div className={styles.progress}>
          <div className={styles.progressBar} style={{ width: `${(['welcome', 'children', 'themes', 'chores', 'finish'].indexOf(step) / 4) * 100}%` }} />
        </div>
        {renderStep()}
      </div>
    </div>
  );
};
