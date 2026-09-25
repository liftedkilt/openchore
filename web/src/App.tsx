import React from 'react';
import { Routes, Route, Navigate, useLocation } from 'react-router-dom';
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
// Dev-only design-system gallery at /design. It renders outside the auth
// gate so it works without the API; the import is dropped from prod builds.
const DesignGallery = import.meta.env.DEV ? React.lazy(() => import('./design/gallery/DesignGallery')) : null;
const IDLE_EXEMPT = import.meta.env.DEV ? ['/setup', '/upload', '/design'] : ['/setup', '/upload'];

const RequireAdmin: React.FC<{ children: React.ReactElement }> = ({ children }) => {
  const { user } = useAuth();
  if (!user) return <Navigate to="/login" replace />;
  if (user.role !== 'admin') return <Navigate to="/" replace />;
  return children;
};

export const App: React.FC = () => {
  const { user, session, isLoading, signOut } = useAuth();
  const location = useLocation();
  // Shared-device (tap/PIN) sessions end after a few idle minutes and the
  // tablet falls back to the wall display. Personal-device (OIDC) sessions
  // persist.
  useIdleRedirect('/ambient', IDLE_EXEMPT, { onIdle: signOut, disabled: !!session?.persistent });

  if (DesignGallery && location.pathname === '/design') {
    return <React.Suspense fallback={null}><DesignGallery /></React.Suspense>;
  }

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
