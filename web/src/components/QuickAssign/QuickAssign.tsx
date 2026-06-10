import React, { useState, useEffect } from 'react';
import { Plus } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import Modal from '../Modal/Modal';
import { api } from '../../api';
import { localDateStr, toggleInArray } from '../../utils';
import type { Chore, User } from '../../types';
import styles from './QuickAssign.module.css';
import clsx from 'clsx';

interface Props {
  isOpen: boolean;
  onClose: () => void;
}

const QuickAssign: React.FC<Props> = ({ isOpen, onClose }) => {
  const { t } = useTranslation();
  const [chores, setChores] = useState<Chore[]>([]);
  const [users, setUsers] = useState<User[]>([]);

  const [selectedChoreId, setSelectedChoreId] = useState<number | 'new' | ''>('');
  const [newTitle, setNewTitle] = useState('');
  const [newPoints, setNewPoints] = useState(5);
  const [selectedUserIds, setSelectedUserIds] = useState<number[]>([]);
  const [dateMode, setDateMode] = useState<'today' | 'tomorrow' | 'custom'>('today');
  const [customDate, setCustomDate] = useState('');

  const [assigning, setAssigning] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!isOpen) return;
    Promise.all([api.chores.list(), api.users.list()]).then(([c, u]) => {
      setChores(c);
      setUsers(u);
    });
    setSelectedChoreId('');
    setNewTitle('');
    setNewPoints(5);
    setSelectedUserIds([]);
    setDateMode('today');
    setCustomDate('');
    setError('');
  }, [isOpen]);

  const getDateString = (): string => {
    const today = new Date();
    if (dateMode === 'today') return localDateStr(today);
    if (dateMode === 'tomorrow') {
      const t = new Date(today);
      t.setDate(t.getDate() + 1);
      return localDateStr(t);
    }
    return customDate;
  };

  const toggleUser = (id: number) => {
    setSelectedUserIds(prev => toggleInArray(prev, id));
  };

  const canAssign =
    (selectedChoreId === 'new' ? newTitle.trim().length > 0 : selectedChoreId !== '') &&
    selectedUserIds.length > 0 &&
    (dateMode !== 'custom' || customDate);

  const handleAssign = async () => {
    if (!canAssign) return;
    setAssigning(true);
    setError('');
    try {
      let choreId: number;
      if (selectedChoreId === 'new') {
        const created = await api.chores.create({
          title: newTitle.trim(),
          points_value: newPoints,
          category: 'bonus',
        });
        choreId = created.id;
      } else {
        choreId = selectedChoreId as number;
      }

      const dateString = getDateString();
      await Promise.all(
        selectedUserIds.map(userId =>
          api.chores.createSchedule(choreId, {
            assigned_to: userId,
            specific_date: dateString,
          })
        )
      );
      onClose();
    } catch (err: any) {
      setError(err.message || t('dashboard.quickAssign.errorFailed'));
    } finally {
      setAssigning(false);
    }
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} title={t('dashboard.quickAssign.title')} maxWidth="420px">
      <div className={styles.form}>
        {error && <div className={styles.error}>{error}</div>}

        <div className={styles.section}>
          <label className={styles.label}>{t('dashboard.quickAssign.labelChore')}</label>
          <select
            className={styles.select}
            value={selectedChoreId}
            onChange={e => {
              const val = e.target.value;
              setSelectedChoreId(val === 'new' ? 'new' : val === '' ? '' : Number(val));
            }}
          >
            <option value="">{t('dashboard.quickAssign.chorePlaceholder')}</option>
            {chores.map(c => (
              <option key={c.id} value={c.id}>{c.title}</option>
            ))}
            <option value="new">{t('dashboard.quickAssign.choreNewOption')}</option>
          </select>

          {selectedChoreId === 'new' && (
            <div className={styles.newChoreFields}>
              <input
                className={styles.input}
                type="text"
                placeholder={t('dashboard.quickAssign.choreNamePlaceholder')}
                value={newTitle}
                onChange={e => setNewTitle(e.target.value)}
                autoFocus
              />
              <div className={styles.pointsRow}>
                <label className={styles.labelSmall}>{t('dashboard.quickAssign.labelPoints')}</label>
                <input
                  className={clsx(styles.input, styles.pointsInput)}
                  type="number"
                  min={0}
                  value={newPoints}
                  onChange={e => setNewPoints(Number(e.target.value))}
                />
              </div>
            </div>
          )}
        </div>

        <div className={styles.section}>
          <label className={styles.label}>{t('dashboard.quickAssign.labelWho')}</label>
          <div className={styles.avatarPicker}>
            {users.map(u => (
              <button
                key={u.id}
                className={clsx(styles.avatarBubble, selectedUserIds.includes(u.id) && styles.avatarBubbleActive)}
                onClick={() => toggleUser(u.id)}
              >
                {u.avatar_url
                  ? <img src={u.avatar_url} alt={u.name} className={styles.avatarImg} />
                  : <div className={styles.avatarPlaceholder}>{u.name[0]}</div>
                }
                <span className={styles.avatarName}>{u.name}</span>
              </button>
            ))}
          </div>
        </div>

        <div className={styles.section}>
          <label className={styles.label}>{t('dashboard.quickAssign.labelWhen')}</label>
          <div className={styles.datePicker}>
            <button
              className={clsx(styles.dateChip, dateMode === 'today' && styles.dateChipActive)}
              onClick={() => setDateMode('today')}
            >
              {t('dashboard.quickAssign.dateToday')}
            </button>
            <button
              className={clsx(styles.dateChip, dateMode === 'tomorrow' && styles.dateChipActive)}
              onClick={() => setDateMode('tomorrow')}
            >
              {t('dashboard.quickAssign.dateTomorrow')}
            </button>
            <button
              className={clsx(styles.dateChip, dateMode === 'custom' && styles.dateChipActive)}
              onClick={() => setDateMode('custom')}
            >
              {t('dashboard.quickAssign.datePickDate')}
            </button>
          </div>
          {dateMode === 'custom' && (
            <input
              className={styles.input}
              type="date"
              value={customDate}
              onChange={e => setCustomDate(e.target.value)}
            />
          )}
        </div>

        <button
          className={styles.assignBtn}
          disabled={!canAssign || assigning}
          onClick={handleAssign}
        >
          {assigning ? t('dashboard.quickAssign.btnAssigning') : t('dashboard.quickAssign.btnAssign')}
        </button>
      </div>
    </Modal>
  );
};

export default QuickAssign;
