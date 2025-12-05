import React, { useState } from 'react';
import { useForm } from 'react-hook-form';
import { Link, useNavigate } from 'react-router-dom';
import { FiMail, FiLock, FiUser, FiEye, FiEyeOff, FiGithub } from 'react-icons/fi';
import { FcGoogle } from 'react-icons/fc';
import { FaDiscord } from 'react-icons/fa';
import { useAuthStore } from '@/stores/authStore';
import { Button, Input } from '@/components/common';
import { OAUTH_PROVIDERS, type OAuthProvider } from '@/lib/supabase';
import toast from 'react-hot-toast';

interface RegisterFormData {
  name: string;
  email: string;
  password: string;
  confirmPassword: string;
}

export function RegisterForm() {
  const navigate = useNavigate();
  const { register: registerUser, loginWithOAuth, isLoading } = useAuthStore();
  const [showPassword, setShowPassword] = useState(false);
  const [oauthLoading, setOauthLoading] = useState<OAuthProvider | null>(null);
  const [emailSent, setEmailSent] = useState(false);

  const {
    register,
    handleSubmit,
    watch,
    formState: { errors },
  } = useForm<RegisterFormData>();

  const password = watch('password');

  const onSubmit = async (data: RegisterFormData) => {
    const success = await registerUser(data.email, data.password, data.name);
    if (success) {
      // Check if email confirmation is required
      setEmailSent(true);
      toast.success('Check your email to confirm your account!');
    } else {
      toast.error('Failed to create account. Please try again.');
    }
  };

  const handleOAuthLogin = async (provider: OAuthProvider) => {
    setOauthLoading(provider);
    try {
      await loginWithOAuth(provider);
    } catch (error) {
      toast.error(`Failed to sign up with ${provider}`);
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

  if (emailSent) {
    return (
      <div className="text-center space-y-4">
        <div className="w-16 h-16 mx-auto bg-green-100 rounded-full flex items-center justify-center">
          <FiMail className="w-8 h-8 text-green-600" />
        </div>
        <h3 className="text-lg font-semibold text-surface-900">Check your email</h3>
        <p className="text-surface-600">
          We've sent you a confirmation link. Please check your email to activate your account.
        </p>
        <Button variant="secondary" onClick={() => setEmailSent(false)}>
          Back to registration
        </Button>
      </div>
    );
  }

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
          <span className="px-2 bg-white text-surface-500">Or register with email</span>
        </div>
      </div>

      {/* Email/Password Form */}
      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        <Input
          label="Full Name"
          type="text"
          placeholder="John Doe"
          leftIcon={<FiUser className="w-5 h-5" />}
          error={errors.name?.message}
          {...register('name', {
            required: 'Name is required',
            minLength: {
              value: 2,
              message: 'Name must be at least 2 characters',
            },
          })}
        />

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
          placeholder="Create a strong password"
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
          helperText="At least 8 characters"
          {...register('password', {
            required: 'Password is required',
            minLength: {
              value: 8,
              message: 'Password must be at least 8 characters',
            },
          })}
        />

        <Input
          label="Confirm Password"
          type={showPassword ? 'text' : 'password'}
          placeholder="Confirm your password"
          leftIcon={<FiLock className="w-5 h-5" />}
          error={errors.confirmPassword?.message}
          {...register('confirmPassword', {
            required: 'Please confirm your password',
            validate: (value) => value === password || 'Passwords do not match',
          })}
        />

        <div className="flex items-start gap-2">
          <input
            type="checkbox"
            id="terms"
            className="mt-1 rounded border-surface-300 text-primary-600 focus:ring-primary-500"
            required
          />
          <label htmlFor="terms" className="text-sm text-surface-600">
            I agree to the{' '}
            <Link to="/terms" className="text-primary-600 hover:text-primary-700">
              Terms of Service
            </Link>{' '}
            and{' '}
            <Link to="/privacy" className="text-primary-600 hover:text-primary-700">
              Privacy Policy
            </Link>
          </label>
        </div>

        <Button type="submit" className="w-full" size="lg" isLoading={isLoading}>
          Create Account
        </Button>
      </form>

      <p className="text-center text-sm text-surface-600">
        Already have an account?{' '}
        <Link to="/login" className="text-primary-600 hover:text-primary-700 font-medium">
          Sign in
        </Link>
      </p>
    </div>
  );
}
