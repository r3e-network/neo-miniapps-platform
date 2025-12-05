import React from 'react';
import { Navigate } from 'react-router-dom';
import { LoginForm } from '@/components/auth/LoginForm';
import { WalletConnect } from '@/components/auth/WalletConnect';
import { useAuthStore } from '@/stores/authStore';

export function LoginPage() {
  const { isAuthenticated } = useAuthStore();

  if (isAuthenticated) {
    return <Navigate to="/dashboard" replace />;
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-primary-900 via-primary-800 to-surface-900 flex items-center justify-center p-4">
      <div className="w-full max-w-md">
        {/* Logo */}
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-16 h-16 bg-white rounded-2xl shadow-lg mb-4">
            <span className="text-2xl font-bold bg-gradient-to-br from-primary-600 to-primary-800 bg-clip-text text-transparent">
              SL
            </span>
          </div>
          <h1 className="text-2xl font-bold text-white">Service Layer</h1>
          <p className="text-primary-200 mt-1">TEE-Powered Blockchain Services</p>
        </div>

        {/* Login Card */}
        <div className="bg-white rounded-2xl shadow-xl p-8">
          <h2 className="text-xl font-semibold text-surface-900 mb-6">Sign in to your account</h2>

          <LoginForm />

          <div className="relative my-6">
            <div className="absolute inset-0 flex items-center">
              <div className="w-full border-t border-surface-200" />
            </div>
            <div className="relative flex justify-center text-sm">
              <span className="px-2 bg-white text-surface-500">Or continue with</span>
            </div>
          </div>

          <WalletConnect />
        </div>

        {/* Footer */}
        <p className="text-center text-primary-200 text-sm mt-6">
          Secured by TEE technology. Your data is protected.
        </p>
      </div>
    </div>
  );
}
