import type {
  APIKeyCreateResponse,
  APIKeyRevokeResponse,
  APIKeysListResponse,
  AppRegisterResponse,
  AppUpdateManifestResponse,
  AutomationDeleteResponse,
  AutomationExecution,
  AutomationStatusResponse,
  AutomationTrigger,
  AutomationTriggerRequest,
  EventsListParams,
  EventsListResponse,
  GasBankAccountResponse,
  GasBankDepositCreateResponse,
  GasBankDepositsResponse,
  GasBankTransactionsResponse,
  HostSDK,
  InvocationIntent,
  MiniAppSDK,
  MiniAppSDKConfig,
  ComputeExecuteRequest,
  ComputeJob,
  OracleQueryRequest,
  OracleQueryResponse,
  PayGASResponse,
  PriceResponse,
  RNGResponse,
  SecretsDeleteResponse,
  SecretsGetResponse,
  SecretsListResponse,
  SecretsPermissionsResponse,
  SecretsUpsertResponse,
  TransactionsListParams,
  TransactionsListResponse,
  VoteNEOResponse,
  WalletBindResponse,
  WalletNonceResponse,
} from "./types.js";

async function getInjectedWalletAddress(): Promise<string> {
  if (typeof window === "undefined") {
    throw new Error("wallet.getAddress must be called in a browser context");
  }

  const g = window as any;

  // NeoLine N3 (common browser wallet).
  const neoline = g?.NEOLineN3;
  if (neoline && typeof neoline.Init === "function") {
    const inst = new neoline.Init();
    if (inst && typeof inst.getAccount === "function") {
      const res = await inst.getAccount();
      const addr = String(res?.address ?? res?.account?.address ?? "").trim();
      if (addr) return addr;
    }
  }

  throw new Error("neo wallet not detected (install NeoLine N3) or host must bridge wallet.getAddress");
}

function getNeoLineN3Instance(): any {
  if (typeof window === "undefined") {
    throw new Error("wallet invocation must be called in a browser context");
  }

  const g = window as any;
  const neoline = g?.NEOLineN3;
  if (!neoline || typeof neoline.Init !== "function") {
    throw new Error("NeoLine N3 not detected (install the NeoLine extension)");
  }

  return new neoline.Init();
}

async function invokeNeoLineInvocation(invocation: InvocationIntent): Promise<unknown> {
  const inst = getNeoLineN3Instance();
  if (!inst || typeof inst.invoke !== "function") {
    throw new Error("wallet does not support invoke (NeoLine N3 required)");
  }

  const scriptHash = String(invocation.contract_hash ?? "").trim();
  const operation = String(invocation.method ?? "").trim();
  const args = Array.isArray(invocation.params) ? invocation.params : [];

  if (!scriptHash) throw new Error("invocation missing contract_hash");
  if (!operation) throw new Error("invocation missing method");

  // NeoLine SDKs vary slightly in accepted shapes; try a small set of candidates.
  const address = await getInjectedWalletAddress();
  const candidates = [
    { scriptHash, operation, args },
    { scriptHash: scriptHash.replace(/^0x/i, ""), operation, args },
    { scriptHash, operation, args, signers: [{ account: address, scopes: "CalledByEntry" }] },
    { scriptHash, operation, args, signers: [{ account: address, scopes: 1 }] },
    {
      scriptHash: scriptHash.replace(/^0x/i, ""),
      operation,
      args,
      signers: [{ account: address, scopes: "CalledByEntry" }],
    },
    { scriptHash: scriptHash.replace(/^0x/i, ""), operation, args, signers: [{ account: address, scopes: 1 }] },
  ];

  let lastErr: unknown = null;
  for (const params of candidates) {
    try {
      return await inst.invoke(params);
    } catch (err) {
      lastErr = err;
    }
  }

  throw lastErr instanceof Error ? lastErr : new Error(String(lastErr ?? "invoke failed"));
}

