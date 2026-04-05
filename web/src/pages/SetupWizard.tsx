import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../AuthContext';
import { api } from '../api';
import styles from './SetupWizard.module.css';
import { UserPlus, Check, ArrowRight, Sparkles, Trash2, Palette } from 'lucide-react';

type Step = 'welcome' | 'children' | 'themes' | 'chores' | 'finish';

const THEMES = [
  { id: 'default', name: 'Classic Blue', color: '#3b82f6' },
  { id: 'quest', name: 'Quest Adventure', color: '#f59e0b' },
  { id: 'galaxy', name: 'Galaxy Purple', color: '#8b5cf6' },
  { id: 'forest', name: 'Nature Forest', color: '#10b981' },
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
      setError('Failed to complete setup. Please try again.');
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
            <h1>Welcome to OpenChore!</h1>
            <p>Let's get your family set up in just a few minutes.</p>
            <button className={styles.primaryBtn} onClick={() => setStep('children')}>
              Get Started <ArrowRight size={20} />
            </button>
          </div>
        );

      case 'children':
        return (
          <div className={styles.stepContent}>
            <h1>Who's doing chores?</h1>
            <p>Add your children's names to get started.</p>
            
            <div className={styles.inputRow}>
              <input 
                type="text" 
                placeholder="Child's name" 
                value={newName} 
                onChange={e => setNewName(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && addChild()}
              />
              <button className={styles.addBtn} onClick={addChild} disabled={!newName.trim()}>
                <UserPlus size={20} /> Add
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
                Next <ArrowRight size={20} />
              </button>
            </div>
          </div>
        );

      case 'themes':
        return (
          <div className={styles.stepContent}>
            <h1>Pick their style</h1>
            <p>Each child can have their own favorite theme.</p>
            
            <div className={styles.themeGrid}>
              {children.map((c, i) => (
                <div key={i} className={styles.themeCard}>
                  <span className={styles.childName}>{c.name}</span>
                  <div className={styles.themeOptions}>
                    {THEMES.map(t => (
                      <button 
                        key={t.id} 
                        className={`${styles.themeOption} ${c.theme === t.id ? styles.activeTheme : ''}`}
                        style={{ backgroundColor: t.color }}
                        onClick={() => updateChildTheme(i, t.id)}
                        title={t.name}
                      >
                        {c.theme === t.id && <Check size={16} color="white" />}
                      </button>
                    ))}
                  </div>
                </div>
              ))}
            </div>

            <div className={styles.navBtns}>
              <button className={styles.secondaryBtn} onClick={() => setStep('children')}>Back</button>
              <button className={styles.primaryBtn} onClick={() => setStep('chores')}>
                Next <ArrowRight size={20} />
              </button>
            </div>
          </div>
        );

      case 'chores':
        return (
          <div className={styles.stepContent}>
            <h1>Assign first chores</h1>
            <p>Select some common chores to get started. You can add more later.</p>
            
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
              <button className={styles.secondaryBtn} onClick={() => setStep('themes')}>Back</button>
              <button className={styles.primaryBtn} onClick={handleFinish} disabled={loading}>
                {loading ? 'Setting up...' : 'Finish Setup'}
              </button>
            </div>
          </div>
        );

      case 'finish':
        return (
          <div className={styles.stepContent}>
            <div className={styles.iconCircle} style={{ backgroundColor: '#10b981' }}><Check size={48} color="white" /></div>
            <h1>All set!</h1>
            <p>Your family is ready to start earning points.</p>
            <button className={styles.primaryBtn} onClick={() => navigate('/login')}>
              Go to Login
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
