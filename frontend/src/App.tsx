import { useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Layout } from './components/Layout';
import { Dashboard } from './pages/Dashboard';
import { Services } from './pages/Services';
import { Secrets } from './pages/Secrets';
import { GasBank } from './pages/GasBank';
import { NeoFlow } from './pages/NeoFlow';
import { Settings } from './pages/Settings';
import { NeoVault } from './pages/NeoVault';
import { VRF } from './pages/VRF';
import { Login } from './pages/Login';
import { AuthCallback } from './pages/AuthCallback';
import { api } from './api/client';
import { useAuthStore } from './stores/auth';

const queryClient = new QueryClient();

function ProtectedLayout() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }
  return <Layout />;
}

export default function App() {
  const token = useAuthStore((s) => s.token);
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const login = useAuthStore((s) => s.login);

  useEffect(() => {
    api.setToken(token);
  }, [token]);

  useEffect(() => {
    if (isAuthenticated) return;

    let canceled = false;
    (async () => {
      try {
        api.setToken(null);
        const profile = await api.getMe();
        if (canceled) return;
        login(
          {
            id: profile.user.id,
            address: profile.user.address || '',
            email: profile.user.email,
          },
          null
        );
      } catch {
        // No active cookie session (or cookie auth disabled) - user must login.
      }
    })();

    return () => {
      canceled = true;
    };
  }, [isAuthenticated, login]);

  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/auth/callback" element={<AuthCallback />} />
          <Route element={<ProtectedLayout />}>
            <Route path="/" element={<Dashboard />} />
            <Route path="/services" element={<Services />} />
            <Route path="/neovault" element={<NeoVault />} />
            <Route path="/neorand" element={<VRF />} />
            <Route path="/neostore" element={<Secrets />} />
            <Route path="/gasbank" element={<GasBank />} />
            <Route path="/neoflow" element={<NeoFlow />} />
            <Route path="/settings" element={<Settings />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
