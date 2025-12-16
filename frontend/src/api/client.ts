export const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1';

interface RequestOptions {
  method?: string;
  body?: unknown;
  headers?: Record<string, string>;
  credentials?: RequestCredentials;
}

// =============================================================================
// Types
// =============================================================================

export interface User {
  id: string;
  address: string;
  email?: string;
  created_at: string;
  updated_at: string;
}

export interface Wallet {
  id: string;
  user_id: string;
  address: string;
  label?: string;
  is_primary: boolean;
  verified: boolean;
  created_at: string;
}

export interface APIKey {
  id: string;
  name: string;
  prefix: string;
  scopes: string[];
  created_at: string;
  last_used?: string;
}

export interface APIKeyCreateResponse extends APIKey {
  key: string; // Only returned once on creation
}

export interface GasBankAccount {
  id: string;
  user_id: string;
  balance: number;
  reserved: number;
  created_at: string;
  updated_at: string;
}

export interface DepositRequest {
  id: string;
  user_id: string;
  account_id: string;
  amount: number;
  tx_hash?: string;
  from_address: string;
  status: 'pending' | 'confirming' | 'confirmed' | 'failed' | 'expired';
  confirmations: number;
  required_confirmations: number;
  created_at: string;
  confirmed_at?: string;
  expires_at: string;
}

export interface GasBankTransaction {
  id: string;
  account_id: string;
  tx_type: string;
  amount: number;
  balance_after: number;
  reference_id?: string;
  tx_hash?: string;
  status: string;
  created_at: string;
}

export interface AuthResponse {
  user_id: string;
  address: string;
  token: string;
}

export interface NonceResponse {
  nonce: string;
  message: string;
}

export interface WalletAuthPayload {
  address: string;
  publicKey: string;
  signature: string;
  message: string;
  nonce: string;
}

export interface MeResponse {
  user: User;
  wallets: Wallet[];
  gasbank: GasBankAccount;
}

export interface OAuthProvider {
  id: string;
  provider: 'google' | 'github';
  email?: string;
  display_name?: string;
  avatar_url?: string;
  created_at: string;
}

// =============================================================================
// API Client
// =============================================================================

class ApiClient {
  private token: string | null = null;
  private apiKey: string | null = null;

  setToken(token: string | null) {
    this.token = token;
  }

  setApiKey(apiKey: string | null) {
    this.apiKey = apiKey;
  }

