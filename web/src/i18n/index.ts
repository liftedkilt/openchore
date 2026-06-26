import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';
import { SUPPORTED_CODES, FALLBACK_CODE } from './languages';
import en from './locales/en/translation.json';
import de from './locales/de/translation.json';

// resources is keyed by language code; add new languages alongside their
// LANGUAGES entry. Each maps to a single `translation` namespace.
const resources = {
  en: { translation: en },
  de: { translation: de },
} as const;

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    supportedLngs: SUPPORTED_CODES,
    fallbackLng: FALLBACK_CODE,
    nonExplicitSupportedLngs: true, // 'en-US' -> 'en'
    returnNull: false,
    interpolation: { escapeValue: false }, // React already escapes
    detection: {
      order: ['localStorage', 'navigator'],
      lookupLocalStorage: 'openchore_lang',
      caches: ['localStorage'],
    },
  });

export default i18n;
