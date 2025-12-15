import { useState, useEffect } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { Shield, Wallet, AlertCircle, ExternalLink } from 'lucide-react';
import { useAuthStore } from '../stores/auth';
import { api, API_BASE, type WalletAuthPayload } from '../api/client';

// NeoLine wallet types
declare global {
  interface Window {
    NEOLine?: {
      getAccount: () => Promise<{ address: string; label: string }>;
      signMessage: (params: { message: string }) => Promise<{ publicKey: string; data: string; salt: string; message: string }>;
    };
    NEOLineN3?: {
      getAccount: () => Promise<{ address: string; label: string }>;
      signMessage: (params: { message: string }) => Promise<{ publicKey: string; data: string; salt: string; message: string }>;
    };
    neo3Dapi?: {
      getAccount: () => Promise<{ address: string; label: string }>;
      signMessage: (params: { message: string }) => Promise<{ publicKey: string; data: string; salt: string; message: string }>;
    };
  }
}

type WalletType = 'neoline' | 'o3';

const normalizeHex = (value: string) => {
  const trimmed = value.trim();
  if (/^0x[0-9a-fA-F]+$/.test(trimmed)) return trimmed.slice(2);
  return trimmed;
};

export function Login() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { login, isAuthenticated } = useAuthStore();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [walletAvailable, setWalletAvailable] = useState<{ neoline: boolean; o3: boolean }>({
    neoline: false,
    o3: false,
  });

  // Handle OAuth error from URL params
  useEffect(() => {
    const errorParam = searchParams.get('error');
    if (errorParam) {
      setError(errorParam.replace(/_/g, ' '));
    }
  }, [searchParams]);

  // Check for available wallets
  useEffect(() => {
    const checkWallets = () => {
      setWalletAvailable({
        neoline: !!(window.NEOLine || window.NEOLineN3),
        o3: !!window.neo3Dapi,
      });
    };

    // Check immediately
    checkWallets();

    // Also check after a delay (wallets may inject later)
    const timer = setTimeout(checkWallets, 1000);
    return () => clearTimeout(timer);
  }, []);

  // Redirect if already authenticated
  useEffect(() => {
    if (isAuthenticated) {
      navigate('/');
    }
  }, [isAuthenticated, navigate]);

  const connectNeoLine = async (): Promise<WalletAuthPayload> => {
    const neoline = window.NEOLineN3 || window.NEOLine;
    if (!neoline) {
      throw new Error('NeoLine wallet not found. Please install NeoLine extension.');
    }

    // Get account
    const account = await neoline.getAccount();
    const address = account.address;

    // Get nonce from server
    const nonceResponse = await api.getNonce(address);

    // Sign the message
    const signResult = await neoline.signMessage({ message: nonceResponse.message });

    return {
      address,
      signature: normalizeHex(signResult.data),
      message: nonceResponse.message,
      publicKey: normalizeHex(signResult.publicKey),
      nonce: nonceResponse.nonce,
    };
  };

  const connectO3 = async (): Promise<WalletAuthPayload> => {
    const o3 = window.neo3Dapi;
    if (!o3) {
      throw new Error('O3 wallet not found. Please install O3 extension.');
    }

    // Get account
    const account = await o3.getAccount();
    const address = account.address;

    // Get nonce from server
    const nonceResponse = await api.getNonce(address);

    // Sign the message
    const signResult = await o3.signMessage({ message: nonceResponse.message });

    return {
      address,
      signature: normalizeHex(signResult.data),
      message: nonceResponse.message,
      publicKey: normalizeHex(signResult.publicKey),
      nonce: nonceResponse.nonce,
    };
  };

  const handleOAuthLogin = (provider: 'google' | 'github') => {
    window.location.href = `${API_BASE}/auth/${provider}`;
  };

  const handleConnect = async (walletType: WalletType) => {
    setLoading(true);
    setError('');

    try {
      let walletData: WalletAuthPayload;

      switch (walletType) {
        case 'neoline':
          walletData = await connectNeoLine();
          break;
        case 'o3':
          walletData = await connectO3();
          break;
        default:
          throw new Error('Unknown wallet type');
      }

      // Try to login first, if user doesn't exist, register
      let result;
      try {
        result = await api.login(walletData);
      } catch {
        // User doesn't exist, register
        result = await api.register(walletData);
      }

      // Prefer cookie-based auth if available (more secure in browsers). If it
      // fails (e.g. server has cookie auth disabled), fall back to bearer JWT.
      api.setToken(null);
      try {
        const profile = await api.getMe();
        login(
          {
            id: profile.user.id,
            address: profile.user.address || result.address,
            email: profile.user.email,
          },
          null
        );
      } catch {
        api.setToken(result.token);
        login({ id: result.user_id, address: result.address }, result.token);
      }

      navigate('/');
    } catch (err) {
      console.error('Connection error:', err);
      setError(err instanceof Error ? err.message : 'Connection failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-gray-900 flex items-center justify-center p-4">
      <div className="max-w-md w-full">
        <div className="text-center mb-8">
          <Shield className="w-16 h-16 text-green-500 mx-auto mb-4" />
          <h1 className="text-3xl font-bold text-white">Neo Service Layer</h1>
          <p className="text-gray-400 mt-2">
            Secure, TEE-protected services for Neo N3
          </p>
        </div>

        <div className="bg-gray-800 rounded-xl p-8 border border-gray-700">
          <h2 className="text-xl font-semibold text-white mb-6 text-center">
            Connect Your Wallet
          </h2>

          {error && (
            <div className="bg-red-500/10 border border-red-500 rounded-lg p-3 mb-4 flex items-start gap-2">
              <AlertCircle className="w-5 h-5 text-red-500 flex-shrink-0 mt-0.5" />
              <p className="text-red-500 text-sm">{error}</p>
            </div>
          )}

          <div className="space-y-3">
            {/* NeoLine Button */}
            <button
              onClick={() => handleConnect('neoline')}
              disabled={loading || !walletAvailable.neoline}
              className={`w-full flex items-center justify-between gap-3 ${
                walletAvailable.neoline
                  ? 'bg-green-600 hover:bg-green-700'
                  : 'bg-gray-700 cursor-not-allowed'
              } disabled:opacity-50 text-white font-medium py-3 px-4 rounded-lg transition-colors`}
            >
              <div className="flex items-center gap-3">
                <Wallet className="w-5 h-5" />
                <span>NeoLine Wallet</span>
              </div>
              {!walletAvailable.neoline && (
                <a
                  href="https://neoline.io/"
                  target="_blank"
                  rel="noopener noreferrer"
                  onClick={(e) => e.stopPropagation()}
                  className="text-xs text-gray-400 hover:text-white flex items-center gap-1"
                >
                  Install <ExternalLink className="w-3 h-3" />
                </a>
              )}
              {walletAvailable.neoline && loading && (
                <span className="text-sm">Connecting...</span>
              )}
            </button>

            {/* O3 Button */}
            <button
              onClick={() => handleConnect('o3')}
              disabled={loading || !walletAvailable.o3}
              className={`w-full flex items-center justify-between gap-3 ${
                walletAvailable.o3
                  ? 'bg-blue-600 hover:bg-blue-700'
                  : 'bg-gray-700 cursor-not-allowed'
              } disabled:opacity-50 text-white font-medium py-3 px-4 rounded-lg transition-colors`}
            >
              <div className="flex items-center gap-3">
                <Wallet className="w-5 h-5" />
                <span>O3 Wallet</span>
              </div>
              {!walletAvailable.o3 && (
                <a
                  href="https://o3.network/"
                  target="_blank"
                  rel="noopener noreferrer"
                  onClick={(e) => e.stopPropagation()}
                  className="text-xs text-gray-400 hover:text-white flex items-center gap-1"
                >
                  Install <ExternalLink className="w-3 h-3" />
                </a>
              )}
              {walletAvailable.o3 && loading && (
                <span className="text-sm">Connecting...</span>
              )}
            </button>

            {/* OAuth Divider */}
            <div className="relative my-4">
              <div className="absolute inset-0 flex items-center">
                <div className="w-full border-t border-gray-700"></div>
              </div>
              <div className="relative flex justify-center text-sm">
                <span className="px-2 bg-gray-800 text-gray-500">or continue with</span>
              </div>
            </div>

            {/* OAuth Buttons */}
            <div className="grid grid-cols-2 gap-3">
              {/* Google Button */}
              <button
                onClick={() => handleOAuthLogin('google')}
                disabled={loading}
                className="flex items-center justify-center gap-2 bg-white hover:bg-gray-100 disabled:opacity-50 text-gray-800 font-medium py-3 px-4 rounded-lg transition-colors"
              >
                <svg className="w-5 h-5" viewBox="0 0 24 24">
                  <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
                  <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
                  <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/>
                  <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
                </svg>
                <span>Google</span>
              </button>

              {/* GitHub Button */}
              <button
                onClick={() => handleOAuthLogin('github')}
                disabled={loading}
                className="flex items-center justify-center gap-2 bg-gray-900 border border-gray-600 hover:bg-gray-800 disabled:opacity-50 text-white font-medium py-3 px-4 rounded-lg transition-colors"
              >
                <svg className="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
                </svg>
                <span>GitHub</span>
              </button>
            </div>
          </div>

          <div className="mt-6 text-center">
            <p className="text-gray-500 text-sm">
              Sign a message to authenticate securely
            </p>
          </div>
        </div>

        <div className="mt-8 text-center space-y-2">
          <p className="text-gray-500 text-sm">
            Protected by MarbleRun + EGo SGX Enclaves
          </p>
          <p className="text-gray-600 text-xs">
            Your private keys never leave your wallet
          </p>
        </div>
      </div>
    </div>
  );
}
