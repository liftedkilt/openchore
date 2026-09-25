import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { Trash2, X } from 'lucide-react';
import { api } from '../../api';
import type { Webhook, WebhookDelivery } from '../../types';
import { Icon } from '../../design';
import { ExportConfigSection } from './ExportConfigSection';
import { APITokensSection } from './APITokensSection';
import { PhotoReviewTester } from './PhotoReviewTester';
import { AIConnectionSection } from './AIConnectionSection';
import { SignInSection } from './SignInSection';
import { FormMessage, type Msg } from './FormMessage';
import { useAIStatus } from '../../hooks/useAIStatus';
import ui from './ui.module.css';
import styles from './SettingsTab.module.css';


const WEBHOOK_EVENT_IDS: { id: string; key: string }[] = [
  { id: 'chore.completed', key: 'completed' },
  { id: 'chore.uncompleted', key: 'uncompleted' },
  { id: 'chore.expired', key: 'expired' },
  { id: 'chore.missed', key: 'missed' },
  { id: 'reward.redeemed', key: 'redeemed' },
  { id: 'daily.complete', key: 'dailyDone' },
  { id: 'streak.milestone', key: 'streak' },
  { id: 'points.decayed', key: 'decay' },
  { id: 'report.weekly_summary', key: 'weeklySummary' },
  { id: 'auth.admin_passcode.verified', key: 'adminPasscodeVerified' },
  { id: 'auth.admin_passcode.failed', key: 'adminPasscodeFailed' },
  { id: 'auth.profile_pin.verified', key: 'profilePinVerified' },
  { id: 'auth.profile_pin.failed', key: 'profilePinFailed' },
  { id: 'auth.profile_pin.changed', key: 'profilePinChanged' },
  { id: 'auth.profile_pin.cleared', key: 'profilePinCleared' },
  { id: 'auth.oidc.login', key: 'oidcLogin' },
  { id: 'auth.identity.linked', key: 'identityLinked' },
  { id: 'auth.identity.unlinked', key: 'identityUnlinked' },
];

