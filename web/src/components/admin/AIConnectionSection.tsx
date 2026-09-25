import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { api, APIError } from '../../api';
import type { AIConnection, AIConnectionUpdate } from '../../types';
import { Icon } from '../../design';
import { refreshAIStatus } from '../../hooks/useAIStatus';
import { FormMessage, type Msg } from './FormMessage';
import ui from './ui.module.css';
import styles from './SettingsTab.module.css';

interface Draft {
  baseUrl: string;
  model: string;
  apiKey: string; // blank keeps the saved key
  clearKey: boolean;
}

const toDraft = (c: AIConnection): Draft => ({ baseUrl: c.base_url, model: c.model, apiKey: '', clearKey: false });

const toUpdate = (d: Draft): AIConnectionUpdate => {
  const u: AIConnectionUpdate = { base_url: d.baseUrl.trim(), model: d.model.trim() };
  if (d.apiKey.trim()) u.api_key = d.apiKey.trim();
  else if (d.clearKey) u.api_key = '';
  return u;
};

/** Where the AI model and speech service live. Settings made through
 * AI_BASE_URL / TTS_BASE_URL are shown read-only. */
export const AIConnectionSection: React.FC = () => {
  const { t } = useTranslation();
  const [saved, setSaved] = useState<{ ai: AIConnection; tts: AIConnection } | null>(null);
  const [ai, setAi] = useState<Draft | null>(null);
  const [tts, setTts] = useState<Draft | null>(null);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<Msg>(null);

  const apply = (c: { ai: AIConnection; tts: AIConnection }) => {
    setSaved(c);
    setAi(toDraft(c.ai));
    setTts(toDraft(c.tts));
  };

  useEffect(() => {
    api.admin.aiConfig().then(apply).catch(() => {});
  }, []);

  if (!saved || !ai || !tts) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setMessage(null);
    try {
      const body: { ai?: AIConnectionUpdate; tts?: AIConnectionUpdate } = {};
      if (!saved.ai.from_env) body.ai = toUpdate(ai);
      if (!saved.tts.from_env) body.tts = toUpdate(tts);
      apply(await api.admin.updateAIConfig(body));
      refreshAIStatus();
      setMessage({ type: 'success', text: t('admin.settingsTab.aiConnection.saveSuccess') });
    } catch (err) {
      const detail = err instanceof APIError ? err.message : '';
      setMessage({ type: 'error', text: t('admin.settingsTab.aiConnection.saveError', { detail }) });
    }
    setSaving(false);
  };

  const fields = (
    kind: 'ai' | 'tts',
    conn: AIConnection,
    draft: Draft,
    set: (d: Draft) => void,
  ) => {
    const k = `admin.settingsTab.aiConnection.${kind}`;
    if (conn.from_env) {
      return (
        <fieldset className={styles.connection}>
          <legend className={ui.label}>{t(`${k}.title`)}</legend>
          <p className={ui.help}>{t(`${k}.fromEnv`)}</p>
          <p className={ui.rowMeta}>
            <code className={ui.code}>{conn.base_url}</code>
            {conn.model && <span>{conn.model}</span>}
          </p>
        </fieldset>
      );
    }
    return (
      <fieldset className={styles.connection}>
        <legend className={ui.label}>{t(`${k}.title`)}</legend>
        <span className={ui.help}>{t(`${k}.help`)}</span>
        <label className={ui.field}>
          <span className={ui.label}>{t('admin.settingsTab.aiConnection.baseUrlLabel')}</span>
          <input
            className={ui.input}
            type="url"
            value={draft.baseUrl}
            onChange={e => set({ ...draft, baseUrl: e.target.value })}
            placeholder={t(`${k}.baseUrlPlaceholder`)}
          />
        </label>
        <label className={ui.field}>
          <span className={ui.label}>{t('admin.settingsTab.aiConnection.modelLabel')}</span>
          <input
            className={ui.input}
            value={draft.model}
            onChange={e => set({ ...draft, model: e.target.value })}
            placeholder={t(`${k}.modelPlaceholder`)}
            required={kind === 'ai' && !!draft.baseUrl.trim()}
          />
        </label>
        <label className={ui.field}>
          <span className={ui.label}>{t('admin.settingsTab.aiConnection.apiKeyLabel')}</span>
          <input
            className={ui.input}
            type="password"
            autoComplete="off"
            value={draft.apiKey}
            onChange={e => set({ ...draft, apiKey: e.target.value, clearKey: false })}
            placeholder={conn.api_key_set
              ? t('admin.settingsTab.aiConnection.apiKeySavedPlaceholder')
              : t('admin.settingsTab.aiConnection.apiKeyPlaceholder')}
          />
        </label>
        {conn.api_key_set && !draft.apiKey && (
          <label className={ui.check}>
            <input type="checkbox" checked={draft.clearKey} onChange={e => set({ ...draft, clearKey: e.target.checked })} />
            <span className={ui.checkLabel}>{t('admin.settingsTab.aiConnection.clearKey')}</span>
          </label>
        )}
      </fieldset>
    );
  };

  const allFromEnv = saved.ai.from_env && saved.tts.from_env;

  return (
    <form className={clsx(ui.card, ui.section)} onSubmit={handleSubmit} aria-labelledby="settings-ai-connection">
      <h3 className={ui.sectionTitle} id="settings-ai-connection"><Icon name="spark" /> {t('admin.settingsTab.aiConnection.title')}</h3>
      <p className={ui.sectionDesc}>{t('admin.settingsTab.aiConnection.description')}</p>
      {fields('ai', saved.ai, ai, setAi)}
      {fields('tts', saved.tts, tts, setTts)}
      <FormMessage msg={message} />
      {!allFromEnv && (
        <div className={ui.actionsEnd}>
          <button type="submit" className={ui.btnPrimary} disabled={saving}>
            <Icon name="check" /> {saving ? t('admin.settingsTab.aiConnection.savingButton') : t('admin.settingsTab.aiConnection.saveButton')}
          </button>
        </div>
      )}
    </form>
  );
};
