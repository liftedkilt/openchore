import React, { useState, useEffect } from 'react';
import { Plus } from 'lucide-react';
import Modal from '../Modal/Modal';
import { api } from '../../api';
import type { Chore, User } from '../../types';
import styles from './QuickAssign.module.css';
import clsx from 'clsx';

interface Props {
  isOpen: boolean;
  onClose: () => void;
}

const QuickAssign: React.FC<Props> = ({ isOpen, onClose }) => {
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
    if (dateMode === 'today') return today.toISOString().slice(0, 10);
    if (dateMode === 'tomorrow') {
      const t = new Date(today);
      t.setDate(t.getDate() + 1);
      return t.toISOString().slice(0, 10);
    }
    return customDate;
  };

  const toggleUser = (id: number) => {
    setSelectedUserIds(prev =>
      prev.includes(id) ? prev.filter(x => x !== id) : [...prev, id]
    );
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
      setError(err.message || 'Failed to assign chore');
    } finally {
      setAssigning(false);
    }
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Quick Assign" maxWidth="420px">
      <div className={styles.form}>
        {error && <div className={styles.error}>{error}</div>}

        <div className={styles.section}>
          <label className={styles.label}>Chore</label>
          <select
            className={styles.select}
            value={selectedChoreId}
            onChange={e => {
              const val = e.target.value;
              setSelectedChoreId(val === 'new' ? 'new' : val === '' ? '' : Number(val));
            }}
          >
            <option value="">Pick a chore...</option>
            {chores.map(c => (
              <option key={c.id} value={c.id}>{c.title}</option>
            ))}
            <option value="new">+ New chore...</option>
          </select>

          {selectedChoreId === 'new' && (
            <div className={styles.newChoreFields}>
              <input
                className={styles.input}
                type="text"
                placeholder="Chore name"
                value={newTitle}
                onChange={e => setNewTitle(e.target.value)}
                autoFocus
              />
              <div className={styles.pointsRow}>
                <label className={styles.labelSmall}>Points</label>
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
          <label className={styles.label}>Who</label>
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
          <label className={styles.label}>When</label>
          <div className={styles.datePicker}>
            <button
              className={clsx(styles.dateChip, dateMode === 'today' && styles.dateChipActive)}
              onClick={() => setDateMode('today')}
            >
              Today
            </button>
            <button
              className={clsx(styles.dateChip, dateMode === 'tomorrow' && styles.dateChipActive)}
              onClick={() => setDateMode('tomorrow')}
            >
              Tomorrow
            </button>
            <button
              className={clsx(styles.dateChip, dateMode === 'custom' && styles.dateChipActive)}
              onClick={() => setDateMode('custom')}
            >
              Pick date
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
          {assigning ? 'Assigning...' : 'Assign'}
        </button>
      </div>
    </Modal>
  );
};

export default QuickAssign;
