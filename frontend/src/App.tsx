import { useEffect } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { Toaster } from 'react-hot-toast';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

// Layout
import { Layout } from '@/components/layout/Layout';

// Auth
import { useAuthStore } from '@/stores/authStore';

// Pages
import { LoginPage } from '@/pages/LoginPage';
import { RegisterPage } from '@/pages/RegisterPage';
import { AuthCallbackPage } from '@/pages/AuthCallbackPage';
import { DashboardPage } from '@/pages/DashboardPage';
import { ServicesPage } from '@/pages/ServicesPage';
import { SecretsPage } from '@/pages/SecretsPage';
import { GasBankPage } from '@/pages/GasBankPage';
import { APIKeysPage } from '@/pages/APIKeysPage';
import { SettingsPage } from '@/pages/SettingsPage';
import { HomePage } from '@/pages/HomePage';
import { DocsPage } from '@/pages/DocsPage';
import { PlaygroundPage } from '@/pages/PlaygroundPage';

// Service-specific pages
import { OracleServicePage } from '@/pages/services/OracleServicePage';
import { VRFServicePage } from '@/pages/services/VRFServicePage';
import { DataFeedsServicePage } from '@/pages/services/DataFeedsServicePage';
import { AutomationServicePage } from '@/pages/services/AutomationServicePage';
import { SecretsServicePage } from '@/pages/services/SecretsServicePage';
import { MixerServicePage } from '@/pages/services/MixerServicePage';
import { AccountsServicePage } from '@/pages/services/AccountsServicePage';
import { CCIPServicePage } from '@/pages/services/CCIPServicePage';
import { ConfidentialServicePage } from '@/pages/services/ConfidentialServicePage';
import { CREServicePage } from '@/pages/services/CREServicePage';
import { DataLinkServicePage } from '@/pages/services/DataLinkServicePage';
import { DataStreamsServicePage } from '@/pages/services/DataStreamsServicePage';
import { DTAServicePage } from '@/pages/services/DTAServicePage';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 1000 * 60 * 5, // 5 minutes
      retry: 1,
    },
  },
});

function AppContent() {
  const { initialize, isInitialized } = useAuthStore();

  useEffect(() => {
    initialize();
  }, [initialize]);

  // Show loading while initializing auth
  if (!isInitialized) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-surface-50">
        <div className="text-center">
          <div className="w-12 h-12 mx-auto mb-4">
            <svg className="animate-spin w-full h-full text-primary-600" fill="none" viewBox="0 0 24 24">
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
            </svg>
          </div>
          <p className="text-surface-600">Loading...</p>
        </div>
      </div>
    );
  }

  return (
    <Routes>
      {/* Public routes */}
      <Route path="/" element={<HomePage />} />
      <Route path="/login" element={<LoginPage />} />
      <Route path="/register" element={<RegisterPage />} />
      <Route path="/auth/callback" element={<AuthCallbackPage />} />
      <Route path="/docs" element={<DocsPage />} />
      <Route path="/docs/:section" element={<DocsPage />} />
      <Route path="/playground" element={<PlaygroundPage />} />

      {/* Protected routes */}
      <Route element={<Layout />}>
        <Route path="/dashboard" element={<DashboardPage />} />
        <Route path="/services" element={<ServicesPage />} />
        <Route path="/services/oracle" element={<OracleServicePage />} />
        <Route path="/services/vrf" element={<VRFServicePage />} />
        <Route path="/services/datafeeds" element={<DataFeedsServicePage />} />
        <Route path="/services/automation" element={<AutomationServicePage />} />
        <Route path="/services/secrets" element={<SecretsServicePage />} />
        <Route path="/services/mixer" element={<MixerServicePage />} />
        <Route path="/services/accounts" element={<AccountsServicePage />} />
        <Route path="/services/ccip" element={<CCIPServicePage />} />
        <Route path="/services/confidential" element={<ConfidentialServicePage />} />
        <Route path="/services/cre" element={<CREServicePage />} />
        <Route path="/services/datalink" element={<DataLinkServicePage />} />
        <Route path="/services/datastreams" element={<DataStreamsServicePage />} />
        <Route path="/services/dta" element={<DTAServicePage />} />
        <Route path="/secrets" element={<SecretsPage />} />
        <Route path="/gasbank" element={<GasBankPage />} />
        <Route path="/apikeys" element={<APIKeysPage />} />
        <Route path="/settings" element={<SettingsPage />} />
      </Route>

      {/* Fallback */}
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <Toaster
        position="top-right"
        toastOptions={{
          duration: 4000,
          style: {
            background: '#1e293b',
            color: '#f8fafc',
            borderRadius: '0.75rem',
          },
          success: {
            iconTheme: {
              primary: '#22c55e',
              secondary: '#f8fafc',
            },
          },
          error: {
            iconTheme: {
              primary: '#ef4444',
              secondary: '#f8fafc',
            },
          },
        }}
      />
      <AppContent />
    </QueryClientProvider>
  );
}

export default App;