async function requestJSON<T>(cfg: MiniAppSDKConfig, path: string, init: RequestInit): Promise<T> {
  const base = cfg.edgeBaseUrl.replace(/\/$/, "");
  const url = `${base}${path.startsWith("/") ? "" : "/"}${path}`;

  const headers = new Headers(init.headers);
  headers.set("Content-Type", "application/json");
  if (cfg.getAuthToken) {
    const token = await cfg.getAuthToken();
    if (token) headers.set("Authorization", `Bearer ${token}`);
  }
  if (!headers.get("Authorization") && cfg.getAPIKey) {
    const apiKey = await cfg.getAPIKey();
    if (apiKey) headers.set("X-API-Key", apiKey);
  }

  const resp = await fetch(url, { ...init, headers });
  const text = await resp.text();
  if (!resp.ok) throw new Error(text || `request failed (${resp.status})`);
  return JSON.parse(text) as T;
}

async function requestHostJSON<T>(cfg: MiniAppSDKConfig, path: string, init: RequestInit): Promise<T> {
  const base = cfg.edgeBaseUrl.replace(/\/$/, "");
  const url = `${base}${path.startsWith("/") ? "" : "/"}${path}`;

  const headers = new Headers(init.headers);
  headers.set("Content-Type", "application/json");

  const apiKey = cfg.getAPIKey ? await cfg.getAPIKey() : undefined;
  if (!apiKey) {
    throw new Error("API key required for host-only endpoint");
  }
  headers.set("X-API-Key", apiKey);

  const resp = await fetch(url, { ...init, headers });
  const text = await resp.text();
  if (!resp.ok) throw new Error(text || `request failed (${resp.status})`);
  return JSON.parse(text) as T;
}

export function createMiniAppSDK(cfg: MiniAppSDKConfig): MiniAppSDK {
  const pendingInvocations = new Map<string, InvocationIntent>();

  return {
    async getAddress() {
      return getInjectedWalletAddress();
    },
    wallet: {
      async getAddress() {
        return getInjectedWalletAddress();
      },
      async invokeIntent(requestId: string): Promise<unknown> {
        const id = String(requestId ?? "").trim();
        if (!id) throw new Error("request_id required");
        const invocation = pendingInvocations.get(id);
        if (!invocation) throw new Error("unknown request_id (no pending invocation)");
        pendingInvocations.delete(id);
        return invokeNeoLineInvocation(invocation);
      },
      async invokeInvocation(invocation: InvocationIntent): Promise<unknown> {
        return invokeNeoLineInvocation(invocation);
      },
    },
    payments: {
      async payGAS(appId: string, amount: string, memo?: string): Promise<PayGASResponse> {
        const res = await requestJSON<PayGASResponse>(cfg, "/pay-gas", {
          method: "POST",
          body: JSON.stringify({ app_id: appId, amount_gas: amount, memo }),
        });
        try {
          pendingInvocations.set(res.request_id, res.invocation);
        } catch {
          // ignore
        }
        return res;
      },
      async payGASAndInvoke(appId: string, amount: string, memo?: string) {
        const intent = await this.payGAS(appId, amount, memo);
        const tx = await invokeNeoLineInvocation(intent.invocation);
        return { intent, tx };
      },
    },
    governance: {
      async vote(appId: string, proposalId: string, neoAmount: string, support?: boolean): Promise<VoteNEOResponse> {
        const res = await requestJSON<VoteNEOResponse>(cfg, "/vote-neo", {
          method: "POST",
          body: JSON.stringify({
            app_id: appId,
            proposal_id: proposalId,
            neo_amount: neoAmount,
            support,
          }),
        });
        try {
          pendingInvocations.set(res.request_id, res.invocation);
        } catch {
          // ignore
        }
        return res;
      },
      async voteAndInvoke(appId: string, proposalId: string, neoAmount: string, support?: boolean) {
        const intent = await this.vote(appId, proposalId, neoAmount, support);
        const tx = await invokeNeoLineInvocation(intent.invocation);
        return { intent, tx };
      },
    },
    rng: {
      async requestRandom(appId: string): Promise<RNGResponse> {
        return requestJSON<RNGResponse>(cfg, "/rng-request", {
          method: "POST",
          body: JSON.stringify({ app_id: appId }),
        });
      },
    },
    datafeed: {
      async getPrice(symbol: string): Promise<PriceResponse> {
        return requestJSON<PriceResponse>(cfg, `/datafeed-price?symbol=${encodeURIComponent(symbol)}`, {
          method: "GET",
        });
      },
    },
    events: {
      async list(params: EventsListParams): Promise<EventsListResponse> {
        const qs = new URLSearchParams();
        if (params.app_id) qs.set("app_id", params.app_id);
        if (params.event_name) qs.set("event_name", params.event_name);
        if (params.contract_hash) qs.set("contract_hash", params.contract_hash);
        if (params.limit) qs.set("limit", String(params.limit));
        if (params.after_id) qs.set("after_id", params.after_id);
        return requestJSON<EventsListResponse>(cfg, `/events-list?${qs.toString()}`, { method: "GET" });
      },
    },
    transactions: {
      async list(params: TransactionsListParams): Promise<TransactionsListResponse> {
        const qs = new URLSearchParams();
        if (params.app_id) qs.set("app_id", params.app_id);
        if (params.limit) qs.set("limit", String(params.limit));
        if (params.after_id) qs.set("after_id", params.after_id);
        return requestJSON<TransactionsListResponse>(cfg, `/transactions-list?${qs.toString()}`, { method: "GET" });
      },
    },
  };
}