  async request<T>(endpoint: string, options: RequestOptions = {}): Promise<T> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...options.headers,
    };

    if (this.apiKey) {
      headers['X-API-Key'] = this.apiKey;
    } else if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`;
    }

    const response = await fetch(`${API_BASE}${endpoint}`, {
      method: options.method || 'GET',
      credentials: options.credentials ?? 'include',
      headers,
      body: options.body ? JSON.stringify(options.body) : undefined,
    });

    if (!response.ok) {
      const errorPayload = await response
        .json()
        .catch(() => ({ message: `HTTP ${response.status}` }) as { error?: string; message?: string });
      const message = errorPayload?.error || errorPayload?.message || `HTTP ${response.status}`;
      throw new Error(message);
    }

    // Handle 204 No Content
    if (response.status === 204) {
      return {} as T;
    }

    return response.json();
  }

  // =============================================================================
  // Auth
  // =============================================================================

  async getNonce(address: string): Promise<NonceResponse> {
    return this.request<NonceResponse>('/auth/nonce', {
      method: 'POST',
      body: { address },
    });
  }

  async register(payload: WalletAuthPayload): Promise<AuthResponse> {
    return this.request<AuthResponse>('/auth/register', {
      method: 'POST',
      body: payload,
    });
  }

  async login(payload: WalletAuthPayload): Promise<AuthResponse> {
    return this.request<AuthResponse>('/auth/login', {
      method: 'POST',
      body: payload,
    });
  }

  async logout(): Promise<void> {
    await this.request('/auth/logout', { method: 'POST' });
    this.token = null;
  }

  async getMe(): Promise<MeResponse> {
    return this.request<MeResponse>('/me');
  }

  // =============================================================================
  // API Keys
  // =============================================================================

  async listAPIKeys(): Promise<APIKey[]> {
    return this.request<APIKey[]>('/apikeys');
  }

  async createAPIKey(name: string, scopes: string[] = []): Promise<APIKeyCreateResponse> {
    return this.request<APIKeyCreateResponse>('/apikeys', {
      method: 'POST',
      body: { name, scopes },
    });
  }

  async revokeAPIKey(keyId: string): Promise<void> {
    await this.request(`/apikeys/${keyId}`, { method: 'DELETE' });
  }

  // =============================================================================
  // Wallets
  // =============================================================================

  async listWallets(): Promise<Wallet[]> {
    return this.request<Wallet[]>('/wallets');
  }

  async addWallet(address: string, label: string, signature: string, message: string, publicKey: string): Promise<Wallet> {
    return this.request<Wallet>('/wallets', {
      method: 'POST',
      body: { address, label, signature, message, publicKey },
    });
  }

  async setPrimaryWallet(walletId: string): Promise<void> {
    await this.request(`/wallets/${walletId}/primary`, { method: 'POST' });
  }

  async verifyWallet(walletId: string, signature: string, message: string, publicKey: string): Promise<void> {
    await this.request(`/wallets/${walletId}/verify`, {
      method: 'POST',
      body: { signature, message, publicKey },
    });
  }

  async deleteWallet(walletId: string): Promise<void> {
    await this.request(`/wallets/${walletId}`, { method: 'DELETE' });
  }

  // =============================================================================
  // OAuth Providers
  // =============================================================================

  async listOAuthProviders(): Promise<OAuthProvider[]> {
    return this.request<OAuthProvider[]>('/oauth/providers');
  }

  async unlinkOAuthProvider(providerId: string): Promise<void> {
    await this.request(`/oauth/providers/${providerId}`, { method: 'DELETE' });
  }

  // =============================================================================
  // Gas Bank
  // =============================================================================

  async getGasBankAccount(): Promise<GasBankAccount> {
    return this.request<GasBankAccount>('/gasbank/account');
  }

  async createDeposit(amount: number, fromAddress: string, txHash?: string): Promise<DepositRequest> {
    return this.request<DepositRequest>('/gasbank/deposit', {
      method: 'POST',
      body: { amount, from_address: fromAddress, tx_hash: txHash },
    });
  }

  async listDeposits(): Promise<DepositRequest[]> {
    return this.request<DepositRequest[]>('/gasbank/deposits');
  }

  async listTransactions(): Promise<GasBankTransaction[]> {
    return this.request<GasBankTransaction[]>('/gasbank/transactions');
  }

  // Legacy method for compatibility
  async getBalance(): Promise<{ balance: number; reserved: number; available: number }> {
    const account = await this.getGasBankAccount();
    return {
      balance: account.balance,
      reserved: account.reserved,
      available: account.balance - account.reserved,
    };
  }

  async deposit(amount: number, txHash: string): Promise<DepositRequest> {
    return this.createDeposit(amount, '', txHash);
  }

  // =============================================================================
  // Services
  // =============================================================================

  async getHealth() {
    return this.request<{ status: string; enclave: boolean }>('/health');
  }

  // Oracle
  async neooracleFetch(url: string, jsonPath?: string) {
    return this.request('/neooracle/fetch', {
      method: 'POST',
      body: { url, json_path: jsonPath },
    });
  }

  // Secrets
  async listSecrets() {
    return this.request<Array<{ id: string; name: string; version: number; created_at: string; updated_at: string }>>('/secrets');
  }

  async createSecret(name: string, value: string) {
    return this.request('/secrets', {
      method: 'POST',
      body: { name, value },
    });
  }

  async deleteSecret(name: string) {
    return this.request(`/secrets/${name}`, {
      method: 'DELETE',
    });
  }

  async getSecretPermissions(name: string): Promise<string[]> {
    const res = await this.request<{ services: string[] }>(`/secrets/${name}/permissions`);
    return res.services ?? [];
  }

  async setSecretPermissions(name: string, services: string[]): Promise<string[]> {
    const res = await this.request<{ services: string[] }>(`/secrets/${name}/permissions`, {
      method: 'PUT',
      body: { services },
    });
    return res.services ?? [];
  }

  async grantSecretPermission(name: string, serviceName: string): Promise<string[]> {
    const normalized = serviceName.trim();
    if (!normalized) return this.getSecretPermissions(name);

    const current = await this.getSecretPermissions(name);
    const alreadyIncluded = current.some((s) => s.toLowerCase() === normalized.toLowerCase());
    if (alreadyIncluded) return current;

    return this.setSecretPermissions(name, [...current, normalized]);
  }

  async revokeSecretPermission(name: string, serviceName: string): Promise<string[]> {
    const normalized = serviceName.trim();
    if (!normalized) return this.getSecretPermissions(name);

    const current = await this.getSecretPermissions(name);
    const next = current.filter((s) => s.toLowerCase() !== normalized.toLowerCase());

    return this.setSecretPermissions(name, next);
  }

  async getSecretAuditLog(name: string) {
    return this.request<Array<{
      id: string;
      user_id: string;
      secret_name: string;
      action: string;
      service_id?: string;
      ip_address?: string;
      user_agent?: string;
      success: boolean;
      error_message?: string;
      created_at: string;
    }>>(`/secrets/${name}/audit`);
  }

  // NeoFlow
  async listTriggers() {
    return this.request<Array<{ id: string; name: string; enabled: boolean }>>('/neoflow/triggers');
  }

  async createTrigger(trigger: { name: string; trigger_type: string; schedule?: string; action: unknown }) {
    return this.request('/neoflow/triggers', {
      method: 'POST',
      body: trigger,
    });
  }

  // NeoFeeds
  async getPrice(pair: string) {
    return this.request<{ price: number; decimals: number; timestamp: string }>(`/neofeeds/price/${pair}`);
  }
}

export const api = new ApiClient();
