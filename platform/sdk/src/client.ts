import type {
  APIKeyCreateResponse,
  APIKeyRevokeResponse,
  APIKeysListResponse,
  AppRegisterResponse,
  AppUpdateManifestResponse,
  GasBankAccountResponse,
  GasBankDepositCreateResponse,
  GasBankDepositsResponse,
  GasBankTransactionsResponse,
  HostSDK,
  MiniAppSDK,
  MiniAppSDKConfig,
  PayGASResponse,
  PriceResponse,
  RNGResponse,
  SecretsDeleteResponse,
  SecretsGetResponse,
  SecretsListResponse,
  SecretsPermissionsResponse,
  SecretsUpsertResponse,
  VoteNEOResponse,
  WalletBindResponse,
  WalletNonceResponse,
} from "./types.js";

async function requestJSON<T>(
  cfg: MiniAppSDKConfig,
  path: string,
  init: RequestInit,
): Promise<T> {
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

export function createMiniAppSDK(cfg: MiniAppSDKConfig): MiniAppSDK {
  return {
    wallet: {
      async getAddress() {
        throw new Error("wallet integration not configured (use NeoLine/O3 dAPI in host app)");
      },
    },
    payments: {
      async payGAS(appId: string, amount: string, memo?: string): Promise<PayGASResponse> {
        return requestJSON<PayGASResponse>(cfg, "/pay-gas", {
          method: "POST",
          body: JSON.stringify({ app_id: appId, amount_gas: amount, memo }),
        });
      },
    },
    governance: {
      async vote(appId: string, proposalId: string, neoAmount: string, support?: boolean): Promise<VoteNEOResponse> {
        return requestJSON<VoteNEOResponse>(cfg, "/vote-neo", {
          method: "POST",
          body: JSON.stringify({
            app_id: appId,
            proposal_id: proposalId,
            neo_amount: neoAmount,
            support,
          }),
        });
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
        const base = cfg.edgeBaseUrl.replace(/\/$/, "");
        const url = `${base}/datafeed-price?symbol=${encodeURIComponent(symbol)}`;
        const resp = await fetch(url);
        const text = await resp.text();
        if (!resp.ok) throw new Error(text || `request failed (${resp.status})`);
        return JSON.parse(text) as PriceResponse;
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
    secrets: {
      async list(): Promise<SecretsListResponse> {
        return requestJSON<SecretsListResponse>(cfg, "/secrets-list", { method: "GET" });
      },
      async get(name: string): Promise<SecretsGetResponse> {
        return requestJSON<SecretsGetResponse>(cfg, `/secrets-get?name=${encodeURIComponent(name)}`, { method: "GET" });
      },
      async upsert(name: string, value: string): Promise<SecretsUpsertResponse> {
        return requestJSON<SecretsUpsertResponse>(cfg, "/secrets-upsert", {
          method: "POST",
          body: JSON.stringify({ name, value }),
        });
      },
      async delete(name: string): Promise<SecretsDeleteResponse> {
        return requestJSON<SecretsDeleteResponse>(cfg, "/secrets-delete", {
          method: "POST",
          body: JSON.stringify({ name }),
        });
      },
      async setPermissions(name: string, services: string[]): Promise<SecretsPermissionsResponse> {
        return requestJSON<SecretsPermissionsResponse>(cfg, "/secrets-permissions", {
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
