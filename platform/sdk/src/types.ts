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

// Wallet invocation result shape varies by wallet implementation (NeoLine/O3/etc).
export type TxResult = unknown;

export type IntentWithTx<TIntent> = {
  intent: TIntent;
  tx: TxResult;
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
  signature?: string;
  public_key?: string;
  attestation_hash?: string;
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

// Note: Balance fields are serialized as strings from Go to avoid JS Number precision loss.
export type GasBankAccount = {
  id: string;
  user_id: string;
  balance: string;
  reserved: string;
  available: string;
  created_at: string;
  updated_at: string;
};

export type GasBankDepositStatus = "pending" | "confirming" | "confirmed" | "failed" | "expired";

export type GasBankDeposit = {
  id: string;
  amount: string;
  tx_hash?: string;
  from_address: string;
  status: GasBankDepositStatus;
  confirmations: number;
  created_at: string;
  confirmed_at?: string;
};

export type GasBankTransactionType = "deposit" | "withdraw" | "service_fee" | "refund";

export type GasBankTransaction = {
  id: string;
  tx_type: GasBankTransactionType;
  amount: string;
  balance_after: string;
  reference_id?: string;
  created_at: string;
};

export type GasBankAccountResponse = { account: GasBankAccount };
export type GasBankDepositsResponse = { deposits: GasBankDeposit[] };
export type GasBankTransactionsResponse = { transactions: GasBankTransaction[] };
export type GasBankDepositCreateResponse = { deposit: GasBankDeposit };

// Note: Price is serialized as string from Go to avoid JS Number precision loss.
export type PriceResponse = {
  feed_id: string;
  pair: string;
  price: string;
  decimals: number;
  timestamp: string;
  sources: string[];
  signature?: string;
  public_key?: string;
};

export type OracleQueryRequest = {
  url: string;
  method?: string;
  headers?: Record<string, string>;
  secret_name?: string;
  secret_as_key?: string;
  body?: string;
};

export type OracleQueryResponse = {
  status_code: number;
  headers: Record<string, string>;
  body: string;
};

export type ComputeExecuteRequest = {
  script: string;
  entry_point?: string;
  input?: Record<string, unknown>;
  secret_refs?: string[];
  timeout?: number;
};

export type ComputeJob = {
  job_id: string;
  status: string;
  output?: Record<string, unknown>;
  logs?: string[];
  error?: string;
  gas_used: number;
  started_at: string;
  duration?: string;
  encrypted_output?: string;
  output_hash?: string;
  signature?: string;
};

export type AutomationTriggerRequest = {
  name: string;
  trigger_type: string;
  schedule?: string;
  condition?: unknown;
  action: unknown;
};

export type AutomationTrigger = {
  id: string;
  user_id?: string;
  name: string;
  trigger_type: string;
  schedule?: string;
  condition?: unknown;
  action?: unknown;
  enabled: boolean;
  last_execution?: string;
  next_execution?: string;
  created_at: string;
};

export type AutomationExecution = {
  id: string;
  trigger_id: string;
  executed_at: string;
  success: boolean;
  error?: string;
  action_type?: string;
  action_payload?: unknown;
};

export type AutomationDeleteResponse = { status: "ok" };
export type AutomationStatusResponse = { status: string };

// Events
export type ContractEvent = {
  id: string;
  tx_hash: string;
  block_index: number;
  contract_hash: string;
  event_name: string;
  app_id?: string;
  state?: unknown;
  created_at: string;
};

export type EventsListParams = {
  app_id?: string;
  event_name?: string;
  contract_hash?: string;
  limit?: number;
  after_id?: string;
};

export type EventsListResponse = {
  events: ContractEvent[];
  has_more: boolean;
  last_id?: string;
};

// Transactions
export type ChainTransaction = {
  id: string;
  tx_hash?: string;
  request_id: string;
  from_service: string;
  tx_type: string;
  contract_address: string;
  method_name: string;
  status: string;
  gas_consumed?: number;
  submitted_at: string;
  confirmed_at?: string;
};

export type TransactionsListParams = {
  app_id?: string;
  limit?: number;
  after_id?: string;
};

export type TransactionsListResponse = {
  transactions: ChainTransaction[];
  has_more: boolean;
  last_id?: string;
};

export interface MiniAppSDK {
  // Blueprint-compatible convenience (alias of wallet.getAddress()).
  getAddress?: () => Promise<string>;
  wallet: {
    getAddress(): Promise<string>;
    // Optional: host-provided helper to submit a previously created invocation intent.
    invokeIntent?: (requestId: string) => Promise<unknown>;
    // Optional: directly invoke a returned invocation (wallet-dependent).
    invokeInvocation?: (invocation: InvocationIntent) => Promise<TxResult>;
  };
  payments: {
    payGAS(appId: string, amount: string, memo?: string): Promise<PayGASResponse>;
    // Convenience: create the intent via the gateway, then submit via the wallet.
    payGASAndInvoke?: (appId: string, amount: string, memo?: string) => Promise<IntentWithTx<PayGASResponse>>;
  };
  governance: {
    vote(appId: string, proposalId: string, neoAmount: string, support?: boolean): Promise<VoteNEOResponse>;
    // Convenience: create the intent via the gateway, then submit via the wallet.
    voteAndInvoke?: (
      appId: string,
      proposalId: string,
      neoAmount: string,
      support?: boolean,
    ) => Promise<IntentWithTx<VoteNEOResponse>>;
  };
  rng: {
    requestRandom(appId: string): Promise<RNGResponse>;
  };
  datafeed: {
    getPrice(symbol: string): Promise<PriceResponse>;
  };
  events: {
    list(params: EventsListParams): Promise<EventsListResponse>;
  };
  transactions: {
    list(params: TransactionsListParams): Promise<TransactionsListResponse>;
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
  oracle: {
    query(params: OracleQueryRequest): Promise<OracleQueryResponse>;
  };
  compute: {
    execute(params: ComputeExecuteRequest): Promise<ComputeJob>;
    listJobs(): Promise<ComputeJob[]>;
    getJob(id: string): Promise<ComputeJob>;
  };
  automation: {
    listTriggers(): Promise<AutomationTrigger[]>;
    createTrigger(params: AutomationTriggerRequest): Promise<AutomationTrigger>;
    getTrigger(id: string): Promise<AutomationTrigger>;
    updateTrigger(id: string, params: AutomationTriggerRequest): Promise<AutomationTrigger>;
    deleteTrigger(id: string): Promise<AutomationDeleteResponse>;
    enableTrigger(id: string): Promise<AutomationStatusResponse>;
    disableTrigger(id: string): Promise<AutomationStatusResponse>;
    resumeTrigger(id: string): Promise<AutomationStatusResponse>;
    listExecutions(id: string, limit?: number): Promise<AutomationExecution[]>;
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
    create(params: {
      name: string;
      scopes?: string[];
      description?: string;
      expires_at?: string;
    }): Promise<APIKeyCreateResponse>;
    revoke(id: string): Promise<APIKeyRevokeResponse>;
  };
  gasbank: {
    getAccount(): Promise<GasBankAccountResponse>;
    listDeposits(): Promise<GasBankDepositsResponse>;
    createDeposit(params: {
      amount: string;
      from_address: string;
      tx_hash?: string;
    }): Promise<GasBankDepositCreateResponse>;
    listTransactions(): Promise<GasBankTransactionsResponse>;
  };
  payments: MiniAppSDK["payments"];
  governance: MiniAppSDK["governance"];
  rng: MiniAppSDK["rng"];
  datafeed: MiniAppSDK["datafeed"];
  events: MiniAppSDK["events"];
  transactions: MiniAppSDK["transactions"];
}

export type MiniAppSDKConfig = {
  edgeBaseUrl: string;
  getAuthToken?: () => Promise<string | undefined>;
  getAPIKey?: () => Promise<string | undefined>;
};
