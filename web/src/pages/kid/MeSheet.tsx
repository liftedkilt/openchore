import React, { useId, useRef, useState } from 'react';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import {
  Avatar, CategoryMark, Icon, PERSON_COLORS, SKINS, SkinScope, isPersonColor,
  type PersonColor, type Skin,
} from '../../design';
import { LANGUAGES } from '../../i18n/languages';
import type { User } from '../../types';
import { Sheet } from './Sheet';
import s from './kid.module.css';

const AVATAR_STYLES = [
  'avataaars-neutral', 'adventurer-neutral', 'big-ears-neutral', 'bottts-neutral', 'fun-emoji', 'lorelei-neutral',
  'croodles-neutral', 'pixel-art-neutral', 'thumbs', 'notionists-neutral', 'shapes', 'glass',
] as const;

const avatarUrl = (style: string, seed: string) =>
  `https://api.dicebear.com/9.x/${style}/svg?seed=${encodeURIComponent(seed)}&backgroundColor=transparent`;

interface MeSheetProps {
  user: User;
  skin: Skin;
  isAdmin: boolean;
  tts: boolean;
  onClose: () => void;
  onSkin: (skin: Skin) => void;
  onColor: (color: PersonColor) => void;
  onAvatar: (url: string) => Promise<void>;
  onTts: (on: boolean) => void;
  onPin: () => void;
  onLinked: () => void;
  onManage: () => void;
  onSignOut: () => void;
}

/** Arrow keys move through a radio group and select, as native radios do. */
function useRovingRadios<T>(values: readonly T[], current: T, select: (v: T) => void) {
  const refs = useRef<(HTMLElement | null)[]>([]);
  const onKeyDown = (e: React.KeyboardEvent, i: number) => {
    const step = e.key === 'ArrowRight' || e.key === 'ArrowDown' ? 1 : e.key === 'ArrowLeft' || e.key === 'ArrowUp' ? -1 : 0;
    if (step) {
      e.preventDefault();
      const next = (i + step + values.length) % values.length;
      refs.current[next]?.focus();
      select(values[next]);
    } else if (e.key === ' ' || e.key === 'Enter') {
      e.preventDefault();
      select(values[i]);
    }
  };
  const tabIndex = (v: T) => (v === current || (!values.includes(current) && v === values[0]) ? 0 : -1);
  return { refs, onKeyDown, tabIndex };
}