export const SettingsTab: React.FC = () => {
  const { t, i18n } = useTranslation();

  const [baseUrl, setBaseUrl] = useState('');
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<Msg>(null);

  // Discord state
  const [discordUrl, setDiscordUrl] = useState('');
  const [discordSaving, setDiscordSaving] = useState(false);
  const [discordMessage, setDiscordMessage] = useState<Msg>(null);

  // AI settings state. What can be set depends on which services are
  // connected (in AIConnectionSection or AI_BASE_URL / TTS_BASE_URL).
  const aiStatus = useAIStatus();
  const [aiPhotoReview, setAiPhotoReview] = useState(false);
  const [aiAutoApprove, setAiAutoApprove] = useState(false);
  const [aiThreshold, setAiThreshold] = useState('0.85');
  const [aiWeeklySummary, setAiWeeklySummary] = useState(false);
  const [ttsVoice, setTtsVoice] = useState('');
  const [savedTtsVoice, setSavedTtsVoice] = useState('');
  const [aiSaving, setAiSaving] = useState(false);
  const [aiMessage, setAiMessage] = useState<Msg>(null);

  // Webhooks state
  const [webhooks, setWebhooks] = useState<Webhook[]>([]);
  const [showWebhookForm, setShowWebhookForm] = useState(false);
  const [webhookUrl, setWebhookUrl] = useState('');
  const [webhookSecret, setWebhookSecret] = useState('');
  const [webhookSelectedEvents, setWebhookSelectedEvents] = useState<Set<string>>(new Set());
  const [expandedWebhook, setExpandedWebhook] = useState<number | null>(null);
  const [deliveries, setDeliveries] = useState<WebhookDelivery[]>([]);

  const eventLabel = (id: string) => {
    const ev = WEBHOOK_EVENT_IDS.find(e => e.id === id.trim());
    return ev ? t(`admin.settingsTab.webhooks.events.${ev.key}`) : id.trim();
  };

  const allEventsSelected = webhookSelectedEvents.size === 0 || webhookSelectedEvents.size === WEBHOOK_EVENT_IDS.length;
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

  // There's no bulk settings API, so fetch each setting we show.
  useEffect(() => {
    api.admin.getSetting('base_url')
      .then(data => setBaseUrl(data.value || ''))
      .catch(() => {});
    api.admin.getSetting('discord_webhook_url')
      .then(data => setDiscordUrl(data.value || ''))
      .catch(() => {});
    const setting = (key: string) => api.admin.getSetting(key).then(d => d.value || '').catch(() => '');
    Promise.all(['ai_photo_review', 'ai_auto_approve', 'ai_auto_approve_threshold', 'ai_weekly_summary', 'tts_voice'].map(setting))
      .then(([photoReview, autoApprove, threshold, weeklySummary, voice]) => {
        setAiPhotoReview(photoReview === 'true');
        setAiAutoApprove(autoApprove === 'true');
        setAiThreshold(threshold || '0.85');
        setAiWeeklySummary(weeklySummary === 'true');
        setTtsVoice(voice);
        setSavedTtsVoice(voice);
      });
  }, []);

  const handleSaveBaseUrl = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      await api.admin.setSetting('base_url', baseUrl);
      setMessage({ type: 'success', text: t('admin.settingsTab.baseUrl.saveSuccess') });
    } catch {
      setMessage({ type: 'error', text: t('admin.settingsTab.baseUrl.saveError') });
    }
    setSaving(false);
  };

  const handleSaveDiscordUrl = async (e: React.FormEvent) => {
    e.preventDefault();
    setDiscordSaving(true);
    setDiscordMessage(null);
    try {
      await api.admin.setSetting('discord_webhook_url', discordUrl);
      setDiscordMessage({ type: 'success', text: discordUrl ? t('admin.settingsTab.discord.saveSuccess') : t('admin.settingsTab.discord.disabledSuccess') });
    } catch {
      setDiscordMessage({ type: 'error', text: t('admin.settingsTab.discord.saveError') });
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
        setDiscordMessage({ type: 'success', text: t('admin.settingsTab.discord.testSuccess') });
      } else {
        setDiscordMessage({ type: 'error', text: t('admin.settingsTab.discord.testStatusError', { status: resp.status }) });
      }
    } catch {
      setDiscordMessage({ type: 'error', text: t('admin.settingsTab.discord.testNetworkError') });
    }
    setDiscordSaving(false);
  };

  const handleSaveAISettings = async (e: React.FormEvent) => {
    e.preventDefault();
    setAiSaving(true);
    setAiMessage(null);
    try {
      await Promise.all([
        api.admin.setSetting('ai_photo_review', aiPhotoReview ? 'true' : 'false'),
        api.admin.setSetting('ai_auto_approve', aiAutoApprove ? 'true' : 'false'),
        api.admin.setSetting('ai_auto_approve_threshold', aiThreshold),
        api.admin.setSetting('ai_weekly_summary', aiWeeklySummary ? 'true' : 'false'),
        api.admin.setSetting('tts_voice', ttsVoice.trim()),
      ]);
      // A new voice means every chore's audio needs re-recording.
      if (aiStatus.tts.configured && ttsVoice.trim() !== savedTtsVoice) {
        api.admin.regenerateAllTTS().catch(() => {});
        setSavedTtsVoice(ttsVoice.trim());
      }
      setAiMessage({ type: 'success', text: t('admin.settingsTab.ai.saveSuccess') });
    } catch {
      setAiMessage({ type: 'error', text: t('admin.settingsTab.ai.saveError') });
    }
    setAiSaving(false);
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
    <div className={ui.page}>
      <div className={ui.pageHead}>
        <div>
          <h2 className={ui.pageTitle}>{t('admin.settingsTab.pageTitle')}</h2>
          <p className={ui.pageSub}>{t('admin.settingsTab.subtitle')}</p>
        </div>
      </div>

      <div className={styles.columns}>
        <div className={ui.cardStack}>
          <form className={clsx(ui.card, ui.section)} onSubmit={handleSaveBaseUrl}>
            <h3 className={ui.sectionTitle}>{t('admin.settingsTab.baseUrl.title')}</h3>
            <p className={ui.sectionDesc}>
              {t('admin.settingsTab.baseUrl.description')}<code>https://chores.example.com</code>{t('admin.settingsTab.baseUrl.descriptionSuffix')}
            </p>
            <input
              className={ui.input}
              value={baseUrl}
              onChange={e => setBaseUrl(e.target.value)}
              placeholder={t('admin.settingsTab.baseUrl.placeholder')}
              aria-label={t('admin.settingsTab.baseUrl.title')}
            />
            <FormMessage msg={message} />
            <div className={ui.actionsEnd}>
              <button type="submit" className={ui.btnPrimary} disabled={saving}>
                <Icon name="check" /> {t('admin.settingsTab.baseUrl.saveButton')}
              </button>
            </div>
          </form>

          <form className={clsx(ui.card, ui.section)} onSubmit={handleSaveDiscordUrl}>
            <h3 className={ui.sectionTitle}>{t('admin.settingsTab.discord.title')}</h3>
            <p className={ui.sectionDesc}>{t('admin.settingsTab.discord.description')}</p>
            <input
              className={ui.input}
              value={discordUrl}
              onChange={e => setDiscordUrl(e.target.value)}
              placeholder={t('admin.settingsTab.discord.placeholder')}
              aria-label={t('admin.settingsTab.discord.title')}
            />
            <FormMessage msg={discordMessage} />
            <div className={ui.actionsEnd}>
              {discordUrl && (
                <button type="button" className={ui.btnGhost} disabled={discordSaving} onClick={handleTestDiscord}>
                  {t('admin.settingsTab.discord.testButton')}
                </button>
              )}
              <button type="submit" className={ui.btnPrimary} disabled={discordSaving}>
                <Icon name="check" /> {t('admin.settingsTab.discord.saveButton')}
              </button>
            </div>
          </form>

          <AIConnectionSection />

          {(aiStatus.ai.configured || aiStatus.tts.configured) && (
            <form className={clsx(ui.card, ui.section)} onSubmit={handleSaveAISettings} aria-labelledby="settings-ai">
              <h3 className={ui.sectionTitle} id="settings-ai"><Icon name="spark" /> {t('admin.settingsTab.ai.title')}</h3>
              <p className={ui.sectionDesc}>
                {aiStatus.ai.configured
                  ? t('admin.settingsTab.ai.description', { model: aiStatus.ai.model })
                  : t('admin.settingsTab.ai.aiNotConfigured')}
              </p>

              {aiStatus.ai.configured && (
                <>
                  <div className={ui.field}>
                    <label className={ui.check}>
                      <input type="checkbox" checked={aiPhotoReview} onChange={e => setAiPhotoReview(e.target.checked)} />
                      <span className={ui.checkLabel}>{t('admin.settingsTab.ai.photoReviewLabel')}</span>
                    </label>
                    <span className={ui.help}>{t('admin.settingsTab.ai.photoReviewHelp')}</span>
                  </div>

                  <label className={ui.check}>
                    <input type="checkbox" checked={aiAutoApprove} disabled={!aiPhotoReview} onChange={e => setAiAutoApprove(e.target.checked)} />
                    <span className={ui.checkLabel}>{t('admin.settingsTab.ai.autoApproveLabel')}</span>
                  </label>

                  <label className={ui.field}>
                    <span className={ui.label}>{t('admin.settingsTab.ai.thresholdLabel')}</span>
                    <span className={styles.rangeRow}>
                      <input
                        type="range"
                        className={ui.range}
                        min="0.5"
                        max="1"
                        step="0.05"
                        value={aiThreshold}
                        onChange={e => setAiThreshold(e.target.value)}
                        disabled={!aiPhotoReview || !aiAutoApprove}
                      />
                      <output className={styles.rangeValue}>{Math.round(Number(aiThreshold) * 100)}%</output>
                    </span>
                    <span className={ui.help}>{t('admin.settingsTab.ai.thresholdHelp')}</span>
                  </label>

                  <div className={ui.field}>
                    <label className={ui.check}>
                      <input type="checkbox" checked={aiWeeklySummary} onChange={e => setAiWeeklySummary(e.target.checked)} />
                      <span className={ui.checkLabel}>{t('admin.settingsTab.ai.weeklySummaryLabel')}</span>
                    </label>
                    <span className={ui.help}>{t('admin.settingsTab.ai.weeklySummaryHelp')}</span>
                  </div>
                </>
              )}

              {aiStatus.tts.configured && (
                <label className={ui.field}>
                  <span className={ui.label}>{t('admin.settingsTab.ai.voiceLabel')}</span>
                  <input
                    className={ui.input}
                    value={ttsVoice}
                    onChange={e => setTtsVoice(e.target.value)}
                    placeholder="af_heart"
                  />
                  <span className={ui.help}>{t('admin.settingsTab.ai.voiceHelp')}</span>
                </label>
              )}

              <FormMessage msg={aiMessage} />

              <div className={ui.actionsEnd}>
                <button type="submit" className={ui.btnPrimary} disabled={aiSaving}>
                  <Icon name="check" /> {aiSaving ? t('admin.settingsTab.ai.savingButton') : t('admin.settingsTab.ai.saveButton')}
                </button>
              </div>
            </form>
          )}

          {aiStatus.ai.configured && <PhotoReviewTester />}
        </div>

        <div className={ui.cardStack}>
          <SignInSection />

          <section className={clsx(ui.card, ui.section)}>
            <div className={ui.sectionHead}>
              <h3 className={ui.sectionTitle}>{t('admin.settingsTab.webhooks.title')}</h3>
              <button type="button" className={ui.btnGhost} aria-expanded={showWebhookForm} onClick={() => setShowWebhookForm(f => !f)}>
                {showWebhookForm ? <X aria-hidden /> : <Icon name="plus" />}
                {showWebhookForm ? t('admin.settingsTab.webhooks.form.cancelButton') : t('admin.settingsTab.webhooks.addButton')}
              </button>
            </div>
            <p className={ui.sectionDesc}>{t('admin.settingsTab.webhooks.description')}</p>

            {showWebhookForm && (
              <form onSubmit={handleCreateWebhook} className={ui.inset}>
                <label className={ui.field}>
                  <span className={ui.label}>{t('admin.settingsTab.webhooks.form.urlLabel')}</span>
                  <input className={ui.input} value={webhookUrl} onChange={e => setWebhookUrl(e.target.value)} placeholder={t('admin.settingsTab.webhooks.form.urlPlaceholder')} required />
                </label>
                <label className={ui.field}>
                  <span className={ui.label}>{t('admin.settingsTab.webhooks.form.secretLabel')}</span>
                  <input className={ui.input} value={webhookSecret} onChange={e => setWebhookSecret(e.target.value)} placeholder={t('admin.settingsTab.webhooks.form.secretPlaceholder')} />
                </label>
                <div className={ui.field}>
                  <span className={ui.label}>
                    {t('admin.settingsTab.webhooks.form.eventsLabel')}
                    {allEventsSelected && <span className={styles.labelNote}> ({t('admin.settingsTab.webhooks.form.eventsAll')})</span>}
                  </span>
                  <div className={ui.chips}>
                    {WEBHOOK_EVENT_IDS.map(ev => (
                      <button
                        key={ev.id}
                        type="button"
                        onClick={() => toggleEvent(ev.id)}
                        className={styles.eventChip}
                        aria-pressed={webhookSelectedEvents.has(ev.id) || allEventsSelected}
                      >
                        {t(`admin.settingsTab.webhooks.events.${ev.key}`)}
                      </button>
                    ))}
                  </div>
                </div>
                <div className={ui.actionsEnd}>
                  <button type="button" className={ui.btnGhost} onClick={() => setShowWebhookForm(false)}>{t('admin.settingsTab.webhooks.form.cancelButton')}</button>
                  <button type="submit" className={ui.btnPrimary}><Icon name="check" /> {t('admin.settingsTab.webhooks.form.createButton')}</button>
                </div>
              </form>
            )}

            {webhooks.length === 0 && !showWebhookForm && (
              <p className={ui.emptyInline}>{t('admin.settingsTab.webhooks.empty')}</p>
            )}

            {webhooks.length > 0 && (
              <div className={styles.subList}>
                {webhooks.map(wh => (
                  <div key={wh.id} className={styles.webhook}>
                    <div className={styles.webhookRow}>
                      <span className={wh.active ? ui.dot : ui.dotOff} aria-hidden />
                      <div className={ui.rowMain}>
                        <span className={styles.url}>{wh.url}</span>
                        <span className={ui.rowMeta}>
                          <span>{wh.active ? t('admin.settingsTab.webhooks.active') : t('admin.settingsTab.webhooks.inactive')}</span>
                          {wh.events === '*' ? (
                            <span>{t('admin.settingsTab.webhooks.allEvents')}</span>
                          ) : (
                            <span>{wh.events.split(',').map(eventLabel).join(', ')}</span>
                          )}
                          {wh.secret && <span><Icon name="lock" /> {t('admin.settingsTab.webhooks.signed')}</span>}
                        </span>
                      </div>
                    </div>
                    <div className={styles.webhookActions}>
                      <button
                        type="button"
                        className={ui.btnGhost}
                        aria-expanded={expandedWebhook === wh.id}
                        onClick={() => handleExpandWebhook(wh.id)}
                      >
                        <Icon name="chev" className={clsx(styles.caret, expandedWebhook === wh.id && styles.caretOpen)} />
                        {t('admin.settingsTab.webhooks.deliveries.title')}
                      </button>
                      <button type="button" className={ui.btnGhost} onClick={() => handleToggleWebhook(wh)}>
                        {wh.active ? t('admin.settingsTab.webhooks.disableButton') : t('admin.settingsTab.webhooks.enableButton')}
                      </button>
                      <button type="button" className={clsx(ui.iconBtn, ui.iconBtnDanger)} aria-label={t('admin.settingsTab.webhooks.deleteAriaLabel')} title={t('admin.settingsTab.webhooks.deleteAriaLabel')} onClick={() => handleDeleteWebhook(wh.id)}>
                        <Trash2 aria-hidden />
                      </button>
                    </div>
                    {expandedWebhook === wh.id && (
                      <div className={styles.deliveries}>
                        {deliveries.length === 0 ? (
                          <p className={ui.emptyInline}>{t('admin.settingsTab.webhooks.deliveries.empty')}</p>
                        ) : (
                          <ul className={styles.deliveryList}>
                            {deliveries.map(d => {
                              const ok = !!d.status_code && d.status_code >= 200 && d.status_code < 300;
                              return (
                                <li key={d.id} className={styles.delivery}>
                                  <span className={ok ? ui.dot : ui.dotError} aria-hidden />
                                  <span className={styles.deliveryEvent}>{d.event}</span>
                                  <span className={ok ? styles.deliveryStatus : styles.deliveryStatusError}>{d.status_code || 'err'}</span>
                                  <span className={styles.deliveryTime}>{new Date(d.created_at).toLocaleString(i18n.language)}</span>
                                </li>
                              );
                            })}
                          </ul>
                        )}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </section>

          <APITokensSection />

          <ExportConfigSection />
        </div>
      </div>
    </div>
  );
};
