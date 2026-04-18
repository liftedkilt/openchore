import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../api';
import { useAuth } from '../AuthContext';
import type { User } from '../types';
import styles from './AdminDashboard.module.css';
import { ArrowLeft, Plus, Users, ListChecks, Gift, Coins, Activity, Settings, Undo2, Camera, Home } from 'lucide-react';
import clsx from 'clsx';
import QuickAssign from '../components/QuickAssign/QuickAssign';
import { ChoresTab } from '../components/admin/ChoresTab';
import { ApprovalsTab } from '../components/admin/ApprovalsTab';
import { UsersTab } from '../components/admin/UsersTab';
import { RewardsTab } from '../components/admin/RewardsTab';
import { PointsTab } from '../components/admin/PointsTab';
import { ActivityTab } from '../components/admin/ActivityTab';
import { AIChoreChecker } from '../components/admin/AIChoreChecker';
import { SettingsTab } from '../components/admin/SettingsTab';
import { KidsStatusTab } from '../components/admin/KidsStatusTab';

type Tab = 'kids-status' | 'chores' | 'approvals' | 'users' | 'rewards' | 'points' | 'activity' | 'ai' | 'settings';

export const AdminDashboard: React.FC = () => {
  const navigate = useNavigate();
  const { setUser } = useAuth();
  const [tab, setTab] = useState<Tab>('kids-status');
  const [ready, setReady] = useState(false);
  const [pendingCount, setPendingCount] = useState(0);
  const [quickAssignOpen, setQuickAssignOpen] = useState(false);

  // Fetch pending count periodically
  useEffect(() => {
    if (!ready) return;
    const fetchCount = () => api.chores.listPending().then(p => setPendingCount(p.length)).catch(() => {});
    fetchCount();
    const interval = setInterval(fetchCount, 30000);
    return () => clearInterval(interval);
  }, [ready]);

  // Clear admin session when navigating away via browser back button
  useEffect(() => {
    const handlePopState = () => {
      sessionStorage.removeItem('openchore_admin');
    };
    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  }, []);

  useEffect(() => {
    const ensureAdminUser = async () => {
      if (!sessionStorage.getItem('openchore_admin')) {
        navigate('/admin', { replace: true });
        return;
      }
      try {
        const users = await api.users.list();
        const admin = users.find((u: User) => u.role === 'admin');
        if (admin) {
          setUser(admin);
          setReady(true);
        } else {
          // No admin exists — redirect to setup
          setUser(null);
          sessionStorage.removeItem('openchore_admin');
          navigate('/setup', { replace: true });
        }
      } catch {
        navigate('/login', { replace: true });
      }
    };
    ensureAdminUser();
  }, [navigate, setUser]);

  // Block render if not authenticated (synchronous check + useEffect redirect)
  if (!ready || !sessionStorage.getItem('openchore_admin')) return null;

  return (
    <div className={styles.wrapper}>
      <header className={styles.header}>
        <button className={styles.backBtn} onClick={() => { sessionStorage.removeItem('openchore_admin'); navigate('/login'); }}>
          <ArrowLeft size={18} />
        </button>
        <h1 className={styles.title}>Admin</h1>
        <button className={styles.btnSmall} style={{ marginLeft: 'auto' }} onClick={() => navigate('/admin/reports')}>
          Reports
        </button>
      </header>

      <nav className={styles.nav}>
        <button className={clsx(styles.navItem, tab === 'kids-status' && styles.navItemActive)} onClick={() => setTab('kids-status')}>
          <Home size={16} /> Kids
        </button>
        <button className={clsx(styles.navItem, tab === 'chores' && styles.navItemActive)} onClick={() => setTab('chores')}>
          <ListChecks size={16} /> Chores
        </button>
        <button className={clsx(styles.navItem, tab === 'approvals' && styles.navItemActive)} onClick={() => setTab('approvals')}>
          <Activity size={16} />
          Approvals
          {pendingCount > 0 && <span className={styles.navBadge}>{pendingCount}</span>}
        </button>
        <button className={clsx(styles.navItem, tab === 'rewards' && styles.navItemActive)} onClick={() => setTab('rewards')}>
          <Gift size={16} /> Rewards
        </button>
        <button className={clsx(styles.navItem, tab === 'points' && styles.navItemActive)} onClick={() => setTab('points')}>
          <Coins size={16} /> Points
        </button>
        <button className={clsx(styles.navItem, tab === 'activity' && styles.navItemActive)} onClick={() => setTab('activity')}>
          <Undo2 size={16} /> Log
        </button>
        <button className={clsx(styles.navItem, tab === 'users' && styles.navItemActive)} onClick={() => setTab('users')}>
          <Users size={16} /> People
        </button>
        <button className={clsx(styles.navItem, tab === 'ai' && styles.navItemActive)} onClick={() => setTab('ai')}>
          <Camera size={16} /> AI
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
        {tab === 'ai' && <AIChoreChecker />}
        {tab === 'settings' && <SettingsTab />}
      </main>

      <button className={styles.fab} onClick={() => setQuickAssignOpen(true)} title="Quick Assign">
        <Plus size={24} />
      </button>

      <QuickAssign isOpen={quickAssignOpen} onClose={() => setQuickAssignOpen(false)} />
    </div>
  );
};
