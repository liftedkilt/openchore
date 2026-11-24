// Settings → Leaderboard & friend families: the weekly leaderboard switch,
// who is shared with friend families, and the links to other families'
// OpenChore (made through a coordination server; see docs/friends.md).
import React, { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { api, APIError } from '../../api';
import type { FriendLink, FriendsSettings } from '../../types';
import { FormMessage } from './FormMessage';
import ui from './ui.module.css';
import ff from './FriendFamiliesSection.module.css';

type Msg = { type: 'success' | 'error'; text: string } | null;

/** The team goal with one family: put one forward, agree to theirs, or drop it. */
function GoalRow({ link, busy, tt, onSet }: {
  link: FriendLink; busy: boolean; tt: (key: string, opts?: Record<string, unknown>) => string; onSet: (target: number) => void;
}) {
  const g = link.goal;
  const [target, setTarget] = useState(g?.target ?? 85);
  const family = link.peer_name || tt('unnamed');
  return (
    <div className={ff.goal}>
      <span className={ui.label}>{tt('goalTitle')}</span>
      <p className={ff.note}>
        {!g ? tt('goalNone')
          : g.state === 'active' ? tt('goalActive', { target: g.target, family })
          : g.state === 'ours' ? tt('goalOurs', { target: g.target, family })
          : g.state === 'differs' ? tt('goalDiffers', { target: g.target, theirs: g.their_target, family })
          : tt('goalTheirs', { target: g.target, family })}
      </p>
      <div className={ff.inline}>
        {g?.state === 'theirs' && (
          <button type="button" className={ui.btnPrimary} disabled={busy} onClick={() => onSet(g.target)}>{tt('goalAgree', { target: g.target })}</button>
        )}
        {g?.state === 'differs' && g.their_target && (
          <button type="button" className={ui.btnPrimary} disabled={busy} onClick={() => onSet(g.their_target ?? 0)}>{tt('goalAgree', { target: g.their_target })}</button>
        )}
        {g?.state !== 'active' && (
          <>
            <input className={clsx(ui.input, ff.goalInput)} type="number" min={1} max={100} value={target}
              aria-label={tt('goalInput')} onChange={e => setTarget(Number(e.target.value))} />
            <button type="button" className={ui.btn} disabled={busy || !(target >= 1 && target <= 100)} onClick={() => onSet(target)}>
              {tt(g?.state === 'ours' || g?.state === 'differs' ? 'goalChange' : 'goalPropose')}
            </button>
          </>
        )}
        {g && g.state !== 'theirs' && (
          <button type="button" className={ui.btnGhost} disabled={busy} onClick={() => onSet(0)}>{tt('goalDrop')}</button>
        )}
      </div>
    </div>
  );
}

export const FriendFamiliesSection: React.FC = () => {
  const { t, i18n } = useTranslation();
  const tt = (key: string, opts: Record<string, unknown> = {}): string => t(`admin.settingsTab.friends.${key}`, opts);
  const [cfg, setCfg] = useState<FriendsSettings | null>(null);
  const [name, setName] = useState('');
  const [hubUrl, setHubUrl] = useState('');
  const [hubKey, setHubKey] = useState('');
  const [invite, setInvite] = useState('');
  const [busy, setBusy] = useState(false);
  const [confirming, setConfirming] = useState<number | null>(null);
  const [copied, setCopied] = useState<number | null>(null);
  const [message, setMessage] = useState<Msg>(null);

  const apply = useCallback((c: FriendsSettings) => {
    setCfg(c);
    setName(c.family_name);
    setHubUrl(c.hub_url);
  }, []);
  const load = useCallback(() => { api.friends.settings().then(apply).catch(() => {}); }, [apply]);
  useEffect(load, [load]);

  // Run one action: busy state, error message, reload.
  const run = async (fn: () => Promise<unknown>, ok?: string) => {
    setBusy(true);
    setMessage(null);
    try {
      await fn();
      if (ok) setMessage({ type: 'success', text: ok });
    } catch (e) {
      setMessage({ type: 'error', text: e instanceof APIError ? e.message : tt('failed') });
    }
    setBusy(false);
    load();
  };

  if (!cfg) return null;

  const when = (iso?: string) => (iso ? new Date(iso).toLocaleString(i18n.language, { dateStyle: 'medium', timeStyle: 'short' }) : '');
  const copy = async (l: FriendLink) => {
    try {
      await navigator.clipboard.writeText(l.invite ?? '');
      setCopied(l.id);
      setTimeout(() => setCopied(null), 2500);
    } catch {
      setMessage({ type: 'error', text: tt('copyFailed') });
    }
  };
  const unlink = (l: FriendLink) => {
    if (confirming !== l.id) {
      setConfirming(l.id);
      return;
    }
    setConfirming(null);
    run(() => api.friends.unlink(l.id), tt('unlinked'));
  };
  const saveHub = () => run(
    () => api.friends.update({ hub_url: hubUrl.trim(), ...(hubKey ? { hub_key: hubKey } : {}) }).then(() => setHubKey('')),
    tt('saved'),
  );
  const nameDirty = name.trim() !== cfg.family_name;
  const canLink = cfg.family_name !== '';
  // Tick or untick one of our people for one family.
  const share = (l: FriendLink, userId: number, on: boolean) => run(() => api.friends.updateLink(l.id, {
    share_user_ids: l.outbound.filter(p => (p.id === userId ? on : p.shared)).map(p => p.id),
  }));
  // Tick or untick one of their people.
  const accept = (l: FriendLink, memberId: string, on: boolean) => run(() => api.friends.updateLink(l.id, {
    accept_member_ids: l.inbound.filter(m => (m.id === memberId ? on : m.accepted)).map(m => m.id),
  }));

  return (
    <section className={clsx(ui.card, ui.section)}>
      <h3 className={ui.sectionTitle}>{tt('title')}</h3>
      <p className={ui.sectionDesc}>{tt('description')}</p>
      <label className={ui.check}>
        <input
          type="checkbox"
          checked={cfg.leaderboard_enabled}
          disabled={busy}
          onChange={e => run(() => api.friends.update({ leaderboard_enabled: e.target.checked }))}
        />
        <span className={ui.checkLabel}>{tt('toggle')}</span>
      </label>

      <h4 className={ff.sub}>{tt('familiesTitle')}</h4>
      <p className={ui.sectionDesc}>{tt('familiesDesc')}</p>

      <div className={ff.inline}>
        <label className={ff.grow}>
          <span className={ui.label}>{tt('familyName')}</span>
          <input className={ui.input} value={name} maxLength={40} placeholder={tt('familyNamePlaceholder')} onChange={e => setName(e.target.value)} />
        </label>
        <button type="button" className={ui.btn} disabled={busy || !nameDirty} onClick={() => run(() => api.friends.update({ family_name: name.trim() }), tt('saved'))}>
          {t('common.save')}
        </button>
      </div>

      {cfg.links.length > 0 && (
        <ul className={ui.list}>
          {cfg.links.map(l => (
            <li key={l.id} className={ff.link}>
              <div className={ff.linkHead}>
                <strong>{l.peer_name || tt(l.status === 'waiting' ? 'waitingName' : 'unnamed')}</strong>
                <span className={clsx(ff.status, ff[l.status])}>{tt(`status.${l.status}`)}</span>
              </div>
              {l.status === 'waiting' && l.invite && (
                <>
                  <p className={ff.note}>{tt('inviteHelp', { when: when(l.invite_expires) })}</p>
                  <div className={ff.inline}>
                    <input className={clsx(ui.input, ff.grow, ff.code)} readOnly value={l.invite} onFocus={e => e.target.select()} aria-label={tt('inviteLabel')} />
                    <button type="button" className={ui.btn} onClick={() => copy(l)}>{copied === l.id ? tt('copied') : tt('copy')}</button>
                  </div>
                </>
              )}
              {l.status === 'active' && (
                <>
                  {l.last_sync_at && <p className={ff.note}>{tt('lastSync', { when: when(l.last_sync_at) })}</p>}
                  <p className={ff.note}>{tt('bothPick')}</p>
                  <div className={ff.sides}>
                    <fieldset className={ff.people}>
                      <legend className={ui.label}>{tt('outboundTitle', { family: l.peer_name || tt('unnamed') })}</legend>
                      {l.outbound.map(p => (
                        <label key={p.id} className={ui.check}>
                          <input type="checkbox" checked={p.shared} disabled={busy} onChange={e => share(l, p.id, e.target.checked)} />
                          <span className={ui.checkLabel}>
                            {p.name}
                            {p.shared && <span className={clsx(ff.hint, p.accepted && ff.ok)}>{tt(p.accepted ? 'theyShow' : 'theyHavent')}</span>}
                          </span>
                        </label>
                      ))}
                    </fieldset>
                    <fieldset className={ff.people}>
                      <legend className={ui.label}>{tt('inboundTitle', { family: l.peer_name || tt('unnamed') })}</legend>
                      {l.inbound.length === 0 && <p className={ff.note}>{tt('theyShareNobody')}</p>}
                      {l.inbound.map(m => (
                        <label key={m.id} className={ui.check}>
                          <input type="checkbox" checked={m.accepted} disabled={busy} onChange={e => accept(l, m.id, e.target.checked)} />
                          <span className={ui.checkLabel}>
                            {m.name}
                            {!m.accepted && <span className={ff.hint}>{tt('notShown')}</span>}
                          </span>
                        </label>
                      ))}
                    </fieldset>
                  </div>
                  <label className={ui.check}>
                    <input type="checkbox" checked={l.show_pictures} disabled={busy}
                      onChange={e => run(() => api.friends.updateLink(l.id, { show_pictures: e.target.checked }))} />
                    <span className={ui.checkLabel}>{tt('showPictures', { family: l.peer_name || tt('unnamed') })}</span>
                  </label>
                  <p className={ff.note}>{tt('showPicturesDesc')}</p>
                  <label className={ui.check}>
                    <input type="checkbox" checked={l.allow_visits} disabled={busy}
                      onChange={e => run(() => api.friends.updateLink(l.id, { allow_visits: e.target.checked }))} />
                    <span className={ui.checkLabel}>{tt('allowVisits', { family: l.peer_name || tt('unnamed') })}</span>
                  </label>
                  <p className={ff.note}>{tt('allowVisitsDesc')}</p>
                  <GoalRow link={l} busy={busy} tt={tt}
                    onSet={target => run(() => api.friends.updateLink(l.id, { goal_target: target }), tt(target ? 'goalSaved' : 'goalDropped'))} />
                  <p className={ff.note}>{tt('shareDesc')}</p>
                </>
              )}
              {l.status === 'ended' && <p className={ff.note}>{tt('endedHelp')}</p>}
              {l.last_error && <p className={clsx(ff.note, ff.error)}>{tt('syncError', { error: l.last_error })}</p>}
              <div className={ff.actions}>
                {l.status !== 'ended' && (
                  <button type="button" className={ui.btnGhost} disabled={busy} onClick={() => run(() => api.friends.sync(l.id), tt('synced'))}>
                    {tt('syncNow')}
                  </button>
                )}
                <button type="button" className={confirming === l.id ? ui.btnDanger : ui.btnGhost} disabled={busy} onClick={() => unlink(l)}>
                  {confirming === l.id ? tt('confirmUnlink') : tt(l.status === 'active' ? 'unlink' : 'remove')}
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}

      <h4 className={ff.sub}>{tt('joinTitle')}</h4>
      <p className={ff.note}>{tt('joinDesc')}</p>
      <div className={ff.inline}>
        <input className={clsx(ui.input, ff.grow, ff.code)} value={invite} placeholder="ocf1.…" aria-label={tt('joinLabel')} onChange={e => setInvite(e.target.value)} />
        <button
          type="button"
          className={ui.btnPrimary}
          disabled={busy || !canLink || !invite.trim()}
          onClick={() => run(() => api.friends.join(invite).then(() => setInvite('')), tt('joined'))}
        >
          {tt('join')}
        </button>
      </div>

      <h4 className={ff.sub}>{tt('inviteTitle')}</h4>
      <p className={ff.note}>{tt('inviteDesc')}</p>
      <div className={ff.inline}>
        <label className={ff.grow}>
          <span className={ui.label}>{tt('hubUrl')}</span>
          <input className={ui.input} value={hubUrl} disabled={cfg.hub_from_env} placeholder="https://" onChange={e => setHubUrl(e.target.value)} />
        </label>
        <label className={ff.grow}>
          <span className={ui.label}>{tt('hubKey')}</span>
          <input
            className={ui.input}
            type="password"
            autoComplete="off"
            value={hubKey}
            disabled={cfg.hub_from_env}
            placeholder={cfg.hub_key_set ? tt('hubKeySet') : ''}
            onChange={e => setHubKey(e.target.value)}
          />
        </label>
        {!cfg.hub_from_env && (
          <button type="button" className={ui.btn} disabled={busy || (hubUrl.trim() === cfg.hub_url && !hubKey)} onClick={saveHub}>
            {t('common.save')}
          </button>
        )}
      </div>
      {cfg.hub_from_env && <p className={ff.note}>{tt('hubFromEnv')}</p>}
      <div className={ff.actions}>
        <button type="button" className={ui.btnPrimary} disabled={busy || !canLink || !cfg.hub_url} onClick={() => run(() => api.friends.invite(), tt('inviteMade'))}>
          {tt('makeInvite')}
        </button>
      </div>
      {!canLink && <p className={ff.note}>{tt('needName')}</p>}
      <FormMessage msg={message} />
    </section>
  );
};
