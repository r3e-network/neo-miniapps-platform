const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1';

interface RequestOptions {
  method?: string;
  body?: unknown;
  headers?: Record<string, string>;
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
      headers,
      body: options.body ? JSON.stringify(options.body) : undefined,
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: `HTTP ${response.status}` }));
      throw new Error(error.error || `HTTP ${response.status}`);
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

  async register(address: string, signature: string, message: string): Promise<AuthResponse> {
    return this.request<AuthResponse>('/auth/register', {
      method: 'POST',
      body: { address, signature, message },
    });
  }

  async login(address: string, signature: string, message: string): Promise<AuthResponse> {
    return this.request<AuthResponse>('/auth/login', {
      method: 'POST',
      body: { address, signature, message },
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

  async addWallet(address: string, label: string, signature: string, message: string): Promise<Wallet> {
    return this.request<Wallet>('/wallets', {
      method: 'POST',
      body: { address, label, signature, message },
    });
  }

  async setPrimaryWallet(walletId: string): Promise<void> {
    await this.request(`/wallets/${walletId}/primary`, { method: 'POST' });
  }

  async verifyWallet(walletId: string, signature: string): Promise<void> {
    await this.request(`/wallets/${walletId}/verify`, {
      method: 'POST',
      body: { signature },
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
  async oracleFetch(url: string, jsonPath?: string) {
    return this.request('/oracle/fetch', {
      method: 'POST',
      body: { url, json_path: jsonPath },
    });
  }

  // VRF
  async vrfRandom(seed: string, numWords = 1) {
    return this.request('/vrf/random', {
      method: 'POST',
      body: { seed, num_words: numWords },
    });
  }

  // Secrets
  async listSecrets() {
    return this.request<Array<{ id: string; name: string; version: number }>>('/secrets/secrets');
  }

  async createSecret(name: string, value: string) {
    return this.request('/secrets/secrets', {
      method: 'POST',
      body: { name, value },
    });
  }

  // Automation
  async listTriggers() {
    return this.request<Array<{ id: string; name: string; enabled: boolean }>>('/automation/triggers');
  }

  async createTrigger(trigger: { name: string; trigger_type: string; schedule?: string; action: unknown }) {
    return this.request('/automation/triggers', {
      method: 'POST',
      body: trigger,
    });
  }

  // DataFeeds
  async getPrice(pair: string) {
    return this.request<{ price: number; decimals: number; timestamp: string }>(`/datafeeds/price/${pair}`);
  }

  // Mixer
  async getMixerInfo() {
    return this.request<{
      status: string;
      bond_amount: string;
      available_capacity: string;
      total_mixed: string;
    }>('/mixer/info');
  }

  async getMixerRequests() {
    return this.request<Array<{
      request_id: string;
      amount: string;
      status: number;
      mix_option: number;
      created_at: string;
      deadline: string;
      can_refund: boolean;
    }>>('/mixer/requests');
  }

  async createMixRequest(targets: Array<{ address: string; amount: string }>, mixOption: number) {
    return this.request<{ request_id: string; tx_hash: string }>('/mixer/request', {
      method: 'POST',
      body: {
        targets,
        mix_option: mixOption,
      },
    });
  }

  async claimMixerRefund(requestId: string) {
    return this.request<{ tx_hash: string }>(`/mixer/refund/${requestId}`, {
      method: 'POST',
    });
  }

  async getMixerRequest(requestId: string) {
    return this.request<{
      request_id: string;
      amount: string;
      status: number;
      mix_option: number;
      created_at: string;
      deadline: string;
      can_refund: boolean;
      outputs_hash?: string;
    }>(`/mixer/request/${requestId}`);
  }
}

export const api = new ApiClient();
