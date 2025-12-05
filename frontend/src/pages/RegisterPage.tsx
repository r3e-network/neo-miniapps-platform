import React from 'react';
import { Navigate } from 'react-router-dom';
import { RegisterForm } from '@/components/auth/RegisterForm';
import { useAuthStore } from '@/stores/authStore';

export function RegisterPage() {
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
          <p className="text-primary-200 mt-1">Create your account</p>
        </div>

        {/* Register Card */}
        <div className="bg-white rounded-2xl shadow-xl p-8">
          <h2 className="text-xl font-semibold text-surface-900 mb-6">Get started for free</h2>
          <RegisterForm />
        </div>

        {/* Features */}
        <div className="grid grid-cols-3 gap-4 mt-8">
          <div className="text-center">
            <div className="text-2xl mb-1">🔒</div>
            <p className="text-xs text-primary-200">TEE Protected</p>
          </div>
          <div className="text-center">
            <div className="text-2xl mb-1">⚡</div>
            <p className="text-xs text-primary-200">Fast & Reliable</p>
          </div>
          <div className="text-center">
            <div className="text-2xl mb-1">🌐</div>
            <p className="text-xs text-primary-200">Multi-Chain</p>
          </div>
        </div>
      </div>
    </div>
  );
}
