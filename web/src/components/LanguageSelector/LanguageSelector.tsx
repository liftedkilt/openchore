import React from 'react';
import { useTranslation } from 'react-i18next';
import { LANGUAGES } from '../../i18n/languages';
import styles from './LanguageSelector.module.css';

interface Props {
  className?: string;
}

export const LanguageSelector: React.FC<Props> = ({ className }) => {
  const { i18n, t } = useTranslation();

  return (
    <select
      className={[styles.select, className].filter(Boolean).join(' ')}
      value={i18n.resolvedLanguage}
      onChange={(e) => i18n.changeLanguage(e.target.value)}
      aria-label={t('common.language', 'Language')}
    >
      {LANGUAGES.map((lng) => (
        <option key={lng.code} value={lng.code}>
          {lng.label}
        </option>
      ))}
    </select>
  );
};

export default LanguageSelector;
