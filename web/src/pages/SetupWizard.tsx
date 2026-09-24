import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { useAuth } from '../AuthContext';
import { api } from '../api';
import {
  Avatar, BLOCKS_UNDER_AGE, Button, CategoryMark, DayProgress, HouseScope, Icon, PERSON_COLORS, SKINS, SkinScope,
  catFromCategory, resolveChoreIcon, resolveSkin, type DayProgressItem, type PersonColor, type Skin,
} from '../design';
import { BrandMark } from '../components/BrandMark/BrandMark';
import { firstFreeColor } from './SetupWizard.data';
import styles from './SetupWizard.module.css';

type Step = 'welcome' | 'parent' | 'children' | 'looks' | 'chores' | 'finish';

const STEPS: Step[] = ['welcome', 'parent', 'children', 'looks', 'chores', 'finish'];

// Chore presets are data: title, icon and category are stored as given.
const CHORE_PRESETS = [
  { title: 'Brush Teeth', icon: '🪥', category: 'required', points: 5 },
  { title: 'Make Bed', icon: '🛏️', category: 'core', points: 10 },
  { title: 'Clean Room', icon: '🧹', category: 'core', points: 20 },
  { title: 'Feed Pet', icon: '🐾', category: 'required', points: 5 },
  { title: 'Set Table', icon: '🍽️', category: 'core', points: 10 },
  { title: 'Read 20 Mins', icon: '📚', category: 'bonus', points: 15 },
];

// A sample day for the skin previews: two of four done.
const PREVIEW_DAY: DayProgressItem[] = [
  { cat: 'essential', done: true, at: 0.1 },
  { cat: 'daily', done: true, at: 0.35 },
  { cat: 'daily' },
  { cat: 'bonus' },
];

interface Child {
  name: string;
  theme: Skin;
  color: PersonColor;
}

interface ColorPickerProps {
  name: string;
  legend: string;
  value: PersonColor;
  onChange: (c: PersonColor) => void;
}

/** Eight person colours as a radio group. */
function ColorPicker({ name, legend, value, onChange }: ColorPickerProps) {
  const { t } = useTranslation();
  return (
    <fieldset className={styles.fieldset}>
      <legend className={styles.legend}>{legend}</legend>
      <div className={styles.swatches}>
        {PERSON_COLORS.map((c) => (
          <label key={c} className={styles.swatch} data-person={c}>
            <input
              type="radio"
              name={name}
              value={c}
              checked={value === c}
              onChange={() => onChange(c)}
              aria-label={t(`entry.setup.colors.${c}`)}
            />
            <span aria-hidden><Icon name="check" /></span>
          </label>
        ))}
      </div>
    </fieldset>
  );
}

interface SkinPickerProps {
  child: Child;
  index: number;
  onChange: (s: Skin) => void;
}

/** Three small doors, one per skin, each drawn in that skin. */
function SkinPicker({ child, index, onChange }: SkinPickerProps) {
  const { t } = useTranslation();
  // Setup doesn't ask ages, so both suggestions show; the preselected skin is
  // what an unset skin resolves to (see resolveSkin).
  const suggestion = (s: Skin) => (s === 'blocks'
    ? t('entry.setup.suggestUnder', { age: BLOCKS_UNDER_AGE })
    : t('entry.setup.suggestFrom', { age: BLOCKS_UNDER_AGE }));
  return (
    <fieldset className={styles.fieldset}>
      <legend className={styles.legend}>{t('entry.setup.skin')}</legend>
      <div className={styles.skins}>
        {SKINS.map((s) => (
          <label key={s} className={styles.skinOption}>
            <input
              type="radio"
              name={`skin-${index}`}
              value={s}
              checked={child.theme === s}
              onChange={() => onChange(s)}
            />
            <SkinScope skin={s} color={child.color} door className={styles.miniDoor}>
              <Avatar name={child.name} color={child.color} size="sm" />
              <span className={styles.miniName}>{t(`entry.setup.skins.${s}`)}</span>
              <span className={styles.miniZone} aria-hidden>
                <span className={styles.miniHero}>
                  <DayProgress items={PREVIEW_DAY} now={0.5} />
                </span>
              </span>
              {child.theme === s && <span className={styles.miniCheck} aria-hidden><Icon name="check" /></span>}
            </SkinScope>
            <span className={styles.suggest}>{suggestion(s)}</span>
          </label>
        ))}
      </div>
    </fieldset>
  );
}

