export interface LanguageOption {
  /** BCP-47 / i18next language code, e.g. 'en', 'fr'. */
  code: string;
  /** Native display name shown in the selector. */
  label: string;
}

// Single source of truth. Adding a language = add one entry here and drop a
// matching locales/<code>/translation.json file (see web/src/i18n/index.ts).
export const LANGUAGES: LanguageOption[] = [
  { code: 'en', label: 'English' },
];

export const SUPPORTED_CODES = LANGUAGES.map((l) => l.code);
export const FALLBACK_CODE = 'en';
