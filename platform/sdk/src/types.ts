export type ContractParam =
  | { type: "String"; value: string }
  | { type: "Integer"; value: string }
  | { type: "Boolean"; value: boolean }
  | { type: "ByteArray"; value: string }
  | { type: "Hash160"; value: string }
  | { type: "Hash256"; value: string }
  | { type: "PublicKey"; value: string }
  | { type: "Any"; value: null }
  | { type: "Array"; value: ContractParam[] };

export type InvocationIntent = {
  contract_hash: string;
  method: string;
  params: ContractParam[];
};

export type PayGASResponse = {
  request_id: string;
  user_id: string;
  intent: "payments";
  constraints: { settlement: "GAS_ONLY" };
  invocation: InvocationIntent;
};

export type VoteNEOResponse = {
  request_id: string;
  user_id: string;
  intent: "governance";
  constraints: { governance: "NEO_ONLY" };
  invocation: InvocationIntent;
};

export type RNGResponse = {
  request_id: string;
  app_id: string;
  randomness: string;
  report_hash?: string;
  anchored_tx?: unknown;
};

export type AppRegisterResponse = {
  request_id: string;
  user_id: string;
  intent: "apps";
  manifest_hash?: string;
  invocation: InvocationIntent;
};

export type AppUpdateManifestResponse = {
  request_id: string;
  user_id: string;
  intent: "apps";
  manifest_hash?: string;
  invocation: InvocationIntent;
};

export type WalletNonceResponse = {
  nonce: string;
  message: string;
};

export type WalletBindResponse = {
  wallet: {
    id: string;
    address: string;
    label?: string | null;
    is_primary: boolean;
    verified: boolean;
    created_at: string;
  };
};

export type SecretMeta = {
  id: string;
  name: string;
  version: number;
  created_at: string;
  updated_at: string;
};

export type SecretsListResponse = { secrets: SecretMeta[] };
export type SecretsGetResponse = { name: string; value: string; version: number };
export type SecretsUpsertResponse = { secret: SecretMeta; created: boolean };
export type SecretsDeleteResponse = { status: "ok" };
export type SecretsPermissionsResponse = { status: "ok"; services: string[] };

export type APIKeyMeta = {
  id: string;
  name: string;
  prefix: string;
  scopes: string[];
  description?: string | null;
  created_at: string;
  last_used?: string | null;
  expires_at?: string | null;
  revoked?: boolean | null;
};

export type APIKeysListResponse = { api_keys: APIKeyMeta[] };
export type APIKeyCreateResponse = { api_key: APIKeyMeta & { key: string } };
export type APIKeyRevokeResponse = { status: "ok" };

export type GasBankAccount = {
  id: string;
  user_id: string;
  balance: number | string;
  reserved: number | string;
  created_at: string;
  updated_at: string;
};

export type GasBankAccountResponse = { account: GasBankAccount };
export type GasBankDepositsResponse = { deposits: unknown[] };
export type GasBankTransactionsResponse = { transactions: unknown[] };
export type GasBankDepositCreateResponse = { deposit: unknown };

export type PriceResponse = {
  feed_id: string;
  pair: string;
  price: number | string;
  decimals: number;
  timestamp: string;
  sources: string[];
  signature?: string;
  public_key?: string;
};

export interface MiniAppSDK {
  wallet: {
    getAddress(): Promise<string>;
  };
  payments: {
    payGAS(appId: string, amount: string, memo?: string): Promise<PayGASResponse>;
  };
  governance: {
    vote(appId: string, proposalId: string, neoAmount: string, support?: boolean): Promise<VoteNEOResponse>;
  };
  rng: {
    requestRandom(appId: string): Promise<RNGResponse>;
  };
  datafeed: {
    getPrice(symbol: string): Promise<PriceResponse>;
  };
}

// Host-only APIs (should not be exposed to untrusted MiniApps).
export interface HostSDK {
  wallet: MiniAppSDK["wallet"] & {
    getBindMessage(): Promise<WalletNonceResponse>;
    bindWallet(params: {
      address: string;
      publicKey: string;
      signature: string;
      message: string;
      nonce: string;
      label?: string;
    }): Promise<WalletBindResponse>;
  };
  apps: {
    register(params: { manifest: Record<string, unknown> }): Promise<AppRegisterResponse>;
    updateManifest(params: { manifest: Record<string, unknown> }): Promise<AppUpdateManifestResponse>;
  };
  secrets: {
    list(): Promise<SecretsListResponse>;
    get(name: string): Promise<SecretsGetResponse>;
    upsert(name: string, value: string): Promise<SecretsUpsertResponse>;
    delete(name: string): Promise<SecretsDeleteResponse>;
    setPermissions(name: string, services: string[]): Promise<SecretsPermissionsResponse>;
  };
  apiKeys: {
    list(): Promise<APIKeysListResponse>;
    create(params: { name: string; scopes?: string[]; description?: string; expires_at?: string }): Promise<APIKeyCreateResponse>;
    revoke(id: string): Promise<APIKeyRevokeResponse>;
  };
  gasbank: {
    getAccount(): Promise<GasBankAccountResponse>;
    listDeposits(): Promise<GasBankDepositsResponse>;
    createDeposit(params: { amount: string; from_address: string; tx_hash?: string }): Promise<GasBankDepositCreateResponse>;
    listTransactions(): Promise<GasBankTransactionsResponse>;
  };
  payments: MiniAppSDK["payments"];
  governance: MiniAppSDK["governance"];
  rng: MiniAppSDK["rng"];
  datafeed: MiniAppSDK["datafeed"];
}

export type MiniAppSDKConfig = {
  edgeBaseUrl: string;
  getAuthToken?: () => Promise<string | undefined>;
  getAPIKey?: () => Promise<string | undefined>;
};