export const SetupWizard: React.FC = () => {
  const { t } = useTranslation();
  const [step, setStep] = useState<Step>('welcome');
  const [children, setChildren] = useState<Child[]>([]);
  const [newName, setNewName] = useState('');
  const [selectedPresets, setSelectedPresets] = useState<number[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [parentName, setParentName] = useState('');
  const [parentPin, setParentPin] = useState('');
  const [parentPinConfirm, setParentPinConfirm] = useState('');
  const [parentColor, setParentColor] = useState<PersonColor>(PERSON_COLORS[0]);
  const [parentError, setParentError] = useState('');
  const navigate = useNavigate();
  const { refresh, session } = useAuth();

  const submitParent = (e: React.FormEvent) => {
    e.preventDefault();
    if (!/^\d{4,8}$/.test(parentPin)) {
      setParentError(t('setup.parentPinFormat'));
      return;
    }
    if (parentPin !== parentPinConfirm) {
      setParentError(t('setup.parentPinMismatch'));
      return;
    }
    setParentError('');
    setStep('children');
  };

  const addChild = () => {
    if (!newName.trim()) return;
    const color = firstFreeColor([parentColor, ...children.map(c => c.color)]);
    setChildren([...children, { name: newName, theme: resolveSkin(''), color }]);
    setNewName('');
  };

  const removeChild = (index: number) => {
    setChildren(children.filter((_, i) => i !== index));
  };

  const updateChild = (index: number, patch: Partial<Child>) => {
    setChildren(children.map((c, i) => (i === index ? { ...c, ...patch } : c)));
  };

  const handleFinish = async () => {
    setLoading(true);
    setError('');
    try {
      await api.setup({
        parent: { name: parentName.trim() || t('setup.parentDefaultName'), pin: parentPin, color: parentColor },
        children: children.map(c => ({ name: c.name, theme: c.theme, color: c.color })),
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

      // Setup signs the parent in (session cookie); pick it up.
      await refresh();

      setStep('finish');
    } catch (err) {
      console.error(err);
      setError(t('setup.errorSetupFailed'));
    } finally {
      setLoading(false);
    }
  };

  const back = (to: Step) => (
    <Button variant="quiet" onClick={() => setStep(to)}>{t('setup.back')}</Button>
  );

  const renderStep = () => {
    switch (step) {
      case 'welcome':
        return (
          <div className={clsx(styles.step, styles.center)}>
            <span className={styles.bigMark} aria-hidden><BrandMark wordmark={false} /></span>
            <h1 className={styles.title}>{t('setup.welcomeTitle')}</h1>
            <p className={styles.lead}>{t('setup.welcomeDescription')}</p>
            <div className={styles.nav}>
              <Button icon="chev" onClick={() => setStep('parent')}>{t('setup.getStarted')}</Button>
            </div>
          </div>
        );

      case 'parent':
        return (
          <form className={styles.step} onSubmit={submitParent}>
            <h1 className={styles.title}>{t('setup.parentTitle')}</h1>
            <p className={styles.lead}>{t('setup.parentDescription')}</p>
            <div className={styles.fields}>
              <label className={styles.field}>
                <span>{t('setup.parentNamePlaceholder')}</span>
                <input
                  type="text"
                  value={parentName}
                  onChange={e => setParentName(e.target.value)}
                  autoComplete="given-name"
                />
              </label>
              <label className={styles.field}>
                <span>{t('setup.parentPinPlaceholder')}</span>
                <input
                  type="password"
                  inputMode="numeric"
                  autoComplete="new-password"
                  value={parentPin}
                  onChange={e => setParentPin(e.target.value.replace(/\D/g, '').slice(0, 8))}
                  required
                />
              </label>
              <label className={styles.field}>
                <span>{t('setup.parentPinConfirmPlaceholder')}</span>
                <input
                  type="password"
                  inputMode="numeric"
                  autoComplete="new-password"
                  value={parentPinConfirm}
                  onChange={e => setParentPinConfirm(e.target.value.replace(/\D/g, '').slice(0, 8))}
                  required
                />
              </label>
              <ColorPicker
                name="parent-color"
                legend={t('entry.setup.yourColor')}
                value={parentColor}
                onChange={setParentColor}
              />
            </div>
            {parentError && <p className={styles.error} role="alert">{parentError}</p>}
            <div className={styles.nav}>
              {back('welcome')}
              <Button type="submit" icon="chev">{t('setup.next')}</Button>
            </div>
          </form>
        );

      case 'children':
        return (
          <div className={styles.step}>
            <h1 className={styles.title}>{t('setup.childrenTitle')}</h1>
            <p className={styles.lead}>{t('setup.childrenDescription')}</p>

            <div className={styles.addRow}>
              <input
                type="text"
                className={styles.input}
                placeholder={t('setup.childNamePlaceholder')}
                aria-label={t('setup.childNamePlaceholder')}
                value={newName}
                onChange={e => setNewName(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && addChild()}
              />
              <button type="button" className={styles.addBtn} onClick={addChild} disabled={!newName.trim()}>
                <Icon name="plus" /> {t('setup.addButton')}
              </button>
            </div>

            {children.length > 0 && (
              <ul className={styles.people}>
                {children.map((c, i) => (
                  <li key={i} className={styles.person}>
                    <Avatar name={c.name} color={c.color} size="md" />
                    <span className={styles.personName}>{c.name}</span>
                    <button
                      type="button"
                      onClick={() => removeChild(i)}
                      className={styles.removeBtn}
                      aria-label={t('entry.setup.remove', { name: c.name })}
                    >
                      <Icon name="plus" />
                    </button>
                  </li>
                ))}
              </ul>
            )}

            <div className={styles.nav}>
              {back('parent')}
              <Button icon="chev" disabled={children.length === 0} onClick={() => setStep('looks')}>
                {t('setup.next')}
              </Button>
            </div>
          </div>
        );

      case 'looks':
        return (
          <div className={styles.step}>
            <h1 className={styles.title}>{t('entry.setup.looksTitle')}</h1>
            <p className={styles.lead}>{t('entry.setup.looksDescription')}</p>

            <div className={styles.looks}>
              {children.map((c, i) => (
                <section key={i} className={styles.lookCard} aria-labelledby={`look-${i}`}>
                  <h2 id={`look-${i}`} className={styles.lookName}>
                    <Avatar name={c.name} color={c.color} size="md" />
                    {c.name}
                  </h2>
                  <SkinPicker child={c} index={i} onChange={(theme) => updateChild(i, { theme })} />
                  <ColorPicker
                    name={`color-${i}`}
                    legend={t('entry.setup.color')}
                    value={c.color}
                    onChange={(color) => updateChild(i, { color })}
                  />
                </section>
              ))}
            </div>

            <div className={styles.nav}>
              {back('children')}
              <Button icon="chev" onClick={() => setStep('chores')}>{t('setup.next')}</Button>
            </div>
          </div>
        );

      case 'chores':
        return (
          <div className={styles.step}>
            <h1 className={styles.title}>{t('setup.choresTitle')}</h1>
            <p className={styles.lead}>{t('setup.choresDescription')}</p>

            <div className={styles.presets}>
              {CHORE_PRESETS.map((p, i) => {
                const cat = catFromCategory(p.category);
                const on = selectedPresets.includes(i);
                return (
                  <button
                    key={i}
                    type="button"
                    aria-pressed={on}
                    className={clsx(styles.preset, on && styles.presetOn)}
                    onClick={() => {
                      if (on) {
                        setSelectedPresets(selectedPresets.filter(idx => idx !== i));
                      } else {
                        setSelectedPresets([...selectedPresets, i]);
                      }
                    }}
                  >
                    <span className={styles.presetWell}><Icon name={resolveChoreIcon(p.icon, cat)} /></span>
                    <span className={styles.presetText}>
                      <span className={styles.presetTitle}>{p.title}</span>
                      <span className={styles.presetCat}>
                        <CategoryMark cat={cat} />
                        {t(`design.category.${cat}`)}
                      </span>
                    </span>
                    <span className={styles.presetCheck} aria-hidden>{on && <Icon name="check" />}</span>
                  </button>
                );
              })}
            </div>

            {error && <p className={styles.error} role="alert">{error}</p>}

            <div className={styles.nav}>
              {back('looks')}
              <Button onClick={handleFinish} disabled={loading}>
                {loading ? t('setup.settingUp') : t('setup.finishSetup')}
              </Button>
            </div>
          </div>
        );

      case 'finish':
        return (
          <div className={clsx(styles.step, styles.center)}>
            <span className={styles.doneMark} aria-hidden><Icon name="check" /></span>
            <h1 className={styles.title}>{t('setup.finishTitle')}</h1>
            <p className={styles.lead}>{t('setup.finishDescription')}</p>
            <div className={styles.nav}>
              <Button icon="chev" onClick={() => navigate('/admin/dashboard')}>
                {t('setup.goToDashboard')}
              </Button>
            </div>
          </div>
        );
    }
  };

  const index = STEPS.indexOf(step);

  return (
    <HouseScope persistent={session?.persistent ?? false} className={styles.screen}>
      <div className={styles.wrap}>
        <header className={styles.top}>
          <BrandMark />
          <span className={styles.stepOf}>
            {t('entry.setup.stepOf', { step: index + 1, total: STEPS.length })}
          </span>
        </header>
        <div
          className={styles.progress}
          role="progressbar"
          aria-label={t('entry.setup.progress')}
          aria-valuemin={1}
          aria-valuemax={STEPS.length}
          aria-valuenow={index + 1}
        >
          <i style={{ width: `${(index / (STEPS.length - 1)) * 100}%` }} />
        </div>
        <main className={styles.card}>{renderStep()}</main>
      </div>
    </HouseScope>
  );
};
