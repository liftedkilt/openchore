import React, { useState, useEffect, useCallback } from 'react';
import { api } from '../../api';
import type { APIToken } from '../../types';
import styles from '../../pages/AdminDashboard.module.css';
import { Plus, Trash2, Check, Copy, Key, AlertTriangle } from 'lucide-react';
import clsx from 'clsx';

export const APITokensSection: React.FC = () => {
  const [tokens, setTokens] = useState<APIToken[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [tokenName, setTokenName] = useState('');
  const [creating, setCreating] = useState(false);
  const [newToken, setNewToken] = useState<{ name: string; token: string } | null>(null);
  const [copied, setCopied] = useState(false);

  const loadTokens = useCallback(async () => {
    try {
      const t = await api.tokens.list();
      setTokens(t);
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

  const activeTokens = tokens.filter(t => !t.revoked);
  const revokedTokens = tokens.filter(t => t.revoked);

  return (
    <div className={styles.form} style={{ marginTop: '1.5rem' }}>
      <div className={styles.formHeader}>
        <h3>API Tokens</h3>
        <button className={styles.btnSmall} onClick={() => { setShowForm(f => !f); setNewToken(null); }}>
          <Plus size={14} /> Add
        </button>
      </div>
      <p className={styles.sectionDesc}>
        Generate tokens for external integrations (Home Assistant, scripts, etc.) to authenticate with the API.
      </p>

      {/* New token reveal banner */}
      {newToken && (
        <div className={styles.tokenRevealBox}>
          <div className={styles.flexRow} style={{ marginBottom: '0.5rem' }}>
            <AlertTriangle size={16} style={{ color: '#f59e0b' }} />
            <span style={{ fontSize: '0.85rem', fontWeight: 700, color: '#f59e0b' }}>
              Copy this token now — it will not be shown again
            </span>
          </div>
          <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: '0.5rem' }}>
            Token for <strong>{newToken.name}</strong>:
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
              {copied ? <><Check size={14} /> Copied</> : <><Copy size={14} /> Copy</>}
            </button>
          </div>
          <button onClick={() => setNewToken(null)} className={styles.dismissBtn}>
            Dismiss
          </button>
        </div>
      )}

      {/* Create form */}
      {showForm && (
        <form onSubmit={handleCreate} style={{ marginBottom: '1rem' }}>
          <div className={styles.formGrid}>
            <div className={styles.formGroup}>
              <label className={styles.label}>Token Name</label>
              <input
                className={styles.input}
                value={tokenName}
                onChange={e => setTokenName(e.target.value)}
                placeholder="e.g. Home Assistant, CI Pipeline"
                required
                autoFocus
              />
            </div>
          </div>
          <div className={styles.formActions}>
            <button type="submit" className={styles.btnPrimary} disabled={creating || !tokenName.trim()}>
              <Key size={14} /> {creating ? 'Creating...' : 'Create Token'}
            </button>
            <button type="button" className={styles.btnSecondary} onClick={() => setShowForm(false)}>Cancel</button>
          </div>
        </form>
      )}

      {/* Token list */}
      {activeTokens.length === 0 && revokedTokens.length === 0 && !showForm && (
        <p className={styles.emptyTextItalic}>No API tokens created</p>
      )}

      {activeTokens.map(t => (
        <div key={t.id} className={styles.listItem} style={{ marginBottom: '0.5rem' }}>
          <div className={styles.listItemContentRow}>
            <Key size={16} style={{ color: 'var(--accent-blue)', flexShrink: 0 }} />
            <div style={{ flex: 1, minWidth: 0 }}>
              <div className={styles.tokenName}>{t.name}</div>
              <div className={styles.tokenMeta}>
                <span>Created {new Date(t.created_at).toLocaleDateString()}</span>
                {t.last_used_at && <span>Last used {new Date(t.last_used_at).toLocaleDateString()}</span>}
                {!t.last_used_at && <span style={{ fontStyle: 'italic' }}>Never used</span>}
              </div>
            </div>
            <button className={clsx(styles.btnSmall, styles.btnDanger)} onClick={() => handleRevoke(t.id)}>
              <Trash2 size={14} /> Revoke
            </button>
          </div>
        </div>
      ))}

      {revokedTokens.length > 0 && (
        <>
          <div className={styles.revokedLabel}>
            Revoked
          </div>
          {revokedTokens.map(t => (
            <div key={t.id} className={styles.listItem} style={{ marginBottom: '0.5rem', opacity: 0.5 }}>
              <div className={styles.listItemContentRow}>
                <Key size={16} style={{ color: 'var(--text-secondary)', flexShrink: 0 }} />
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div className={styles.tokenName} style={{ textDecoration: 'line-through' }}>{t.name}</div>
                  <div className={styles.tokenMeta}>
                    <span>Created {new Date(t.created_at).toLocaleDateString()}</span>
                    <span style={{ color: '#ef4444', fontWeight: 600 }}>Revoked</span>
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
