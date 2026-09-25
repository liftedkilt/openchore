import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { Pencil, Trash2, X } from 'lucide-react';
import { api, APIError } from '../../api';
import type { AdminAuthConfig, AdminAuthProvider, AuthProviderInput } from '../../types';
import { Icon } from '../../design';
import { FormMessage, type Msg } from './FormMessage';
import ui from './ui.module.css';
import styles from './SettingsTab.module.css';

const PROMPTS = ['', 'login', 'select_account', 'consent'] as const;

interface Draft {
  id: string;
  name: string;
  issuer: string;
  clientId: string;
  clientSecret: string; // blank keeps the saved secret when editing
  scopes: string;
  prompt: string;
}

const EMPTY: Draft = { id: '', name: '', issuer: '', clientId: '', clientSecret: '', scopes: '', prompt: 'login' };

/** "Pocket ID" -> "pocket-id": the provider id people see in the callback URL. */
export const slugifyProviderId = (name: string) =>
  name.toLowerCase().normalize('NFKD').replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 32);

const errorText = (err: unknown) => (err instanceof APIError ? err.message : '');

/** Single sign-on providers and session lengths. Anything set in config.yaml
 * or the OIDC_* environment variables is shown read-only. */
export const SignInSection: React.FC = () => {
  const { t } = useTranslation();
  const [cfg, setCfg] = useState<AdminAuthConfig | null>(null);
  const [editing, setEditing] = useState<string | null>(null); // provider id, '' = new
  const [draft, setDraft] = useState<Draft>(EMPTY);
  const [idTouched, setIdTouched] = useState(false);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<Msg>(null);
  const [testing, setTesting] = useState<string | null>(null);

  const [kioskHours, setKioskHours] = useState('');
  const [personalDays, setPersonalDays] = useState('');
  const [sessionSaving, setSessionSaving] = useState(false);
  const [sessionMessage, setSessionMessage] = useState<Msg>(null);

  const apply = (c: AdminAuthConfig) => {
    setCfg(c);
    setKioskHours(String(+c.kiosk_session.hours.toFixed(2)));
    setPersonalDays(String(+(c.personal_session.hours / 24).toFixed(2)));
  };

  useEffect(() => {
    api.admin.authConfig().then(apply).catch(() => {});
  }, []);

  if (!cfg) return null;

  const startAdd = () => {
    setEditing('');
    setDraft(EMPTY);
    setIdTouched(false);
    setMessage(null);
  };

  const startEdit = (p: AdminAuthProvider) => {
    setEditing(p.id);
    setDraft({
      id: p.id, name: p.name, issuer: p.issuer, clientId: p.client_id, clientSecret: '',
      scopes: p.scopes === 'openid profile email' ? '' : p.scopes, prompt: p.prompt,
    });
    setMessage(null);
  };

  const closeForm = () => setEditing(null);

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    if (editing === null) return;
    setSaving(true);
    setMessage(null);
    const body: AuthProviderInput = {
      name: draft.name.trim(),
      issuer: draft.issuer.trim(),
      client_id: draft.clientId.trim(),
      scopes: draft.scopes.trim(),
      prompt: draft.prompt,
    };
    if (draft.clientSecret.trim()) body.client_secret = draft.clientSecret.trim();
    try {
      if (editing === '') {
        apply(await api.admin.createAuthProvider({ ...body, id: draft.id.trim() }));
      } else {
        apply(await api.admin.updateAuthProvider(editing, body));
      }
      setEditing(null);
      setMessage({ type: 'success', text: t('admin.settingsTab.signIn.saveSuccess', { name: body.name || draft.id }) });
    } catch (err) {
      setMessage({ type: 'error', text: t('admin.settingsTab.signIn.saveError', { detail: errorText(err) }) });
    }
    setSaving(false);
  };

  const handleDelete = async (p: AdminAuthProvider) => {
    const confirmText = p.linked_accounts > 0
      ? t('admin.settingsTab.signIn.confirmRemoveLinked', { name: p.name, count: p.linked_accounts })
      : t('admin.settingsTab.signIn.confirmRemove', { name: p.name });
    if (!window.confirm(confirmText)) return;
    setMessage(null);
    try {
      apply(await api.admin.deleteAuthProvider(p.id));
      if (editing === p.id) setEditing(null);
      setMessage({ type: 'success', text: t('admin.settingsTab.signIn.removed', { name: p.name }) });
    } catch (err) {
      setMessage({ type: 'error', text: t('admin.settingsTab.signIn.removeError', { detail: errorText(err) }) });
    }
  };

  const handleTest = async (p: AdminAuthProvider) => {
    setTesting(p.id);
    setMessage(null);
    try {
      await api.admin.testAuthProvider(p.id);
      setMessage({ type: 'success', text: t('admin.settingsTab.signIn.testSuccess', { name: p.name }) });
    } catch (err) {
      setMessage({ type: 'error', text: t('admin.settingsTab.signIn.testError', { name: p.name, detail: errorText(err) }) });
    }
    setTesting(null);
  };

  const handleSaveSessions = async (e: React.FormEvent) => {
    e.preventDefault();
    setSessionSaving(true);
    setSessionMessage(null);
    // Blank, zero or the default all mean "use the default", so the
    // setting keeps following it.
    const hours = (v: number, def: number) => (Number.isFinite(v) && v > 0 && v !== def ? v : null);
    try {
      apply(await api.admin.updateSessionLengths({
        kiosk_session_hours: hours(parseFloat(kioskHours), cfg.kiosk_session.default_hours),
        personal_session_hours: hours(parseFloat(personalDays) * 24, cfg.personal_session.default_hours),
      }));
      setSessionMessage({ type: 'success', text: t('admin.settingsTab.signIn.sessions.saveSuccess') });
    } catch (err) {
      setSessionMessage({ type: 'error', text: t('admin.settingsTab.signIn.sessions.saveError', { detail: errorText(err) }) });
    }
    setSessionSaving(false);
  };

  const editingProvider = cfg.providers.find(p => p.id === editing);
  const callbackPreview = cfg.callback_base + (draft.id || '…') + '/callback';

  const form = editing !== null && (
    <form onSubmit={handleSave} className={ui.inset} aria-label={editing === '' ? t('admin.settingsTab.signIn.addTitle') : t('admin.settingsTab.signIn.editTitle', { name: editingProvider?.name })}>
      <label className={ui.field}>
        <span className={ui.label}>{t('admin.settingsTab.signIn.form.name')}</span>
        <input
          className={ui.input}
          value={draft.name}
          onChange={e => {
            const name = e.target.value;
            setDraft(d => ({ ...d, name, id: editing === '' && !idTouched ? slugifyProviderId(name) : d.id }));
          }}
          placeholder="Pocket ID"
          required
          autoFocus
        />
      </label>
      <label className={ui.field}>
        <span className={ui.label}>{t('admin.settingsTab.signIn.form.id')}</span>
        <input
          className={ui.input}
          value={draft.id}
          onChange={e => { setIdTouched(true); setDraft(d => ({ ...d, id: e.target.value })); }}
          pattern="[a-z0-9][a-z0-9_\-]{0,31}"
          disabled={editing !== ''}
          required
        />
        <span className={ui.help}>{t('admin.settingsTab.signIn.form.idHelp')}</span>
      </label>
      <div className={ui.field}>
        <span className={ui.label}>{t('admin.settingsTab.signIn.form.redirectUri')}</span>
        <code className={clsx(ui.code, styles.wrapCode)}>{callbackPreview}</code>
        <span className={ui.help}>{t('admin.settingsTab.signIn.form.redirectUriHelp')}</span>
      </div>
      <label className={ui.field}>
        <span className={ui.label}>{t('admin.settingsTab.signIn.form.issuer')}</span>
        <input
          className={ui.input}
          type="url"
          value={draft.issuer}
          onChange={e => setDraft(d => ({ ...d, issuer: e.target.value }))}
          placeholder="https://id.example.com"
          required
        />
        {editingProvider && editingProvider.linked_accounts > 0 && draft.issuer.trim().replace(/\/+$/, '') !== editingProvider.issuer && (
          <span className={ui.msgWaiting}>{t('admin.settingsTab.signIn.form.issuerChangeWarning', { count: editingProvider.linked_accounts })}</span>
        )}
      </label>
      <label className={ui.field}>
        <span className={ui.label}>{t('admin.settingsTab.signIn.form.clientId')}</span>
        <input
          className={ui.input}
          value={draft.clientId}
          onChange={e => setDraft(d => ({ ...d, clientId: e.target.value }))}
          required
        />
      </label>
      <label className={ui.field}>
        <span className={ui.label}>{t('admin.settingsTab.signIn.form.clientSecret')}</span>
        <input
          className={ui.input}
          type="password"
          autoComplete="off"
          value={draft.clientSecret}
          onChange={e => setDraft(d => ({ ...d, clientSecret: e.target.value }))}
          placeholder={editingProvider?.client_secret_set ? t('admin.settingsTab.signIn.form.clientSecretSaved') : ''}
        />
      </label>
      <label className={ui.field}>
        <span className={ui.label}>{t('admin.settingsTab.signIn.form.scopes')}</span>
        <input
          className={ui.input}
          value={draft.scopes}
          onChange={e => setDraft(d => ({ ...d, scopes: e.target.value }))}
          placeholder="openid profile email"
        />
      </label>
      <label className={ui.field}>
        <span className={ui.label}>{t('admin.settingsTab.signIn.form.prompt')}</span>
        <select className={ui.input} value={draft.prompt} onChange={e => setDraft(d => ({ ...d, prompt: e.target.value }))}>
          {PROMPTS.map(p => (
            <option key={p} value={p}>{t(`admin.settingsTab.signIn.form.prompts.${p || 'default'}`)}</option>
          ))}
        </select>
        <span className={ui.help}>{t('admin.settingsTab.signIn.form.promptHelp')}</span>
      </label>
      <div className={ui.actionsEnd}>
        <button type="button" className={ui.btnGhost} onClick={closeForm}>{t('admin.settingsTab.signIn.form.cancel')}</button>
        <button type="submit" className={ui.btnPrimary} disabled={saving}>
          <Icon name="check" /> {editing === '' ? t('admin.settingsTab.signIn.form.add') : t('admin.settingsTab.signIn.form.save')}
        </button>
      </div>
    </form>
  );

  const sessionField = (
    label: string,
    help: string,
    value: string,
    set: (v: string) => void,
    unit: string,
    fromConfig: boolean,
  ) => (
    <label className={ui.field}>
      <span className={ui.label}>{label}</span>
      <span className={styles.rangeRow}>
        <input
          className={clsx(ui.input, styles.numberInput)}
          type="number"
          min="0"
          step="any"
          value={value}
          onChange={e => set(e.target.value)}
          disabled={fromConfig}
        />
        <span>{unit}</span>
      </span>
      <span className={ui.help}>{fromConfig ? t('admin.settingsTab.signIn.sessions.fromConfig') : help}</span>
    </label>
  );

  return (
    <section className={clsx(ui.card, ui.section)} aria-labelledby="settings-sign-in">
      <div className={ui.sectionHead}>
        <h3 className={ui.sectionTitle} id="settings-sign-in"><Icon name="lock" /> {t('admin.settingsTab.signIn.title')}</h3>
        <button
          type="button"
          className={ui.btnGhost}
          aria-expanded={editing === ''}
          onClick={() => (editing === '' ? closeForm() : startAdd())}
        >
          {editing === '' ? <X aria-hidden /> : <Icon name="plus" />}
          {editing === '' ? t('admin.settingsTab.signIn.form.cancel') : t('admin.settingsTab.signIn.addButton')}
        </button>
      </div>
      <p className={ui.sectionDesc}>{t('admin.settingsTab.signIn.description')}</p>
      <p className={ui.sectionDesc}>{t('admin.settingsTab.signIn.providersIntro')}</p>

      {editing === '' && form}

      {cfg.providers.length === 0 && editing !== '' && (
        <p className={ui.emptyInline}>{t('admin.settingsTab.signIn.noProviders')}</p>
      )}

      {cfg.providers.length > 0 && (
        <div className={styles.subList}>
          {cfg.providers.map(p => (
            <div key={p.id} className={styles.webhook}>
              <div className={ui.rowMain}>
                <span className={ui.rowTitle}>{p.name}</span>
                <span className={ui.rowMeta}>
                  <code className={ui.code}>{p.id}</code>
                  <span className={styles.url}>{p.issuer}</span>
                  <span>{t('admin.settingsTab.signIn.linkedAccounts', { count: p.linked_accounts })}</span>
                  {p.source === 'config' && <span className={ui.badgeOutline}>{t('admin.settingsTab.signIn.fromConfig')}</span>}
                </span>
              </div>
              <div className={styles.webhookActions}>
                <button type="button" className={ui.btnGhost} disabled={testing === p.id} onClick={() => handleTest(p)}>
                  {testing === p.id ? t('admin.settingsTab.signIn.testing') : t('admin.settingsTab.signIn.testButton')}
                </button>
                {p.source === 'settings' && (
                  <>
                    <button type="button" className={ui.btnGhost} aria-expanded={editing === p.id} onClick={() => (editing === p.id ? closeForm() : startEdit(p))}>
                      <Pencil aria-hidden /> {t('admin.settingsTab.signIn.editButton')}
                    </button>
                    <button
                      type="button"
                      className={clsx(ui.iconBtn, ui.iconBtnDanger)}
                      aria-label={t('admin.settingsTab.signIn.removeAriaLabel', { name: p.name })}
                      title={t('admin.settingsTab.signIn.removeAriaLabel', { name: p.name })}
                      onClick={() => handleDelete(p)}
                    >
                      <Trash2 aria-hidden />
                    </button>
                  </>
                )}
              </div>
              {editing === p.id && form}
            </div>
          ))}
        </div>
      )}

      <FormMessage msg={message} />

      <form onSubmit={handleSaveSessions} className={styles.sessions}>
        <h4 className={ui.eyebrow}>{t('admin.settingsTab.signIn.sessions.title')}</h4>
        {sessionField(
          t('admin.settingsTab.signIn.sessions.kioskLabel'),
          t('admin.settingsTab.signIn.sessions.kioskHelp', { hours: cfg.kiosk_session.default_hours }),
          kioskHours, setKioskHours, t('admin.settingsTab.signIn.sessions.hours'), cfg.kiosk_session.from_config,
        )}
        {sessionField(
          t('admin.settingsTab.signIn.sessions.personalLabel'),
          t('admin.settingsTab.signIn.sessions.personalHelp', { days: cfg.personal_session.default_hours / 24 }),
          personalDays, setPersonalDays, t('admin.settingsTab.signIn.sessions.days'), cfg.personal_session.from_config,
        )}
        <FormMessage msg={sessionMessage} />
        {!(cfg.kiosk_session.from_config && cfg.personal_session.from_config) && (
          <div className={ui.actionsEnd}>
            <button type="submit" className={ui.btnPrimary} disabled={sessionSaving}>
              <Icon name="check" /> {t('admin.settingsTab.signIn.sessions.saveButton')}
            </button>
          </div>
        )}
      </form>
    </section>
  );
};
