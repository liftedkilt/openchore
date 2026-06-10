// @vitest-environment jsdom
import { describe, it, expect, beforeAll } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import i18n from '../../i18n';
import { LanguageSelector } from './LanguageSelector';

beforeAll(async () => {
  if (!i18n.isInitialized) await i18n.init();
  await i18n.changeLanguage('en');
});

describe('LanguageSelector', () => {
  it('renders an option per supported language', () => {
    render(<LanguageSelector />);
    expect(screen.getByRole('option', { name: 'English' })).toBeDefined();
  });

  it('changes the active language on selection', async () => {
    // Inject a second language so selection is observable.
    i18n.addResourceBundle('zz', 'translation', { common: { back: 'Zz' } });
    render(<LanguageSelector />);
    const select = screen.getByRole('combobox') as HTMLSelectElement;
    await userEvent.selectOptions(select, 'en');
    expect(i18n.resolvedLanguage).toBe('en');
  });
});
