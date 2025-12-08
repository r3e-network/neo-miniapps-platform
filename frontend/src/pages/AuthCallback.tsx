import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { Shield, Loader2, AlertCircle } from 'lucide-react';
import { useAuthStore } from '../stores/auth';
import { api } from '../api/client';

export function AuthCallback() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { login } = useAuthStore();
  const [error, setError] = useState('');

  useEffect(() => {
    const handleCallback = async () => {
      const token = searchParams.get('token');
      const errorParam = searchParams.get('error');

      if (errorParam) {
        setError(errorParam.replace(/_/g, ' '));
        setTimeout(() => navigate('/login'), 3000);
        return;
      }

      if (!token) {
        setError('No authentication token received');
        setTimeout(() => navigate('/login'), 3000);
        return;
      }

      try {
        api.setToken(token);

        // Fetch user profile to get user data
        const profile = await api.getMe();

        login(
          {
            id: profile.user.id,
            address: profile.user.address || '',
            email: profile.user.email
          },
          token
        );

        navigate('/');
      } catch {
        setError('Failed to authenticate. Please try again.');
        setTimeout(() => navigate('/login'), 3000);
      }
    };

    handleCallback();
  }, [searchParams, navigate, login]);

  return (
    <div className="min-h-screen bg-gray-900 flex items-center justify-center p-4">
      <div className="max-w-md w-full text-center">
        <Shield className="w-16 h-16 text-green-500 mx-auto mb-4" />

        {error ? (
          <>
            <div className="bg-red-500/10 border border-red-500 rounded-lg p-4 mb-4 flex items-center gap-3 justify-center">
              <AlertCircle className="w-5 h-5 text-red-500" />
              <p className="text-red-500">{error}</p>
            </div>
            <p className="text-gray-400 text-sm">Redirecting to login...</p>
          </>
        ) : (
          <>
            <Loader2 className="w-8 h-8 text-green-500 mx-auto mb-4 animate-spin" />
            <h1 className="text-xl font-semibold text-white mb-2">
              Completing Authentication...
            </h1>
            <p className="text-gray-400 text-sm">
              Please wait while we verify your credentials.
            </p>
          </>
        )}
      </div>
    </div>
  );
}
