import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { Trash2, Copy, KeyRound, X } from 'lucide-react';
import { api } from '../../api';
import type { APIToken } from '../../types';
import { Icon } from '../../design';
import ui from './ui.module.css';
import styles from './SettingsTab.module.css';
import local from './APITokensSection.module.css';

export const APITokensSection: React.FC = () => {
  const { t, i18n } = useTranslation();
  const [tokens, setTokens] = useState<APIToken[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [tokenName, setTokenName] = useState('');
  const [creating, setCreating] = useState(false);
  const [newToken, setNewToken] = useState<{ name: string; token: string } | null>(null);
  const [copied, setCopied] = useState(false);

  const loadTokens = useCallback(async () => {
    try {
      const tokenList = await api.tokens.list();
      setTokens(tokenList);
    } catch (e) { console.error(e); }
  }, []);

  useEffect(() => { loadTokens(); }, [loadTokens]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!tokenName.trim()) return;
    setCreating(true);
    try {
      const result = await api.tokens.create(tokenName.trim());
      setNewToken({ name: result.name, token: result.token });
      setTokenName('');
      setShowForm(false);
      loadTokens();
    } catch (e) { console.error(e); }
    setCreating(false);
  };

  const handleRevoke = async (id: number) => {
    try {
      await api.tokens.revoke(id);
      loadTokens();
    } catch (e) { console.error(e); }
  };

  const handleCopy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      // Fallback for older browsers
      const textarea = document.createElement('textarea');
      textarea.value = text;
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand('copy');
      document.body.removeChild(textarea);
    }
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const date = (s: string) => new Date(s).toLocaleDateString(i18n.language);
  const activeTokens = tokens.filter(tok => !tok.revoked);
  const revokedTokens = tokens.filter(tok => tok.revoked);

  return (
    <section className={clsx(ui.card, ui.section)}>
      <div className={ui.sectionHead}>
        <h3 className={ui.sectionTitle}><KeyRound aria-hidden className={local.titleIcon} /> {t('admin.apiTokens.heading')}</h3>
        <button
          type="button"
          className={ui.btnGhost}
          aria-expanded={showForm}
          onClick={() => { setShowForm(f => !f); setNewToken(null); }}
        >
          {showForm ? <X aria-hidden /> : <Icon name="plus" />}
          {showForm ? t('admin.apiTokens.cancel') : t('admin.apiTokens.addButton')}
        </button>
      </div>
      <p className={ui.sectionDesc}>{t('admin.apiTokens.description')}</p>

      {/* New token reveal */}
      {newToken && (
        <div className={local.reveal} role="status">
          <p className={ui.msgWaiting}><Icon name="lock" /> {t('admin.apiTokens.copyNowWarning')}</p>
          <p className={ui.help}>{t('admin.apiTokens.tokenFor', { name: newToken.name })}</p>
          <div className={local.tokenRow}>
            <code className={local.token}>{newToken.token}</code>
            <button type="button" className={ui.btn} onClick={() => handleCopy(newToken.token)}>
              {copied ? <><Icon name="check" /> {t('admin.apiTokens.copied')}</> : <><Copy aria-hidden /> {t('admin.apiTokens.copy')}</>}
            </button>
          </div>
          <div>
            <button type="button" className={ui.btnGhost} onClick={() => setNewToken(null)}>
              {t('admin.apiTokens.dismiss')}
            </button>
          </div>
        </div>
      )}

      {showForm && (
        <form onSubmit={handleCreate} className={ui.inset}>
          <label className={ui.field}>
            <span className={ui.label}>{t('admin.apiTokens.tokenNameLabel')}</span>
            <input
              className={ui.input}
              value={tokenName}
              onChange={e => setTokenName(e.target.value)}
              placeholder={t('admin.apiTokens.tokenNamePlaceholder')}
              required
              autoFocus
            />
          </label>
          <div className={ui.actionsEnd}>
            <button type="submit" className={ui.btnPrimary} disabled={creating || !tokenName.trim()}>
              <Icon name="check" /> {creating ? t('admin.apiTokens.creating') : t('admin.apiTokens.createToken')}
            </button>
          </div>
        </form>
      )}

      {activeTokens.length === 0 && revokedTokens.length === 0 && !showForm && (
        <p className={ui.emptyInline}>{t('admin.apiTokens.noTokens')}</p>
      )}

      {activeTokens.length > 0 && (
        <div className={styles.subList}>
          {activeTokens.map(tok => (
            <div key={tok.id} className={local.tokenItem}>
              <div className={ui.rowMain}>
                <span className={ui.rowTitle}>{tok.name}</span>
                <span className={ui.rowMeta}>
                  <span>{t('admin.apiTokens.created', { date: date(tok.created_at) })}</span>
                  <span>{tok.last_used_at ? t('admin.apiTokens.lastUsed', { date: date(tok.last_used_at) }) : t('admin.apiTokens.neverUsed')}</span>
                </span>
              </div>
              <button type="button" className={ui.btnDanger} onClick={() => handleRevoke(tok.id)}>
                <Trash2 aria-hidden /> {t('admin.apiTokens.revoke')}
              </button>
            </div>
          ))}
        </div>
      )}

      {revokedTokens.length > 0 && (
        <>
          <h4 className={ui.eyebrow}>{t('admin.apiTokens.revokedLabel')}</h4>
          <div className={clsx(styles.subList, ui.rowMuted)}>
            {revokedTokens.map(tok => (
              <div key={tok.id} className={local.tokenItem}>
                <div className={ui.rowMain}>
                  <span className={clsx(ui.rowTitle, local.struck)}>{tok.name}</span>
                  <span className={ui.rowMeta}>
                    <span>{t('admin.apiTokens.created', { date: date(tok.created_at) })}</span>
                    <span className={ui.badgeOutline}>{t('admin.apiTokens.revokedStatus')}</span>
                  </span>
                </div>
              </div>
            ))}
          </div>
        </>
      )}
    </section>
  );
};
