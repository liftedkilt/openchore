import React, { useEffect, useState } from 'react';
import { Routes, Route, Navigate, useLocation } from 'react-router-dom';
import { useAuth } from './AuthContext';
import { api } from './api';
import { claimAutoSignIn } from './signInRedirect';
import { ProfileSelection } from './pages/ProfileSelection';
import { Dashboard } from './pages/Dashboard';
import { AmbientDashboard } from './pages/AmbientDashboard';
import { useIdleRedirect } from './hooks/useIdleRedirect';

// Admin screens live inside a parent's own profile: sign in as a parent and
// the "Manage" button appears. The server enforces the role on every call;
// this only keeps non-admins off the pages.
// Dev-only design-system gallery at /design. It renders outside the auth
// gate so it works without the API; the import is dropped from prod builds.
const DesignGallery = import.meta.env.DEV ? React.lazy(() => import('./design/gallery/DesignGallery')) : null;
const IDLE_EXEMPT = import.meta.env.DEV ? ['/setup', '/upload', '/design'] : ['/setup', '/upload'];

// Screens only a parent (or first-run setup) opens load on demand, so a kid's
// tablet doesn't download and parse the whole Manage console to show the picker.
const loadAdmin = () => import('./pages/AdminDashboard');
const loadReports = () => import('./pages/Reports');
const AdminDashboard = React.lazy(() => loadAdmin().then(m => ({ default: m.AdminDashboard })));
const Reports = React.lazy(() => loadReports().then(m => ({ default: m.Reports })));
const SetupWizard = React.lazy(() => import('./pages/SetupWizard').then(m => ({ default: m.SetupWizard })));
const PhotoUpload = React.lazy(() => import('./pages/PhotoUpload').then(m => ({ default: m.PhotoUpload })));

const RequireAdmin: React.FC<{ children: React.ReactElement }> = ({ children }) => {
  const { user } = useAuth();
  if (!user) return <Navigate to="/login" replace />;
  if (user.role !== 'admin') return <Navigate to="/" replace />;
  return children;
};

// Where someone who opens the app signed out lands: the family picker, the
// wall display, or straight to a sign-in provider (Manage → Settings →
// Sign-in). /login and /ambient always work, so a shared tablet can be
// pointed at either whatever this is set to.
const SignedOut: React.FC = () => {
  const { signInOptions, signedOutHere } = useAuth();
  const location = useLocation();
  const provider = signInOptions.start_page === 'provider' && !signedOutHere ? signInOptions.start_provider : undefined;
  // Decided once per visit, so a re-render doesn't re-check the loop guard.
  const [redirect] = useState(() => !!provider && claimAutoSignIn());

  useEffect(() => {
    if (redirect && provider) {
      window.location.replace(api.auth.oidcLoginURL(provider, undefined, location.pathname + location.search));
    }
  }, [redirect, provider, location.pathname, location.search]);

  if (redirect) return null;
  return <Navigate to={signInOptions.start_page === 'wall' ? '/ambient' : '/login'} replace />;
};

export const App: React.FC = () => {
  const { user, session, isLoading, signOut } = useAuth();
  const location = useLocation();
  // Shared-device (tap/PIN) sessions end after a few idle minutes and the
  // tablet falls back to the wall display. Personal-device (OIDC) sessions
  // persist.
  useIdleRedirect('/ambient', IDLE_EXEMPT, { onIdle: signOut, disabled: !!session?.persistent });
  // Fetch Manage and Reports in the background once a parent is in. Router
  // navigations are transitions, so a screen still loading would leave the
  // old one up under the new URL; prefetched, they open at once.
  const isAdmin = user?.role === 'admin';
  useEffect(() => {
    if (!isAdmin) return;
    loadAdmin().catch(() => {});
    loadReports().catch(() => {});
  }, [isAdmin]);

  if (DesignGallery && location.pathname === '/design') {
    return <React.Suspense fallback={null}><DesignGallery /></React.Suspense>;
  }

  if (isLoading) return null;

  return (
    <React.Suspense fallback={null}>
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
          element={user ? <Dashboard /> : <SignedOut />}
        />
      </Routes>
    </React.Suspense>
  );
};
