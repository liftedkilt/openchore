import React from 'react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { Icon } from '../../design';
import { LANGUAGES } from '../../i18n/languages';
import styles from './LanguageSelector.module.css';

interface Props {
  /** Extra class for the select itself. */
  className?: string;
}

export const LanguageSelector: React.FC<Props> = ({ className }) => {
  const { i18n, t } = useTranslation();

  return (
    <span className={styles.wrap}>
      <select
        className={clsx(styles.select, className)}
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
      <Icon name="chev" className={styles.chev} />
    </span>
  );
};

export default LanguageSelector;
