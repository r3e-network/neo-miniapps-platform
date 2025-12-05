import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { User as SupabaseUser, Session } from '@supabase/supabase-js';
import {
  supabase,
  signInWithOAuth,
  signInWithEmail,
  signUpWithEmail,
  signOut as supabaseSignOut,
  getSession,
  type OAuthProvider,
} from '@/lib/supabase';
import type { Account } from '@/types';
import apiClient from '@/api/client';

interface WalletState {
  connected: boolean;
  address: string | null;
  balance: string;
}

interface AuthState {
  user: SupabaseUser | null;
  session: Session | null;
  account: Account | null;
  wallet: WalletState;
  isLoading: boolean;
  isAuthenticated: boolean;
  isInitialized: boolean;

  // Actions
  initialize: () => Promise<void>;
  loginWithEmail: (email: string, password: string) => Promise<boolean>;
  loginWithOAuth: (provider: OAuthProvider) => Promise<void>;
  register: (email: string, password: string, name: string) => Promise<boolean>;
  logout: () => Promise<void>;
  refreshSession: () => Promise<void>;
  connectWallet: (address: string, balance: string) => void;
  disconnectWallet: () => void;
  linkWallet: (address: string, signature: string) => Promise<boolean>;
  updateAccount: (updates: Partial<Account>) => Promise<boolean>;
  fetchAccount: () => Promise<void>;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      user: null,
      session: null,
      account: null,
      wallet: {
        connected: false,
        address: null,
        balance: '0',
      },
      isLoading: false,
      isAuthenticated: false,
      isInitialized: false,

      initialize: async () => {
        try {
          // Get current session
          const session = await getSession();

          if (session) {
            // Set API client token
            apiClient.setToken(session.access_token);

            set({
              user: session.user,
              session,
              isAuthenticated: true,
              isInitialized: true,
            });

            // Fetch account data
            await get().fetchAccount();
          } else {
            set({ isInitialized: true });
          }

          // Listen for auth changes
          supabase.auth.onAuthStateChange(async (event, session) => {
            if (event === 'SIGNED_IN' && session) {
              apiClient.setToken(session.access_token);
              set({
                user: session.user,
                session,
                isAuthenticated: true,
              });
              await get().fetchAccount();
            } else if (event === 'SIGNED_OUT') {
              apiClient.setToken(null);
              set({
                user: null,
                session: null,
                account: null,
                isAuthenticated: false,
              });
            } else if (event === 'TOKEN_REFRESHED' && session) {
              apiClient.setToken(session.access_token);
              set({ session });
            }
          });
        } catch (error) {
          console.error('Auth initialization failed:', error);
          set({ isInitialized: true });
        }
      },

      loginWithEmail: async (email: string, password: string) => {
        set({ isLoading: true });
        try {
          const { session, user } = await signInWithEmail(email, password);

          if (session && user) {
            apiClient.setToken(session.access_token);
            set({
              user,
              session,
              isAuthenticated: true,
              isLoading: false,
            });
            await get().fetchAccount();
            return true;
          }
          set({ isLoading: false });
          return false;
        } catch (error) {
          console.error('Login failed:', error);
          set({ isLoading: false });
          return false;
        }
      },

      loginWithOAuth: async (provider: OAuthProvider) => {
        set({ isLoading: true });
        try {
          await signInWithOAuth(provider);
          // OAuth redirects, so we don't need to handle the response here
        } catch (error) {
          console.error('OAuth login failed:', error);
          set({ isLoading: false });
          throw error;
        }
      },

      register: async (email: string, password: string, name: string) => {
        set({ isLoading: true });
        try {
          const { session, user } = await signUpWithEmail(email, password, { name });

          if (session && user) {
            apiClient.setToken(session.access_token);
            set({
              user,
              session,
              isAuthenticated: true,
              isLoading: false,
            });
            await get().fetchAccount();
            return true;
          }

          // Email confirmation required
          set({ isLoading: false });
          return true; // Return true to show confirmation message
        } catch (error) {
          console.error('Registration failed:', error);
          set({ isLoading: false });
          return false;
        }
      },

      logout: async () => {
        try {
          await supabaseSignOut();
          apiClient.setToken(null);
          set({
            user: null,
            session: null,
            account: null,
            isAuthenticated: false,
            wallet: {
              connected: false,
              address: null,
              balance: '0',
            },
          });
        } catch (error) {
          console.error('Logout failed:', error);
        }
      },

      refreshSession: async () => {
        try {
          const session = await getSession();
          if (session) {
            apiClient.setToken(session.access_token);
            set({
              user: session.user,
              session,
              isAuthenticated: true,
            });
          } else {
            await get().logout();
          }
        } catch (error) {
          console.error('Session refresh failed:', error);
          await get().logout();
        }
      },

      fetchAccount: async () => {
        try {
          const response = await apiClient.get<Account>('/account');
          if (response.success && response.data) {
            set({ account: response.data });
          }
        } catch (error) {
          console.error('Failed to fetch account:', error);
        }
      },

      connectWallet: (address: string, balance: string) => {
        set({
          wallet: {
            connected: true,
            address,
            balance,
          },
        });
      },

      disconnectWallet: () => {
        set({
          wallet: {
            connected: false,
            address: null,
            balance: '0',
          },
        });
      },

      linkWallet: async (address: string, signature: string) => {
        try {
          const response = await apiClient.post('/account/link-wallet', {
            address,
            signature,
          });

          if (response.success) {
            await get().fetchAccount();
            return true;
          }
          return false;
        } catch (error) {
          console.error('Failed to link wallet:', error);
          return false;
        }
      },

      updateAccount: async (updates: Partial<Account>) => {
        try {
          const response = await apiClient.patch<Account>('/account', updates);
          if (response.success && response.data) {
            set({ account: response.data });
            return true;
          }
          return false;
        } catch (error) {
          console.error('Failed to update account:', error);
          return false;
        }
      },
    }),
    {
      name: 'auth-storage',
      partialize: (state) => ({
        // Only persist minimal data, session is managed by Supabase
        wallet: state.wallet,
      }),
    }
  )
);
