import React, { useState } from 'react';
import { useForm } from 'react-hook-form';
import { Link, useNavigate } from 'react-router-dom';
import { FiMail, FiLock, FiEye, FiEyeOff, FiGithub } from 'react-icons/fi';
import { FcGoogle } from 'react-icons/fc';
import { FaDiscord } from 'react-icons/fa';
import { useAuthStore } from '@/stores/authStore';
import { Button, Input } from '@/components/common';
import { OAUTH_PROVIDERS, type OAuthProvider } from '@/lib/supabase';
import toast from 'react-hot-toast';

interface LoginFormData {
  email: string;
  password: string;
}

export function LoginForm() {
  const navigate = useNavigate();
  const { loginWithEmail, loginWithOAuth, isLoading } = useAuthStore();
  const [showPassword, setShowPassword] = useState(false);
  const [oauthLoading, setOauthLoading] = useState<OAuthProvider | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<LoginFormData>();

  const onSubmit = async (data: LoginFormData) => {
    const success = await loginWithEmail(data.email, data.password);
    if (success) {
      toast.success('Welcome back!');
      navigate('/dashboard');
    } else {
      toast.error('Invalid email or password');
    }
  };

  const handleOAuthLogin = async (provider: OAuthProvider) => {
    setOauthLoading(provider);
    try {
      await loginWithOAuth(provider);
      // OAuth will redirect, so no need to handle success here
    } catch (error) {
      toast.error(`Failed to sign in with ${provider}`);
      setOauthLoading(null);
    }
  };

  const getProviderIcon = (provider: OAuthProvider) => {
    switch (provider) {
      case 'github':
        return <FiGithub className="w-5 h-5" />;
      case 'google':
        return <FcGoogle className="w-5 h-5" />;
      case 'discord':
        return <FaDiscord className="w-5 h-5 text-[#5865f2]" />;
      default:
        return null;
    }
  };

  return (
    <div className="space-y-6">
      {/* OAuth Providers */}
      <div className="space-y-3">
        {OAUTH_PROVIDERS.slice(0, 3).map((provider) => (
          <button
            key={provider.id}
            onClick={() => handleOAuthLogin(provider.id)}
            disabled={isLoading || oauthLoading !== null}
            className="w-full flex items-center justify-center gap-3 px-4 py-3 border border-surface-300 rounded-lg hover:bg-surface-50 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {oauthLoading === provider.id ? (
              <svg className="animate-spin h-5 w-5 text-surface-500" fill="none" viewBox="0 0 24 24">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
              </svg>
            ) : (
              getProviderIcon(provider.id)
            )}
            <span className="font-medium text-surface-700">Continue with {provider.name}</span>
          </button>
        ))}
      </div>

      <div className="relative">
        <div className="absolute inset-0 flex items-center">
          <div className="w-full border-t border-surface-200" />
        </div>
        <div className="relative flex justify-center text-sm">
          <span className="px-2 bg-white text-surface-500">Or continue with email</span>
        </div>
      </div>

      {/* Email/Password Form */}
      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        <Input
          label="Email"
          type="email"
          placeholder="you@example.com"
          leftIcon={<FiMail className="w-5 h-5" />}
          error={errors.email?.message}
          {...register('email', {
            required: 'Email is required',
            pattern: {
              value: /^[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}$/i,
              message: 'Invalid email address',
            },
          })}
        />

        <Input
          label="Password"
          type={showPassword ? 'text' : 'password'}
          placeholder="Enter your password"
          leftIcon={<FiLock className="w-5 h-5" />}
          rightIcon={
            <button
              type="button"
              onClick={() => setShowPassword(!showPassword)}
              className="focus:outline-none"
            >
              {showPassword ? <FiEyeOff className="w-5 h-5" /> : <FiEye className="w-5 h-5" />}
            </button>
          }
          error={errors.password?.message}
          {...register('password', {
            required: 'Password is required',
            minLength: {
              value: 6,
              message: 'Password must be at least 6 characters',
            },
          })}
        />

        <div className="flex items-center justify-between">
          <label className="flex items-center gap-2">
            <input type="checkbox" className="rounded border-surface-300 text-primary-600 focus:ring-primary-500" />
            <span className="text-sm text-surface-600">Remember me</span>
          </label>
          <Link to="/forgot-password" className="text-sm text-primary-600 hover:text-primary-700">
            Forgot password?
          </Link>
        </div>

        <Button type="submit" className="w-full" size="lg" isLoading={isLoading}>
          Sign In
        </Button>
      </form>

      <p className="text-center text-sm text-surface-600">
        Don't have an account?{' '}
        <Link to="/register" className="text-primary-600 hover:text-primary-700 font-medium">
          Sign up
        </Link>
      </p>
    </div>
  );
}
