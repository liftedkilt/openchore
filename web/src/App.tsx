import React from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { useAuth } from './AuthContext';
import { ProfileSelection } from './pages/ProfileSelection';
import { Dashboard } from './pages/Dashboard';
import { AdminPasscode } from './pages/AdminPasscode';
import { AdminDashboard } from './pages/AdminDashboard';
import { AmbientDashboard } from './pages/AmbientDashboard';
import { useIdleRedirect } from './hooks/useIdleRedirect';

const RequireAdmin: React.FC<{ children: React.ReactElement }> = ({ children }) => {
  if (!sessionStorage.getItem('openchore_admin')) {
    return <Navigate to="/admin" replace />;
  }
  return children;
};

export const App: React.FC = () => {
  const { user, isLoading } = useAuth();
  useIdleRedirect('/ambient');

  if (isLoading) return null;

  return (
    <Routes>
      <Route path="/login" element={<ProfileSelection />} />
      <Route path="/admin" element={<AdminPasscode />} />
      <Route path="/admin/dashboard" element={
        <RequireAdmin><AdminDashboard /></RequireAdmin>
      } />
      <Route path="/ambient" element={<AmbientDashboard />} />
      <Route
        path="/*"
        element={user ? <Dashboard /> : <Navigate to="/login" />}
      />
    </Routes>
  );
};
