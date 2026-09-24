import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, APIError } from '../../api';
import type { AuthProvider, LinkedIdentity, User } from '../../types';
import { Icon } from '../../design';
import Modal from '../Modal/Modal';
import styles from './LinkedAccountsModal.module.css';

interface LinkedAccountsModalProps {
  user: User;
  // Whether the viewer is managing their own profile. Linking always happens
  // from the profile's own session (the provider sign-in proves it); parents
  // can only review and unlink on someone else's behalf.
  self: boolean;
  onClose: () => void;
  onChanged: (providers: string[]) => void;
  onSignedOutEverywhere?: () => void;
}

export const LinkedAccountsModal: React.FC<LinkedAccountsModalProps> = ({ user, self, onClose, onChanged, onSignedOutEverywhere }) => {
  const { t } = useTranslation();
  const [providers, setProviders] = useState<AuthProvider[]>([]);
  const [identities, setIdentities] = useState<LinkedIdentity[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    try {
      const [ps, ids] = await Promise.all([api.auth.providers(), api.auth.identities(user.id)]);
      setProviders(ps);
      setIdentities(ids);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, [user.id]);

  useEffect(() => { load(); }, [load]);

  const providerName = useMemo(() => {
    const byId = new Map(providers.map(p => [p.id, p.name]));
    return (id: string) => byId.get(id) ?? id;
  }, [providers]);

  const linkedIds = new Set(identities.map(i => i.provider));
  const unlinkedProviders = providers.filter(p => !linkedIds.has(p.id));

  const unlink = async (identity: LinkedIdentity) => {
    if (!window.confirm(t('linkedAccounts.confirmUnlink', { provider: providerName(identity.provider) }))) return;
    setError('');
    try {
      await api.auth.unlink(user.id, identity.id);
      const next = identities.filter(i => i.id !== identity.id);
      setIdentities(next);
      onChanged(next.map(i => i.provider));
    } catch (e) {
      setError(e instanceof APIError ? e.message : String(e));
    }
  };

  const signOutEverywhere = async () => {
    if (!window.confirm(t('linkedAccounts.confirmSignOutEverywhere'))) return;
    try {
      await api.auth.logoutEverywhere();
      onSignedOutEverywhere?.();
    } catch (e) {
      setError(e instanceof APIError ? e.message : String(e));
    }
  };

  // Come back to the dashboard with this modal's context after linking.
  const returnPath = `${window.location.pathname}${window.location.search}`;

  return (
    <Modal isOpen onClose={onClose} title={t('linkedAccounts.title')} maxWidth="440px">
      <div className={styles.body}>
        <p className={styles.hint}>
          {self ? t('linkedAccounts.introSelf') : t('linkedAccounts.introOther', { name: user.name })}
        </p>

        {loading && <p className={styles.hint}>{t('common.loading')}</p>}

        {!loading && providers.length === 0 && identities.length === 0 && (
          <p className={styles.empty}>{t('linkedAccounts.noProviders')}</p>
        )}

        {identities.length > 0 && (
          <ul className={styles.list}>
            {identities.map(i => (
              <li key={i.id} className={styles.row}>
                <div>
                  <div className={styles.provider}>{providerName(i.provider)}</div>
                  {(i.email || i.display_name) && (
                    <div className={styles.detail}>{i.email || i.display_name}</div>
                  )}
                </div>
                <button type="button" className={styles.unlinkBtn} onClick={() => unlink(i)}>
                  {t('linkedAccounts.unlink')}
                </button>
              </li>
            ))}
          </ul>
        )}

        {self && unlinkedProviders.length > 0 && (
          <div className={styles.linkButtons}>
            {unlinkedProviders.map(p => (
              <a key={p.id} className={styles.linkBtn} href={api.auth.oidcLinkURL(p.id, returnPath)}>
                {t('linkedAccounts.link', { provider: p.name })}
                <Icon name="chev" />
              </a>
            ))}
          </div>
        )}

        {error && <p className={styles.error} role="alert">{error}</p>}

        {self && (
          <button type="button" className={styles.signOutAll} onClick={signOutEverywhere}>
            {t('linkedAccounts.signOutEverywhere')}
          </button>
        )}
      </div>
    </Modal>
  );
};

export default LinkedAccountsModal;