/** "Me": skin, colour, picture, read-aloud, language and account. */
export function MeSheet(props: MeSheetProps) {
  const { user, skin, isAdmin, tts, onClose, onSkin, onColor, onTts, onPin, onLinked, onManage, onSignOut } = props;
  const { t, i18n } = useTranslation();
  const [picking, setPicking] = useState(false);
  const color = isPersonColor(user.color) ? user.color : null;
  const skinHead = useId();
  const colorHead = useId();
  const langId = useId();
  const skins = useRovingRadios(SKINS, skin, onSkin);
  const colors = useRovingRadios(PERSON_COLORS, color as PersonColor, onColor);

  if (picking) {
    return <AvatarPicker user={user} onBack={() => setPicking(false)} onClose={onClose} onSave={props.onAvatar} />;
  }

  return (
    <Sheet title={t('kid.me.title')} onClose={onClose}>
      <div className={s.meHead}>
        <Avatar name={user.name} color={color} size="lg" src={user.avatar_url || null} />
        <div className={s.meWho}>
          <span className={s.meName}>{user.name}</span>
          <button type="button" className={s.linkBtn} onClick={() => setPicking(true)}>
            {t('kid.me.changePicture')}
          </button>
        </div>
      </div>

      <h3 id={skinHead} className={s.meSection}>{t('kid.me.skin')}</h3>
      <div role="radiogroup" aria-labelledby={skinHead} className={s.doors}>
        {SKINS.map((x, i) => (
          <div
            key={x}
            ref={(el) => { skins.refs.current[i] = el; }}
            role="radio"
            aria-checked={x === skin}
            aria-label={t(`kid.skins.${x}.name`)}
            aria-describedby={`${skinHead}-${x}`}
            tabIndex={skins.tabIndex(x)}
            className={clsx(s.doorBtn, x === skin && s.doorOn)}
            onClick={() => onSkin(x)}
            onKeyDown={(e) => skins.onKeyDown(e, i)}
          >
            {/* A door: a small preview drawn in that skin, as on the family picker. */}
            <SkinScope skin={x} color={color} glow={false} className={s.door}>
              <span className={s.doorTop}>
                <Avatar name={user.name} color={color} size="sm" />
                <span className={s.doorCheck} aria-hidden>{x === skin && <Icon name="check" />}</span>
              </span>
              <span className={s.doorName} aria-hidden>{t(`kid.skins.${x}.name`)}</span>
              <span className={s.doorMarks} aria-hidden>
                <CategoryMark cat="essential" done />
                <CategoryMark cat="daily" done />
                <CategoryMark cat="bonus" />
              </span>
              <span className={s.doorHint} id={`${skinHead}-${x}`}>{t(`kid.skins.${x}.hint`)}</span>
            </SkinScope>
          </div>
        ))}
      </div>

      <h3 id={colorHead} className={s.meSection}>{t('kid.me.color')}</h3>
      <div role="radiogroup" aria-labelledby={colorHead} className={s.swatches}>
        {PERSON_COLORS.map((c, i) => (
          <button
            key={c}
            ref={(el) => { colors.refs.current[i] = el; }}
            type="button"
            role="radio"
            aria-checked={c === color}
            aria-label={t(`kid.colors.${c}`)}
            tabIndex={colors.tabIndex(c)}
            data-person={c}
            className={clsx(s.swatch, c === color && s.swatchOn)}
            onClick={() => onColor(c)}
            onKeyDown={(e) => colors.onKeyDown(e, i)}
          >
            {c === color && <Icon name="check" />}
          </button>
        ))}
      </div>

      <h3 className={s.meSection}>{t('kid.me.settings')}</h3>
      <div className={s.settingsList}>
        <button
          type="button"
          role="switch"
          aria-checked={tts}
          className={s.settingRow}
          onClick={() => onTts(!tts)}
        >
          <Icon name="sound" />
          <span className={s.settingText}>
            {t('kid.me.readAloud')}
            <small>{t('kid.me.readAloudHint')}</small>
          </span>
          <span className={clsx(s.switch, tts && s.switchOn)} aria-hidden><i /></span>
        </button>

        <label className={s.settingRow} htmlFor={langId}>
          <Icon name="book" />
          <span className={s.settingText}>{t('kid.me.language')}</span>
          <select
            id={langId}
            className={s.select}
            value={i18n.resolvedLanguage}
            onChange={(e) => i18n.changeLanguage(e.target.value)}
          >
            {LANGUAGES.map(l => <option key={l.code} value={l.code}>{l.label}</option>)}
          </select>
        </label>

        <button type="button" className={s.settingRow} onClick={onPin}>
          <Icon name="lock" />
          <span className={s.settingText}>{user.has_pin ? t('kid.me.changePin') : t('kid.me.setPin')}</span>
          <Icon name="chev" />
        </button>

        <button type="button" className={s.settingRow} onClick={onLinked}>
          <Icon name="people" />
          <span className={s.settingText}>{t('kid.me.linkedAccounts')}</span>
          <Icon name="chev" />
        </button>

        {isAdmin && (
          <button type="button" className={s.settingRow} onClick={onManage}>
            <Icon name="home" />
            <span className={s.settingText}>{t('kid.me.manage')}<small>{t('kid.me.manageHint')}</small></span>
            <Icon name="chev" />
          </button>
        )}
      </div>

      <button type="button" className={s.signOut} onClick={onSignOut}>
        <Icon name="back" />
        {t('kid.me.signOut')}
      </button>
    </Sheet>
  );
}

function AvatarPicker({ user, onBack, onClose, onSave }: {
  user: User;
  onBack: () => void;
  onClose: () => void;
  onSave: (url: string) => Promise<void>;
}) {
  const { t } = useTranslation();
  const current = user.avatar_url || '';
  const [style, setStyle] = useState<string>(() => current.match(/dicebear\.com\/\d+\.x\/([^/]+)\//)?.[1] || AVATAR_STYLES[0]);
  const [seed, setSeed] = useState(user.name);
  const [saving, setSaving] = useState(false);
  const preview = avatarUrl(style, seed);
  const color = isPersonColor(user.color) ? user.color : null;

  const save = async () => {
    setSaving(true);
    try {
      await onSave(preview);
      onBack();
    } finally {
      setSaving(false);
    }
  };

  return (
    <Sheet
      title={t('kid.me.pictureTitle')}
      onClose={onClose}
      leading={(
        <button type="button" className={s.backBtn} onClick={onBack} aria-label={t('kid.sheet.back')}>
          <Icon name="back" />
        </button>
      )}
    >
      <div className={s.pickerPreview}>
        <Avatar name={user.name} color={color} size="lg" src={preview} />
      </div>
      <div className={s.pickerGrid} role="radiogroup" aria-label={t('kid.me.pictureStyles')}>
        {AVATAR_STYLES.map((st, i) => (
          <button
            key={st}
            type="button"
            role="radio"
            aria-checked={st === style}
            aria-label={t('kid.me.pictureStyle', { n: i + 1 })}
            className={clsx(s.pickerItem, st === style && s.pickerItemOn)}
            onClick={() => setStyle(st)}
          >
            <Avatar name={user.name} color={color} size="md" src={avatarUrl(st, seed)} />
          </button>
        ))}
      </div>
      <div className={s.sheetActions}>
        <button type="button" className={s.secondaryAction} onClick={() => setSeed(`${user.name}-${Math.random().toString(36).slice(2, 8)}`)}>
          {t('kid.me.shuffle')}
        </button>
        <button type="button" className={s.primaryAction} onClick={save} disabled={saving || preview === current}>
          {t('kid.me.savePicture')}
        </button>
      </div>
    </Sheet>
  );
}
