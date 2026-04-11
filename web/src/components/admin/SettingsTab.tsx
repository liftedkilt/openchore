import React, { useState, useEffect, useCallback } from 'react';
import { api } from '../../api';
import type { Webhook, WebhookDelivery } from '../../types';
import styles from '../../pages/AdminDashboard.module.css';
import { Plus, Trash2, Save, ChevronDown, ChevronUp } from 'lucide-react';
import clsx from 'clsx';
import { ExportConfigSection } from './ExportConfigSection';
import { APITokensSection } from './APITokensSection';

export const SettingsTab: React.FC = () => {
  const [currentPin, setCurrentPin] = useState('');
  const [newPin, setNewPin] = useState('');
  const [confirmPin, setConfirmPin] = useState('');
  const [baseUrl, setBaseUrl] = useState('');
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  // Discord state
  const [discordUrl, setDiscordUrl] = useState('');
  const [discordSaving, setDiscordSaving] = useState(false);
  const [discordMessage, setDiscordMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  // AI settings state
  const [aiEnabled, setAiEnabled] = useState(false);
  const [aiThreshold, setAiThreshold] = useState('0.85');
  const [aiTtsEnabled, setAiTtsEnabled] = useState(false);
  const [aiSaving, setAiSaving] = useState(false);
  const [aiMessage, setAiMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  // Webhooks state
  const [webhooks, setWebhooks] = useState<Webhook[]>([]);
  const [showWebhookForm, setShowWebhookForm] = useState(false);
  const [webhookUrl, setWebhookUrl] = useState('');
  const [webhookSecret, setWebhookSecret] = useState('');
  const [webhookSelectedEvents, setWebhookSelectedEvents] = useState<Set<string>>(new Set());
  const [expandedWebhook, setExpandedWebhook] = useState<number | null>(null);
  const [deliveries, setDeliveries] = useState<WebhookDelivery[]>([]);

  const WEBHOOK_EVENTS = [
    { id: 'chore.completed', label: 'Completed', icon: '✅' },
    { id: 'chore.uncompleted', label: 'Uncompleted', icon: '↩️' },
    { id: 'chore.expired', label: 'Expired', icon: '⏰' },
    { id: 'chore.missed', label: 'Missed', icon: '❌' },
    { id: 'reward.redeemed', label: 'Redeemed', icon: '🎁' },
    { id: 'daily.complete', label: 'Daily Done', icon: '🌟' },
    { id: 'streak.milestone', label: 'Streak', icon: '🔥' },
    { id: 'points.decayed', label: 'Decay', icon: '📉' },
  ];

  const allEventsSelected = webhookSelectedEvents.size === 0 || webhookSelectedEvents.size === WEBHOOK_EVENTS.length;
  const toggleEvent = (id: string) => {
    setWebhookSelectedEvents(prev => {
      const next = new Set(prev);
      if (next.has(id)) { next.delete(id); } else { next.add(id); }
      return next;
    });
  };
  const eventsToString = () => allEventsSelected ? '*' : Array.from(webhookSelectedEvents).join(',');

  const loadWebhooks = useCallback(async () => {
    try {
      const wh = await api.webhooks.list();
      setWebhooks(wh);
    } catch (e) { console.error(e); }
  }, []);

  useEffect(() => { loadWebhooks(); }, [loadWebhooks]);

  // Load initial settings
  useEffect(() => {
    // We don't have a bulk settings API, so we fetch what we need
    // For now, let's just assume we can fetch specific settings if needed
    // or add a new endpoint. Since we're here, let's add a quick fetch for base_url.
    api.admin.getSetting('base_url')
      .then(data => setBaseUrl(data.value || ''))
      .catch(() => {});
    api.admin.getSetting('discord_webhook_url')
      .then(data => setDiscordUrl(data.value || ''))
      .catch(() => {});
    api.admin.getAISettings()
      .then(settings => {
        setAiEnabled(settings.ai_enabled === 'true');
        setAiThreshold(settings.ai_auto_approve_threshold || '0.85');
        setAiTtsEnabled(settings.ai_tts_enabled === 'true');
      })
      .catch(() => {});
  }, []);

  const handleSaveBaseUrl = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      await api.admin.setSetting('base_url', baseUrl);
      setMessage({ type: 'success', text: 'Base URL updated' });
    } catch {
      setMessage({ type: 'error', text: 'Failed to update Base URL' });
    }
    setSaving(false);
  };

  const handleSaveDiscordUrl = async (e: React.FormEvent) => {
    e.preventDefault();
    setDiscordSaving(true);
    setDiscordMessage(null);
    try {
      await api.admin.setSetting('discord_webhook_url', discordUrl);
      setDiscordMessage({ type: 'success', text: discordUrl ? 'Discord webhook URL saved' : 'Discord notifications disabled' });
    } catch {
      setDiscordMessage({ type: 'error', text: 'Failed to save Discord webhook URL' });
    }
    setDiscordSaving(false);
  };

  const handleTestDiscord = async () => {
    if (!discordUrl) return;
    setDiscordSaving(true);
    setDiscordMessage(null);
    try {
      const resp = await fetch(discordUrl, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          embeds: [{
            title: 'OpenChore Test',
            description: 'Discord notifications are working!',
            color: 0x22c55e,
            timestamp: new Date().toISOString(),
          }]
        })
      });
      if (resp.ok) {
        setDiscordMessage({ type: 'success', text: 'Test message sent to Discord!' });
      } else {
        setDiscordMessage({ type: 'error', text: `Discord returned status ${resp.status}` });
      }
    } catch {
      setDiscordMessage({ type: 'error', text: 'Failed to reach Discord webhook URL' });
    }
    setDiscordSaving(false);
  };

  const handleSaveAISettings = async (e: React.FormEvent) => {
    e.preventDefault();
    setAiSaving(true);
    setAiMessage(null);
    try {
      await Promise.all([
        api.admin.setSetting('ai_enabled', aiEnabled ? 'true' : 'false'),
        api.admin.setSetting('ai_auto_approve_threshold', aiThreshold),
        api.admin.setSetting('ai_tts_enabled', aiTtsEnabled ? 'true' : 'false'),
      ]);
      if (aiTtsEnabled) {
        api.admin.triggerTTSSync().catch(() => {});
      }
      setAiMessage({ type: 'success', text: 'AI settings saved' });
    } catch {
      setAiMessage({ type: 'error', text: 'Failed to save AI settings' });
    }
    setAiSaving(false);
  };

  const handleChangePin = async (e: React.FormEvent) => {
    e.preventDefault();
    setMessage(null);

    if (newPin.length < 4) {
      setMessage({ type: 'error', text: 'New PIN must be at least 4 digits' });
      return;
    }
    if (newPin !== confirmPin) {
      setMessage({ type: 'error', text: 'New PINs do not match' });
      return;
    }

    setSaving(true);
    try {
      await api.admin.updatePasscode(currentPin, newPin);
      setMessage({ type: 'success', text: 'PIN updated successfully' });
      setCurrentPin('');
      setNewPin('');
      setConfirmPin('');
    } catch {
      setMessage({ type: 'error', text: 'Failed — check your current PIN' });
    }
    setSaving(false);
  };

  const handleCreateWebhook = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!webhookUrl) return;
    try {
      await api.webhooks.create({ url: webhookUrl, secret: webhookSecret || undefined, events: eventsToString() });
      setWebhookUrl('');
      setWebhookSecret('');
      setWebhookSelectedEvents(new Set());
      setShowWebhookForm(false);
      loadWebhooks();
    } catch (e) { console.error(e); }
  };

  const handleDeleteWebhook = async (id: number) => {
    try {
      await api.webhooks.delete(id);
      loadWebhooks();
    } catch (e) { console.error(e); }
  };

  const handleToggleWebhook = async (wh: Webhook) => {
    try {
      await api.webhooks.update(wh.id, { active: !wh.active });
      loadWebhooks();
    } catch (e) { console.error(e); }
  };

  const handleExpandWebhook = async (id: number) => {
    if (expandedWebhook === id) {
      setExpandedWebhook(null);
      return;
    }
    setExpandedWebhook(id);
    try {
      const d = await api.webhooks.listDeliveries(id);
      setDeliveries(d);
    } catch (e) { console.error(e); }
  };

  return (
    <div>
      <h2 className={styles.sectionTitle}>Settings</h2>

      <form className={styles.form} onSubmit={handleSaveBaseUrl}>
        <div className={styles.formHeader}>
          <h3>System Base URL</h3>
        </div>
        <p className={styles.sectionDesc}>
          The public URL of this server (e.g. <code>https://chores.example.com</code>). Used for QR codes and notifications.
        </p>
        <div className={styles.formGroup}>
          <input
            className={styles.input}
            value={baseUrl}
            onChange={e => setBaseUrl(e.target.value)}
            placeholder="https://your-domain.com"
          />
        </div>
        <div className={styles.formActions}>
          <button type="submit" className={styles.btnPrimary} disabled={saving}>
            <Save size={16} /> Save Base URL
          </button>
        </div>
      </form>

      <form className={styles.form} onSubmit={handleSaveDiscordUrl}>
        <div className={styles.formHeader}>
          <h3>Discord Notifications</h3>
        </div>
        <p className={styles.sectionDesc}>
          Get notified in Discord when chores are completed, approved, or rejected. Paste a Discord webhook URL below.
        </p>
        <div className={styles.formGroup}>
          <input
            className={styles.input}
            value={discordUrl}
            onChange={e => setDiscordUrl(e.target.value)}
            placeholder="https://discord.com/api/webhooks/..."
          />
        </div>
        {discordMessage && (
          <p className={clsx(styles.feedbackMsg, discordMessage.type === 'success' ? styles.feedbackMsgSuccess : styles.feedbackMsgError)} style={{ marginTop: '0.25rem', marginBottom: '0.25rem' }}>
            {discordMessage.text}
          </p>
        )}
        <div className={styles.formActions}>
          <button type="submit" className={styles.btnPrimary} disabled={discordSaving}>
            <Save size={16} /> Save
          </button>
          {discordUrl && (
            <button type="button" className={styles.btnSecondary} disabled={discordSaving} onClick={handleTestDiscord}>
              Send Test
            </button>
          )}
        </div>
      </form>

      <form className={styles.form} onSubmit={handleSaveAISettings}>
        <div className={styles.formHeader}>
          <h3>AI Photo Review</h3>
        </div>
        <p className={styles.sectionDesc}>
          Use AI to automatically verify chore completion photos. When enabled, uploaded photos are analyzed before marking a chore complete.
        </p>

        <div className={styles.formGrid}>
          <label className={styles.checkboxLabel}>
            <input type="checkbox" checked={aiEnabled} onChange={e => setAiEnabled(e.target.checked)} />
            Enable AI photo review
          </label>

          <div className={styles.formGroup}>
            <label className={styles.label}>Auto-Approve Threshold (0 &ndash; 1)</label>
            <div className={styles.flexRow} style={{ gap: '0.75rem' }}>
              <input
                type="range"
                min="0"
                max="1"
                step="0.05"
                value={aiThreshold}
                onChange={e => setAiThreshold(e.target.value)}
                disabled={!aiEnabled}
                style={{ flex: 1, accentColor: 'var(--accent-blue)' }}
              />
              <span style={{ fontSize: '0.9rem', fontWeight: 700, minWidth: '3ch', textAlign: 'right' }}>{aiThreshold}</span>
            </div>
            <span className={styles.helpText}>
              Photos with confidence above this threshold are auto-approved. Lower values are more lenient.
            </span>
          </div>

          <label className={styles.checkboxLabel}>
            <input type="checkbox" checked={aiTtsEnabled} onChange={e => setAiTtsEnabled(e.target.checked)} />
            Generate TTS descriptions for chores
          </label>
        </div>

        {aiMessage && (
          <p className={clsx(styles.feedbackMsg, aiMessage.type === 'success' ? styles.feedbackMsgSuccess : styles.feedbackMsgError)}>
            {aiMessage.text}
          </p>
        )}

        <div className={styles.formActions}>
          <button type="submit" className={styles.btnPrimary} disabled={aiSaving}>
            <Save size={16} /> {aiSaving ? 'Saving...' : 'Save AI Settings'}
          </button>
        </div>
      </form>

      <form className={styles.form} onSubmit={handleChangePin}>
        <div className={styles.formHeader}>
          <h3>Change Admin PIN</h3>
        </div>

        <div className={styles.formGrid}>
          <div className={styles.formGroup}>
            <label className={styles.label}>Current PIN</label>
            <input
              className={styles.input}
              type="password"
              inputMode="numeric"
              pattern="[0-9]*"
              value={currentPin}
              onChange={e => setCurrentPin(e.target.value.replace(/\D/g, ''))}
              placeholder="Enter current PIN"
              required
            />
          </div>
          <div className={styles.formRow}>
            <div className={styles.formGroup}>
              <label className={styles.label}>New PIN</label>
              <input
                className={styles.input}
                type="password"
                inputMode="numeric"
                pattern="[0-9]*"
                value={newPin}
                onChange={e => setNewPin(e.target.value.replace(/\D/g, ''))}
                placeholder="4+ digits"
                required
              />
            </div>
            <div className={styles.formGroup}>
              <label className={styles.label}>Confirm New PIN</label>
              <input
                className={styles.input}
                type="password"
                inputMode="numeric"
                pattern="[0-9]*"
                value={confirmPin}
                onChange={e => setConfirmPin(e.target.value.replace(/\D/g, ''))}
                placeholder="Re-enter"
                required
              />
            </div>
          </div>
        </div>

        {message && (
          <p className={clsx(styles.feedbackMsg, message.type === 'success' ? styles.feedbackMsgSuccess : styles.feedbackMsgError)}>
            {message.text}
          </p>
        )}

        <div className={styles.formActions}>
          <button type="submit" className={styles.btnPrimary} disabled={saving || !currentPin || !newPin || !confirmPin}>
            <Save size={16} /> Update PIN
          </button>
        </div>
      </form>

      {/* Export Config Section */}
      <ExportConfigSection />

      {/* Webhooks Section */}
      <div className={styles.form} style={{ marginTop: '1.5rem' }}>
        <div className={styles.formHeader}>
          <h3>Webhooks</h3>
          <button className={styles.btnSmall} onClick={() => setShowWebhookForm(f => !f)}>
            <Plus size={14} /> Add
          </button>
        </div>
        <p className={styles.sectionDesc}>
          Send events to external services (Home Assistant, Discord, etc.)
        </p>

        {showWebhookForm && (
          <form onSubmit={handleCreateWebhook} style={{ marginBottom: '1rem' }}>
            <div className={styles.formGrid}>
              <div className={styles.formGroup}>
                <label className={styles.label}>URL</label>
                <input className={styles.input} value={webhookUrl} onChange={e => setWebhookUrl(e.target.value)} placeholder="https://..." required />
              </div>
              <div className={styles.formGroup}>
                <label className={styles.label}>Secret (optional HMAC signing key)</label>
                <input className={styles.input} value={webhookSecret} onChange={e => setWebhookSecret(e.target.value)} placeholder="Leave blank for unsigned" />
              </div>
              <div className={styles.formGroup}>
                <label className={styles.label}>Events {allEventsSelected && <span style={{ fontWeight: 400, color: 'var(--text-secondary)' }}>(all)</span>}</label>
                <div className={styles.chipRow} style={{ gap: '0.4rem', marginTop: '0.3rem' }}>
                  {WEBHOOK_EVENTS.map(ev => {
                    const selected = webhookSelectedEvents.has(ev.id) || allEventsSelected;
                    return (
                      <button
                        key={ev.id}
                        type="button"
                        onClick={() => toggleEvent(ev.id)}
                        className={clsx(styles.webhookEventChip, selected && styles.webhookEventChipActive)}
                      >
                        <span>{ev.icon}</span> {ev.label}
                      </button>
                    );
                  })}
                </div>
              </div>
            </div>
            <div className={styles.formActions}>
              <button type="submit" className={styles.btnPrimary}><Save size={14} /> Create</button>
              <button type="button" className={styles.btnSecondary} onClick={() => setShowWebhookForm(false)}>Cancel</button>
            </div>
          </form>
        )}

        {webhooks.length === 0 && !showWebhookForm && (
          <p className={styles.emptyTextItalic}>No webhooks configured</p>
        )}

        {webhooks.map(wh => (
          <div key={wh.id} className={styles.listItem} style={{ marginBottom: '0.5rem' }}>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div className={styles.flexRow}>
                <span className={clsx(styles.statusDot, wh.active ? styles.statusDotActive : styles.statusDotInactive)} />
                <span className={styles.webhookUrlText}>
                  {wh.url}
                </span>
              </div>
              <div className={styles.webhookMeta}>
                {wh.events === '*' ? (
                  <span>All events</span>
                ) : (
                  wh.events.split(',').map(e => {
                    const ev = WEBHOOK_EVENTS.find(we => we.id === e.trim());
                    return <span key={e} className={styles.webhookEventTag}>{ev ? `${ev.icon} ${ev.label}` : e.trim()}</span>;
                  })
                )}
                {wh.secret && <span>• Signed</span>}
              </div>
            </div>
            <div className={styles.btnGroup}>
              <button className={styles.btnSmall} onClick={() => handleExpandWebhook(wh.id)}>
                {expandedWebhook === wh.id ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
              </button>
              <button className={styles.btnSmall} onClick={() => handleToggleWebhook(wh)}>
                {wh.active ? 'Disable' : 'Enable'}
              </button>
              <button className={clsx(styles.btnSmall, styles.btnDanger)} aria-label="Delete webhook" onClick={() => handleDeleteWebhook(wh.id)}>
                <Trash2 size={14} />
              </button>
            </div>
            {expandedWebhook === wh.id && (
              <div style={{ width: '100%', marginTop: '0.5rem' }}>
                <h4 className={styles.deliveryHeader}>Recent Deliveries</h4>
                {deliveries.length === 0 ? (
                  <p className={styles.emptyTextItalic} style={{ fontSize: '0.8rem' }}>No deliveries yet</p>
                ) : (
                  <div className={styles.deliveryList}>
                    {deliveries.map(d => (
                      <div key={d.id} className={styles.deliveryItem}>
                        <span className={clsx(styles.statusDot, d.status_code && d.status_code >= 200 && d.status_code < 300 ? styles.statusDotActive : styles.statusDotError)} />
                        <span style={{ fontWeight: 600 }}>{d.event}</span>
                        <span style={{ color: 'var(--text-secondary)' }}>{d.status_code || 'err'}</span>
                        <span style={{ color: 'var(--text-secondary)', marginLeft: 'auto' }}>
                          {new Date(d.created_at).toLocaleString()}
                        </span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}
          </div>
        ))}

      </div>

      {/* API Tokens Section */}
      <APITokensSection />
    </div>
  );
};
