import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../../api';
import type { APIToken } from '../../types';
import styles from '../../pages/AdminDashboard.module.css';
import { Plus, Trash2, Check, Copy, Key, AlertTriangle } from 'lucide-react';
import clsx from 'clsx';

export const APITokensSection: React.FC = () => {
  const { t } = useTranslation();
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
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Fallback for older browsers
      const textarea = document.createElement('textarea');
      textarea.value = text;
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand('copy');
      document.body.removeChild(textarea);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const activeTokens = tokens.filter(tok => !tok.revoked);
  const revokedTokens = tokens.filter(tok => tok.revoked);

  return (
    <div className={styles.form} style={{ marginTop: '1.5rem' }}>
      <div className={styles.formHeader}>
        <h3>{t('admin.apiTokens.heading')}</h3>
        <button className={styles.btnSmall} onClick={() => { setShowForm(f => !f); setNewToken(null); }}>
          <Plus size={14} /> {t('admin.apiTokens.addButton')}
        </button>
      </div>
      <p className={styles.sectionDesc}>
        {t('admin.apiTokens.description')}
      </p>

      {/* New token reveal banner */}
      {newToken && (
        <div className={styles.tokenRevealBox}>
          <div className={styles.flexRow} style={{ marginBottom: '0.5rem' }}>
            <AlertTriangle size={16} style={{ color: '#f59e0b' }} />
            <span style={{ fontSize: '0.85rem', fontWeight: 700, color: '#f59e0b' }}>
              {t('admin.apiTokens.copyNowWarning')}
            </span>
          </div>
          <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: '0.5rem' }}>
            {t('admin.apiTokens.tokenFor', { name: newToken.name })}
          </p>
          <div className={styles.flexRow}>
            <code className={styles.tokenCode}>
              {newToken.token}
            </code>
            <button
              className={styles.btnSmall}
              onClick={() => handleCopy(newToken.token)}
              style={{ flexShrink: 0 }}
            >
              {copied ? <><Check size={14} /> {t('admin.apiTokens.copied')}</> : <><Copy size={14} /> {t('admin.apiTokens.copy')}</>}
            </button>
          </div>
          <button onClick={() => setNewToken(null)} className={styles.dismissBtn}>
            {t('admin.apiTokens.dismiss')}
          </button>
        </div>
      )}

      {/* Create form */}
      {showForm && (
        <form onSubmit={handleCreate} style={{ marginBottom: '1rem' }}>
          <div className={styles.formGrid}>
            <div className={styles.formGroup}>
              <label className={styles.label}>{t('admin.apiTokens.tokenNameLabel')}</label>
              <input
                className={styles.input}
                value={tokenName}
                onChange={e => setTokenName(e.target.value)}
                placeholder={t('admin.apiTokens.tokenNamePlaceholder')}
                required
                autoFocus
              />
            </div>
          </div>
          <div className={styles.formActions}>
            <button type="submit" className={styles.btnPrimary} disabled={creating || !tokenName.trim()}>
              <Key size={14} /> {creating ? t('admin.apiTokens.creating') : t('admin.apiTokens.createToken')}
            </button>
            <button type="button" className={styles.btnSecondary} onClick={() => setShowForm(false)}>{t('admin.apiTokens.cancel')}</button>
          </div>
        </form>
      )}

      {/* Token list */}
      {activeTokens.length === 0 && revokedTokens.length === 0 && !showForm && (
        <p className={styles.emptyTextItalic}>{t('admin.apiTokens.noTokens')}</p>
      )}

      {activeTokens.map(tok => (
        <div key={tok.id} className={styles.listItem} style={{ marginBottom: '0.5rem' }}>
          <div className={styles.listItemContentRow}>
            <Key size={16} style={{ color: 'var(--accent-blue)', flexShrink: 0 }} />
            <div style={{ flex: 1, minWidth: 0 }}>
              <div className={styles.tokenName}>{tok.name}</div>
              <div className={styles.tokenMeta}>
                <span>{t('admin.apiTokens.created', { date: new Date(tok.created_at).toLocaleDateString() })}</span>
                {tok.last_used_at && <span>{t('admin.apiTokens.lastUsed', { date: new Date(tok.last_used_at).toLocaleDateString() })}</span>}
                {!tok.last_used_at && <span style={{ fontStyle: 'italic' }}>{t('admin.apiTokens.neverUsed')}</span>}
              </div>
            </div>
            <button className={clsx(styles.btnSmall, styles.btnDanger)} onClick={() => handleRevoke(tok.id)}>
              <Trash2 size={14} /> {t('admin.apiTokens.revoke')}
            </button>
          </div>
        </div>
      ))}

      {revokedTokens.length > 0 && (
        <>
          <div className={styles.revokedLabel}>
            {t('admin.apiTokens.revokedLabel')}
          </div>
          {revokedTokens.map(tok => (
            <div key={tok.id} className={styles.listItem} style={{ marginBottom: '0.5rem', opacity: 0.5 }}>
              <div className={styles.listItemContentRow}>
                <Key size={16} style={{ color: 'var(--text-secondary)', flexShrink: 0 }} />
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div className={styles.tokenName} style={{ textDecoration: 'line-through' }}>{tok.name}</div>
                  <div className={styles.tokenMeta}>
                    <span>{t('admin.apiTokens.created', { date: new Date(tok.created_at).toLocaleDateString() })}</span>
                    <span style={{ color: '#ef4444', fontWeight: 600 }}>{t('admin.apiTokens.revokedStatus')}</span>
                  </div>
                </div>
              </div>
            </div>
          ))}
        </>
      )}
    </div>
  );
};