export function createHostSDK(cfg: MiniAppSDKConfig): HostSDK {
  const mini = createMiniAppSDK(cfg);

  return {
    ...mini,
    wallet: {
      ...mini.wallet,
      async getBindMessage(): Promise<WalletNonceResponse> {
        return requestJSON<WalletNonceResponse>(cfg, "/wallet-nonce", {
          method: "POST",
          body: JSON.stringify({}),
        });
      },
      async bindWallet(params): Promise<WalletBindResponse> {
        return requestJSON<WalletBindResponse>(cfg, "/wallet-bind", {
          method: "POST",
          body: JSON.stringify({
            address: params.address,
            public_key: params.publicKey,
            signature: params.signature,
            message: params.message,
            nonce: params.nonce,
            label: params.label,
          }),
        });
      },
    },
    apps: {
      async register(params): Promise<AppRegisterResponse> {
        return requestJSON<AppRegisterResponse>(cfg, "/app-register", {
          method: "POST",
          body: JSON.stringify({
            manifest: params.manifest,
          }),
        });
      },
      async updateManifest(params): Promise<AppUpdateManifestResponse> {
        return requestJSON<AppUpdateManifestResponse>(cfg, "/app-update-manifest", {
          method: "POST",
          body: JSON.stringify({
            manifest: params.manifest,
          }),
        });
      },
    },
    oracle: {
      async query(params: OracleQueryRequest): Promise<OracleQueryResponse> {
        return requestHostJSON<OracleQueryResponse>(cfg, "/oracle-query", {
          method: "POST",
          body: JSON.stringify(params),
        });
      },
    },
    compute: {
      async execute(params: ComputeExecuteRequest): Promise<ComputeJob> {
        return requestHostJSON<ComputeJob>(cfg, "/compute-execute", {
          method: "POST",
          body: JSON.stringify(params),
        });
      },
      async listJobs(): Promise<ComputeJob[]> {
        return requestHostJSON<ComputeJob[]>(cfg, "/compute-jobs", { method: "GET" });
      },
      async getJob(id: string): Promise<ComputeJob> {
        return requestHostJSON<ComputeJob>(cfg, `/compute-job?id=${encodeURIComponent(id)}`, { method: "GET" });
      },
    },
    automation: {
      async listTriggers(): Promise<AutomationTrigger[]> {
        return requestHostJSON<AutomationTrigger[]>(cfg, "/automation-triggers", { method: "GET" });
      },
      async createTrigger(params: AutomationTriggerRequest): Promise<AutomationTrigger> {
        return requestHostJSON<AutomationTrigger>(cfg, "/automation-triggers", {
          method: "POST",
          body: JSON.stringify(params),
        });
      },
      async getTrigger(id: string): Promise<AutomationTrigger> {
        return requestHostJSON<AutomationTrigger>(cfg, `/automation-trigger?id=${encodeURIComponent(id)}`, {
          method: "GET",
        });
      },
      async updateTrigger(id: string, params: AutomationTriggerRequest): Promise<AutomationTrigger> {
        return requestHostJSON<AutomationTrigger>(cfg, "/automation-trigger-update", {
          method: "POST",
          body: JSON.stringify({ id, ...params }),
        });
      },
      async deleteTrigger(id: string): Promise<AutomationDeleteResponse> {
        return requestHostJSON<AutomationDeleteResponse>(cfg, "/automation-trigger-delete", {
          method: "POST",
          body: JSON.stringify({ id }),
        });
      },
      async enableTrigger(id: string): Promise<AutomationStatusResponse> {
        return requestHostJSON<AutomationStatusResponse>(cfg, "/automation-trigger-enable", {
          method: "POST",
          body: JSON.stringify({ id }),
        });
      },
      async disableTrigger(id: string): Promise<AutomationStatusResponse> {
        return requestHostJSON<AutomationStatusResponse>(cfg, "/automation-trigger-disable", {
          method: "POST",
          body: JSON.stringify({ id }),
        });
      },
      async resumeTrigger(id: string): Promise<AutomationStatusResponse> {
        return requestHostJSON<AutomationStatusResponse>(cfg, "/automation-trigger-resume", {
          method: "POST",
          body: JSON.stringify({ id }),
        });
      },
      async listExecutions(id: string, limit?: number): Promise<AutomationExecution[]> {
        const qs = new URLSearchParams({ id });
        if (typeof limit === "number" && Number.isFinite(limit)) qs.set("limit", String(limit));
        return requestHostJSON<AutomationExecution[]>(cfg, `/automation-trigger-executions?${qs.toString()}`, {
          method: "GET",
        });
      },
    },
    secrets: {
      async list(): Promise<SecretsListResponse> {
        return requestHostJSON<SecretsListResponse>(cfg, "/secrets-list", { method: "GET" });
      },
      async get(name: string): Promise<SecretsGetResponse> {
        return requestHostJSON<SecretsGetResponse>(cfg, `/secrets-get?name=${encodeURIComponent(name)}`, {
          method: "GET",
        });
      },
      async upsert(name: string, value: string): Promise<SecretsUpsertResponse> {
        return requestHostJSON<SecretsUpsertResponse>(cfg, "/secrets-upsert", {
          method: "POST",
          body: JSON.stringify({ name, value }),
        });
      },
      async delete(name: string): Promise<SecretsDeleteResponse> {
        return requestHostJSON<SecretsDeleteResponse>(cfg, "/secrets-delete", {
          method: "POST",
          body: JSON.stringify({ name }),
        });
      },
      async setPermissions(name: string, services: string[]): Promise<SecretsPermissionsResponse> {
        return requestHostJSON<SecretsPermissionsResponse>(cfg, "/secrets-permissions", {
          method: "POST",
          body: JSON.stringify({ name, services }),
        });
      },
    },
    apiKeys: {
      async list(): Promise<APIKeysListResponse> {
        return requestJSON<APIKeysListResponse>(cfg, "/api-keys-list", { method: "GET" });
      },
      async create(params): Promise<APIKeyCreateResponse> {
        return requestJSON<APIKeyCreateResponse>(cfg, "/api-keys-create", {
          method: "POST",
          body: JSON.stringify({
            name: params.name,
            scopes: params.scopes,
            description: params.description,
            expires_at: params.expires_at,
          }),
        });
      },
      async revoke(id: string): Promise<APIKeyRevokeResponse> {
        return requestJSON<APIKeyRevokeResponse>(cfg, "/api-keys-revoke", {
          method: "POST",
          body: JSON.stringify({ id }),
        });
      },
    },
    gasbank: {
      async getAccount(): Promise<GasBankAccountResponse> {
        return requestJSON<GasBankAccountResponse>(cfg, "/gasbank-account", { method: "GET" });
      },
      async listDeposits(): Promise<GasBankDepositsResponse> {
        return requestJSON<GasBankDepositsResponse>(cfg, "/gasbank-deposits", { method: "GET" });
      },
      async createDeposit(params): Promise<GasBankDepositCreateResponse> {
        return requestJSON<GasBankDepositCreateResponse>(cfg, "/gasbank-deposit", {
          method: "POST",
          body: JSON.stringify({
            amount: params.amount,
            from_address: params.from_address,
            tx_hash: params.tx_hash,
          }),
        });
      },
      async listTransactions(): Promise<GasBankTransactionsResponse> {
        return requestJSON<GasBankTransactionsResponse>(cfg, "/gasbank-transactions", { method: "GET" });
      },
    },
  };
}
