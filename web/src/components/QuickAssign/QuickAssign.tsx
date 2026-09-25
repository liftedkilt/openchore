import React, { useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import Modal from '../Modal/Modal';
import { api } from '../../api';
import { localDateStr, toggleInArray } from '../../utils';
import type { Chore, User } from '../../types';
import { PersonToggle } from '../admin/pickers';
import styles from './QuickAssign.module.css';

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
    } catch (err: unknown) {
      setError(err instanceof Error && err.message ? err.message : t('dashboard.quickAssign.errorFailed'));
    } finally {
      setAssigning(false);
    }
  };

  const dates: { id: 'today' | 'tomorrow' | 'custom'; label: string }[] = [
    { id: 'today', label: t('dashboard.quickAssign.dateToday') },
    { id: 'tomorrow', label: t('dashboard.quickAssign.dateTomorrow') },
    { id: 'custom', label: t('dashboard.quickAssign.datePickDate') },
  ];

  return (
    <Modal isOpen={isOpen} onClose={onClose} title={t('dashboard.quickAssign.title')} maxWidth="460px">
      <div className={styles.form}>
        {error && <p className={styles.error} role="alert">{error}</p>}

        <div className={styles.section}>
          <label className={styles.label} htmlFor="quick-assign-chore">{t('dashboard.quickAssign.labelChore')}</label>
          <select
            id="quick-assign-chore"
            className={styles.input}
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
                aria-label={t('dashboard.quickAssign.choreNamePlaceholder')}
                value={newTitle}
                onChange={e => setNewTitle(e.target.value)}
                autoFocus
              />
              <label className={styles.pointsRow}>
                <span className={styles.labelSmall}>{t('dashboard.quickAssign.labelPoints')}</span>
                <input
                  className={styles.input}
                  type="number"
                  min={0}
                  value={newPoints}
                  onChange={e => setNewPoints(Number(e.target.value))}
                />
              </label>
            </div>
          )}
        </div>

        <div className={styles.section}>
          <span className={styles.label}>{t('dashboard.quickAssign.labelWho')}</span>
          <div className={styles.chips}>
            {users.map(u => (
              <PersonToggle key={u.id} user={u} pressed={selectedUserIds.includes(u.id)} onClick={() => toggleUser(u.id)} />
            ))}
          </div>
        </div>

        <div className={styles.section}>
          <span className={styles.label}>{t('dashboard.quickAssign.labelWhen')}</span>
          <div className={styles.chips}>
            {dates.map(d => (
              <button
                key={d.id}
                type="button"
                className={styles.chip}
                aria-pressed={dateMode === d.id}
                onClick={() => setDateMode(d.id)}
              >
                {d.label}
              </button>
            ))}
          </div>
          {dateMode === 'custom' && (
            <input
              className={styles.input}
              type="date"
              value={customDate}
              aria-label={t('dashboard.quickAssign.datePickDate')}
              onChange={e => setCustomDate(e.target.value)}
            />
          )}
        </div>

        <button
          type="button"
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
