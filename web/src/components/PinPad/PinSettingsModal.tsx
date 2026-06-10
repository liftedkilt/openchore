import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, APIError } from '../../api';
import Modal from '../Modal/Modal';
import PinPad from './PinPad';
import styles from './PinSettingsModal.module.css';

interface PinSettingsModalProps {
  userId: number;
  hasPin: boolean;
  onClose: () => void;
  onChanged: (hasPin: boolean) => void;
}

type Step =
  | 'menu'          // choose Set/Change/Remove (only shown when hasPin is true)
  | 'current'       // prompt for existing PIN
  | 'new'           // prompt for new PIN
  | 'confirm'       // confirm new PIN
  | 'removeConfirm' // confirm current PIN for removal
  | 'done';

export const PinSettingsModal: React.FC<PinSettingsModalProps> = ({ userId, hasPin, onClose, onChanged }) => {
  const { t } = useTranslation();
  const [step, setStep] = useState<Step>(hasPin ? 'menu' : 'new');
  const [intent, setIntent] = useState<'set' | 'change' | 'remove'>(hasPin ? 'change' : 'set');
  const [currentPin, setCurrentPin] = useState('');
  const [firstPin, setFirstPin] = useState('');
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);
  const [successMsg, setSuccessMsg] = useState('');

  const resetError = () => setError('');

  const startChange = () => {
    setIntent('change');
    setError('');
    setStep('current');
  };

  const startRemove = () => {
    setIntent('remove');
    setError('');
    setStep('removeConfirm');
  };

  const handleCurrentPin = (pin: string) => {
    // We don't verify here; the server checks it on submit. Just store + advance.
    resetError();
    setCurrentPin(pin);
    setStep('new');
  };

  const handleNewPin = (pin: string) => {
    resetError();
    setFirstPin(pin);
    setStep('confirm');
  };

  const handleConfirmPin = async (pin: string) => {
    if (pin !== firstPin) {
      setError(t('common.pinSettings.errorPinsMismatch'));
      setFirstPin('');
      setStep('new');
      return;
    }
    setSaving(true);
    try {
      await api.users.setPin(userId, pin, currentPin || undefined);
      setSuccessMsg(intent === 'set' ? t('common.pinSettings.successPinSet') : t('common.pinSettings.successPinUpdated'));
      setStep('done');
      onChanged(true);
    } catch (e) {
      if (e instanceof APIError && e.status === 401) {
        setError(t('common.pinSettings.errorIncorrectCurrentPin'));
        setCurrentPin('');
        setFirstPin('');
        setStep('current');
      } else if (e instanceof APIError && e.status === 400) {
        setError(e.message || t('common.pinSettings.errorInvalidPin'));
        setFirstPin('');
        setStep('new');
      } else {
        setError(t('common.pinSettings.errorFailedSave'));
      }
    } finally {
      setSaving(false);
    }
  };

  const handleRemoveSubmit = async (pin: string) => {
    setSaving(true);
    try {
      await api.users.clearPin(userId, pin);
      setSuccessMsg(t('common.pinSettings.successPinRemoved'));
      setStep('done');
      onChanged(false);
    } catch (e) {
      if (e instanceof APIError && e.status === 401) {
        setError(t('common.pinSettings.errorIncorrectPin'));
      } else {
        setError(t('common.pinSettings.errorFailedRemove'));
      }
    } finally {
      setSaving(false);
    }
  };

  const title = intent === 'remove' ? t('common.pinSettings.titleRemove') : (hasPin ? t('common.pinSettings.titleChange') : t('common.pinSettings.titleSet'));

  return (
    <Modal isOpen onClose={onClose} title={title} maxWidth="400px">
      <div className={styles.body}>
        {step === 'menu' && (
          <div className={styles.menu}>
            <p className={styles.hint}>{t('common.pinSettings.menuHint')}</p>
            <button className={styles.menuBtn} onClick={startChange}>{t('common.pinSettings.btnChange')}</button>
            <button className={`${styles.menuBtn} ${styles.danger}`} onClick={startRemove}>{t('common.pinSettings.btnRemove')}</button>
          </div>
        )}

        {step === 'current' && (
          <PinPad
            prompt={t('common.pinSettings.promptEnterCurrent')}
            error={error}
            onSubmit={handleCurrentPin}
          />
        )}

        {step === 'new' && (
          <PinPad
            prompt={intent === 'set' ? t('common.pinSettings.promptChooseNew') : t('common.pinSettings.promptEnterNew')}
            error={error}
            onSubmit={handleNewPin}
          />
        )}

        {step === 'confirm' && (
          <PinPad
            prompt={t('common.pinSettings.promptConfirm')}
            error={error}
            onSubmit={handleConfirmPin}
          />
        )}

        {step === 'removeConfirm' && (
          <PinPad
            prompt={t('common.pinSettings.promptEnterCurrentToRemove')}
            error={error}
            onSubmit={handleRemoveSubmit}
          />
        )}

        {step === 'done' && (
          <div className={styles.done}>
            <p className={styles.doneText}>{successMsg}</p>
            <button className={styles.menuBtn} onClick={onClose}>{t('common.pinSettings.btnDone')}</button>
          </div>
        )}

        {saving && step !== 'done' && <p className={styles.saving}>{t('common.pinSettings.saving')}</p>}
      </div>
    </Modal>
  );
};

export default PinSettingsModal;
