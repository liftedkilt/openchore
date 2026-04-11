import React, { useState } from 'react';
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
      setError("PINs don't match");
      setFirstPin('');
      setStep('new');
      return;
    }
    setSaving(true);
    try {
      await api.users.setPin(userId, pin, currentPin || undefined);
      setSuccessMsg(intent === 'set' ? 'PIN set' : 'PIN updated');
      setStep('done');
      onChanged(true);
    } catch (e) {
      if (e instanceof APIError && e.status === 401) {
        setError('Incorrect current PIN');
        setCurrentPin('');
        setFirstPin('');
        setStep('current');
      } else if (e instanceof APIError && e.status === 400) {
        setError(e.message || 'Invalid PIN');
        setFirstPin('');
        setStep('new');
      } else {
        setError('Failed to save PIN');
      }
    } finally {
      setSaving(false);
    }
  };

  const handleRemoveSubmit = async (pin: string) => {
    setSaving(true);
    try {
      await api.users.clearPin(userId, pin);
      setSuccessMsg('PIN removed');
      setStep('done');
      onChanged(false);
    } catch (e) {
      if (e instanceof APIError && e.status === 401) {
        setError('Incorrect PIN');
      } else {
        setError('Failed to remove PIN');
      }
    } finally {
      setSaving(false);
    }
  };

  const title = intent === 'remove' ? 'Remove PIN' : (hasPin ? 'Change PIN' : 'Set PIN');

  return (
    <Modal isOpen onClose={onClose} title={title} maxWidth="400px">
      <div className={styles.body}>
        {step === 'menu' && (
          <div className={styles.menu}>
            <p className={styles.hint}>Your profile is protected by a PIN.</p>
            <button className={styles.menuBtn} onClick={startChange}>Change PIN</button>
            <button className={`${styles.menuBtn} ${styles.danger}`} onClick={startRemove}>Remove PIN</button>
          </div>
        )}

        {step === 'current' && (
          <PinPad
            prompt="Enter your current PIN"
            error={error}
            onSubmit={handleCurrentPin}
          />
        )}

        {step === 'new' && (
          <PinPad
            prompt={intent === 'set' ? 'Choose a 4-digit PIN' : 'Enter a new 4-digit PIN'}
            error={error}
            onSubmit={handleNewPin}
          />
        )}

        {step === 'confirm' && (
          <PinPad
            prompt="Enter the same PIN again to confirm"
            error={error}
            onSubmit={handleConfirmPin}
          />
        )}

        {step === 'removeConfirm' && (
          <PinPad
            prompt="Enter your current PIN to remove it"
            error={error}
            onSubmit={handleRemoveSubmit}
          />
        )}

        {step === 'done' && (
          <div className={styles.done}>
            <p className={styles.doneText}>{successMsg}</p>
            <button className={styles.menuBtn} onClick={onClose}>Done</button>
          </div>
        )}

        {saving && step !== 'done' && <p className={styles.saving}>Saving…</p>}
      </div>
    </Modal>
  );
};

export default PinSettingsModal;
