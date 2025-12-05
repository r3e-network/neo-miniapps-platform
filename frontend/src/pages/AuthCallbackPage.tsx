import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuthStore } from '@/stores/authStore';

export function AuthCallbackPage() {
  const navigate = useNavigate();
  const { initialize, isAuthenticated } = useAuthStore();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const handleCallback = async () => {
      try {
        // Initialize auth - this will pick up the session from the URL
        await initialize();

        // Small delay to ensure state is updated
        setTimeout(() => {
          if (isAuthenticated) {
            navigate('/dashboard', { replace: true });
          } else {
            // Check URL for error
            const params = new URLSearchParams(window.location.search);
            const errorParam = params.get('error');
            const errorDescription = params.get('error_description');

            if (errorParam) {
              setError(errorDescription || errorParam);
            } else {
              // No error but not authenticated - might need to wait
              navigate('/dashboard', { replace: true });
            }
          }
        }, 500);
      } catch (err) {
        console.error('Auth callback error:', err);
        setError('Authentication failed. Please try again.');
      }
    };

    handleCallback();
  }, [initialize, isAuthenticated, navigate]);

  if (error) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-surface-50">
        <div className="text-center">
          <div className="w-16 h-16 mx-auto bg-red-100 rounded-full flex items-center justify-center mb-4">
            <svg className="w-8 h-8 text-red-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </div>
          <h2 className="text-xl font-semibold text-surface-900 mb-2">Authentication Failed</h2>
          <p className="text-surface-600 mb-4">{error}</p>
          <button
            onClick={() => navigate('/login')}
            className="px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
          >
            Back to Login
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-surface-50">
      <div className="text-center">
        <div className="w-16 h-16 mx-auto mb-4">
          <svg className="animate-spin w-full h-full text-primary-600" fill="none" viewBox="0 0 24 24">
            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
          </svg>
        </div>
        <h2 className="text-xl font-semibold text-surface-900">Completing sign in...</h2>
        <p className="text-surface-600 mt-2">Please wait while we verify your credentials.</p>
      </div>
    </div>
  );
}
