import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { api } from '../api';
import { useAuth } from '../AuthContext';
import styles from './AdminDashboard.module.css';
import { ArrowLeft, Plus, Users, ListChecks, Gift, Coins, Activity, Settings, Undo2, Home } from 'lucide-react';
import clsx from 'clsx';
import QuickAssign from '../components/QuickAssign/QuickAssign';
import { ChoresTab } from '../components/admin/ChoresTab';
import { ApprovalsTab } from '../components/admin/ApprovalsTab';
import { UsersTab } from '../components/admin/UsersTab';
import { RewardsTab } from '../components/admin/RewardsTab';
import { PointsTab } from '../components/admin/PointsTab';
import { ActivityTab } from '../components/admin/ActivityTab';
import { SettingsTab } from '../components/admin/SettingsTab';
import { KidsStatusTab } from '../components/admin/KidsStatusTab';
import { LanguageSelector } from '../components/LanguageSelector/LanguageSelector';

type Tab = 'kids-status' | 'chores' | 'approvals' | 'users' | 'rewards' | 'points' | 'activity' | 'settings';

export const AdminDashboard: React.FC = () => {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const { user } = useAuth();
  const [tab, setTab] = useState<Tab>('kids-status');
  const [pendingCount, setPendingCount] = useState(0);
  const [quickAssignOpen, setQuickAssignOpen] = useState(false);

  // Fetch pending count periodically
  useEffect(() => {
    const fetchCount = () => api.chores.listPending().then(p => setPendingCount(p.length)).catch(() => {});
    fetchCount();
    const interval = setInterval(fetchCount, 30000);
    return () => clearInterval(interval);
  }, []);

  return (
    <div className={styles.wrapper}>
      <header className={styles.header}>
        <button className={styles.backBtn} onClick={() => navigate('/')} aria-label={t('admin.dashboard.backToMyChores', { name: user?.name ?? '' })} title={t('admin.dashboard.backToMyChores', { name: user?.name ?? '' })}>
          <ArrowLeft size={18} />
        </button>
        <h1 className={styles.title}>{t('admin.dashboard.title')}</h1>
        <LanguageSelector />
        <button className={styles.btnSmall} style={{ marginLeft: 'auto' }} onClick={() => navigate('/admin/reports')}>
          {t('admin.dashboard.reports')}
        </button>
      </header>

      <nav className={styles.nav}>
        <button className={clsx(styles.navItem, tab === 'kids-status' && styles.navItemActive)} onClick={() => setTab('kids-status')}>
          <Home size={16} /> {t('admin.dashboard.tabKids')}
        </button>
        <button className={clsx(styles.navItem, tab === 'chores' && styles.navItemActive)} onClick={() => setTab('chores')}>
          <ListChecks size={16} /> {t('admin.dashboard.tabChores')}
        </button>
        <button className={clsx(styles.navItem, tab === 'approvals' && styles.navItemActive)} onClick={() => setTab('approvals')}>
          <Activity size={16} />
          {t('admin.dashboard.tabApprovals')}
          {pendingCount > 0 && <span className={styles.navBadge}>{pendingCount}</span>}
        </button>
        <button className={clsx(styles.navItem, tab === 'rewards' && styles.navItemActive)} onClick={() => setTab('rewards')}>
          <Gift size={16} /> {t('admin.dashboard.tabRewards')}
        </button>
        <button className={clsx(styles.navItem, tab === 'points' && styles.navItemActive)} onClick={() => setTab('points')}>
          <Coins size={16} /> {t('admin.dashboard.tabPoints')}
        </button>
        <button className={clsx(styles.navItem, tab === 'activity' && styles.navItemActive)} onClick={() => setTab('activity')}>
          <Undo2 size={16} /> {t('admin.dashboard.tabLog')}
        </button>
        <button className={clsx(styles.navItem, tab === 'users' && styles.navItemActive)} onClick={() => setTab('users')}>
          <Users size={16} /> {t('admin.dashboard.tabPeople')}
        </button>
        <button className={clsx(styles.navItem, tab === 'settings' && styles.navItemActive)} onClick={() => setTab('settings')}>
          <Settings size={16} />
        </button>
      </nav>

      <main className={styles.content}>
        {tab === 'kids-status' && <KidsStatusTab />}
        {tab === 'chores' && <ChoresTab />}
        {tab === 'approvals' && <ApprovalsTab onCountChange={setPendingCount} />}
        {tab === 'users' && <UsersTab />}
        {tab === 'rewards' && <RewardsTab />}
        {tab === 'points' && <PointsTab />}
        {tab === 'activity' && <ActivityTab />}
        {tab === 'settings' && <SettingsTab />}
      </main>

      <button className={styles.fab} onClick={() => setQuickAssignOpen(true)} title={t('admin.dashboard.quickAssign')}>
        <Plus size={24} />
      </button>

      <QuickAssign isOpen={quickAssignOpen} onClose={() => setQuickAssignOpen(false)} />
    </div>
  );
};
