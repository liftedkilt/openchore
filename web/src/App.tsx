import React from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { useAuth } from './AuthContext';
import { ProfileSelection } from './pages/ProfileSelection';
import { Dashboard } from './pages/Dashboard';
import { AdminDashboard } from './pages/AdminDashboard';
import { AmbientDashboard } from './pages/AmbientDashboard';
import { Reports } from './pages/Reports';
import { SetupWizard } from './pages/SetupWizard';
import { PhotoUpload } from './pages/PhotoUpload';
import { useIdleRedirect } from './hooks/useIdleRedirect';

// Admin screens live inside a parent's own profile: sign in as a parent and
// the "Manage" button appears. The server enforces the role on every call;
// this only keeps non-admins off the pages.
const RequireAdmin: React.FC<{ children: React.ReactElement }> = ({ children }) => {
  const { user } = useAuth();
  if (!user) return <Navigate to="/login" replace />;
  if (user.role !== 'admin') return <Navigate to="/" replace />;
  return children;
};

export const App: React.FC = () => {
  const { user, session, isLoading, signOut } = useAuth();
  // Shared-device (tap/PIN) sessions end after a few idle minutes and the
  // tablet falls back to the wall display. Personal-device (OIDC) sessions
  // persist.
  useIdleRedirect('/ambient', ['/setup', '/upload'], { onIdle: signOut, disabled: !!session?.persistent });

  if (isLoading) return null;

  return (
    <Routes>
      <Route path="/login" element={<ProfileSelection />} />
      <Route path="/setup" element={<SetupWizard />} />
      <Route path="/upload" element={<PhotoUpload />} />
      <Route path="/admin" element={<Navigate to="/admin/dashboard" replace />} />
      <Route path="/admin/dashboard" element={
        <RequireAdmin><AdminDashboard /></RequireAdmin>
      } />
      <Route path="/admin/reports" element={
        <RequireAdmin><Reports /></RequireAdmin>
      } />
      <Route path="/ambient" element={<AmbientDashboard />} />
      <Route
        path="/*"
        element={user ? <Dashboard /> : <Navigate to="/login" />}
      />
    </Routes>
  );
};
