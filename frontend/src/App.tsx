import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Layout } from './components/Layout';
import { Dashboard } from './pages/Dashboard';
import { Services } from './pages/Services';
import { Secrets } from './pages/Secrets';
import { GasBank } from './pages/GasBank';
import { Automation } from './pages/Automation';
import { Settings } from './pages/Settings';
import { Mixer } from './pages/Mixer';
import { Login } from './pages/Login';
import { AuthCallback } from './pages/AuthCallback';
import { useAuthStore } from './stores/auth';

const queryClient = new QueryClient();

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated } = useAuthStore();
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }
  return <>{children}</>;
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/auth/callback" element={<AuthCallback />} />
          <Route
            path="/*"
            element={
              <ProtectedRoute>
                <Layout>
                  <Routes>
                    <Route path="/" element={<Dashboard />} />
                    <Route path="/services" element={<Services />} />
                    <Route path="/mixer" element={<Mixer />} />
                    <Route path="/secrets" element={<Secrets />} />
                    <Route path="/gasbank" element={<GasBank />} />
                    <Route path="/automation" element={<Automation />} />
                    <Route path="/settings" element={<Settings />} />
                  </Routes>
                </Layout>
              </ProtectedRoute>
            }
          />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
