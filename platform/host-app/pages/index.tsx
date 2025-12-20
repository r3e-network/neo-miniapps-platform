import Head from "next/head";
import dynamic from "next/dynamic";
import { Component, ReactNode, useEffect, useMemo, useRef, useState } from "react";

type ContractParam =
  | { type: "String"; value: string }
  | { type: "Integer"; value: string }
  | { type: "Boolean"; value: boolean }
  | { type: "ByteArray"; value: string }
  | { type: "Hash160"; value: string }
  | { type: "Hash256"; value: string }
  | { type: "PublicKey"; value: string }
  | { type: "Any"; value: null }
  | { type: "Array"; value: ContractParam[] };

type InvocationIntent = {
  contract_hash: string;
  method: string;
  params: ContractParam[];
};

type WalletNonceResponse = { nonce: string; message: string };

type WalletBindResponse = {
  wallet: {
    id: string;
    address: string;
    label?: string | null;
    is_primary: boolean;
    verified: boolean;
    created_at: string;
  };
};

type PayGASResponse = {
  request_id: string;
  user_id: string;
  intent: "payments";
  constraints: { settlement: "GAS_ONLY" };
  invocation: InvocationIntent;
};

type VoteNEOResponse = {
  request_id: string;
  user_id: string;
  intent: "governance";
  constraints: { governance: "NEO_ONLY" };
  invocation: InvocationIntent;
};

type AppIntentResponse = {
  request_id: string;
  user_id: string;
  intent: "apps";
  manifest_hash?: string;
  invocation: InvocationIntent;
};

type SDKConfig = {
  edgeBaseUrl: string;
  getAuthToken?: () => Promise<string | undefined>;
  getAPIKey?: () => Promise<string | undefined>;
};

const BuiltinMiniApp = dynamic(
  () => import("builtin/App").then((mod: any) => mod.default ?? mod.App),
  {
    ssr: false,
    loading: () => <p>Loading federated MiniApp…</p>,
  },
);

class RemoteErrorBoundary extends Component<{ children: ReactNode }, { error?: Error }> {
  state: { error?: Error } = {};

  static getDerivedStateFromError(error: Error) {
    return { error };
  }

  render() {
    if (!this.state.error) return this.props.children;
    return (
      <div style={{ padding: 12, border: "1px solid #f2c6c6", borderRadius: 8, background: "#fff6f6" }}>
        <div style={{ fontWeight: 600, marginBottom: 6 }}>Failed to load federated MiniApp</div>
        <div style={{ fontSize: 12, color: "#8a2c2c" }}>{this.state.error.message}</div>
      </div>
    );
  }
}

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

  throw new Error("neo wallet not detected (install NeoLine N3) or add a host wallet bridge");
}

function getNeoLineN3Instance(): any {
  if (typeof window === "undefined") {
    throw new Error("wallet.* must be called in a browser context");
  }

  const g = window as any;

  const neoline = g?.NEOLineN3;
  if (!neoline || typeof neoline.Init !== "function") {
    throw new Error("NeoLine N3 not detected (install the NeoLine extension)");
  }

  return new neoline.Init();
}

async function signNeoLineMessage(message: string): Promise<{ publicKey: string; signature: string }> {
  const inst = getNeoLineN3Instance();
  if (!inst || typeof inst.signMessage !== "function") {
    throw new Error("wallet does not support signMessage (NeoLine N3 required)");
  }

  let res: any;
  try {
    res = await inst.signMessage({ message });
  } catch {
    res = await inst.signMessage(message);
  }

  const publicKey = String(
    res?.publicKey ?? res?.public_key ?? res?.pubkey ?? res?.account?.publicKey ?? res?.account?.public_key ?? "",
  ).trim();
  const signature = String(res?.signature ?? res?.sig ?? res?.signedData ?? res?.data?.signature ?? "").trim();

  if (!publicKey || !signature) {
    throw new Error("signMessage returned an unexpected shape (missing publicKey/signature)");
  }

  return { publicKey, signature };
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

async function requestJSON<T>(cfg: SDKConfig, path: string, init: RequestInit): Promise<T> {
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

type MiniAppSDKHooks = {
  rememberInvocation?: (requestId: string, invocation: InvocationIntent) => void;
  takeInvocation?: (requestId: string) => InvocationIntent | undefined;
};

function createMiniAppSDK(cfg: SDKConfig, hooks: MiniAppSDKHooks = {}) {
  return {
    // Blueprint-compatible convenience (alias of wallet.getAddress()).
    async getAddress(): Promise<string> {
      return getInjectedWalletAddress();
    },
    wallet: {
      async getAddress(): Promise<string> {
        return getInjectedWalletAddress();
      },
      async invokeIntent(requestId: string): Promise<unknown> {
        const id = String(requestId ?? "").trim();
        if (!id) throw new Error("request_id required");
        if (!hooks.takeInvocation) {
          throw new Error("invokeIntent not available in this host configuration");
        }
        const invocation = hooks.takeInvocation(id);
        if (!invocation) throw new Error("unknown request_id (no pending invocation)");
        return invokeNeoLineInvocation(invocation);
      },
    },
    payments: {
      async payGAS(appId: string, amount: string, memo?: string) {
        const res = await requestJSON<PayGASResponse>(cfg, "/pay-gas", {
          method: "POST",
          body: JSON.stringify({ app_id: appId, amount_gas: amount, memo }),
        });
        try {
          hooks.rememberInvocation?.(res.request_id, res.invocation);
        } catch {
          // ignore
        }
        return res;
      },
    },
    governance: {
      async vote(appId: string, proposalId: string, neoAmount: string, support?: boolean) {
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
          hooks.rememberInvocation?.(res.request_id, res.invocation);
        } catch {
          // ignore
        }
        return res;
      },
    },
    rng: {
      async requestRandom(appId: string) {
        return requestJSON(cfg, "/rng-request", {
          method: "POST",
          body: JSON.stringify({ app_id: appId }),
        });
      },
    },
    datafeed: {
      async getPrice(symbol: string) {
        const base = cfg.edgeBaseUrl.replace(/\/$/, "");
        const url = `${base}/datafeed-price?symbol=${encodeURIComponent(symbol)}`;
        const resp = await fetch(url);
        const text = await resp.text();
        if (!resp.ok) throw new Error(text || `request failed (${resp.status})`);
        return JSON.parse(text);
      },
    },
  };
}

const storageKeys = {
  entryUrl: "neo_host_entry_url",
  edgeBaseUrl: "neo_host_edge_base_url",
  authToken: "neo_host_auth_token",
  apiKey: "neo_host_api_key",
} as const;

function isSameOriginEntry(entryUrl: string): boolean {
  if (!entryUrl) return false;
  if (entryUrl.startsWith("/")) return true;
  try {
    return new URL(entryUrl).origin === window.location.origin;
  } catch {
    return false;
  }
}

const bridgeMessageTypes = {
  request: "neo_miniapp_sdk_request",
  response: "neo_miniapp_sdk_response",
} as const;

type MiniAppBridgeRequest = {
  type: typeof bridgeMessageTypes.request;
  id: string;
  method: string;
  params?: unknown[];
};

type MiniAppBridgeResponse =
  | {
      type: typeof bridgeMessageTypes.response;
      id: string;
      ok: true;
      result: unknown;
    }
  | {
      type: typeof bridgeMessageTypes.response;
      id: string;
      ok: false;
      error: string;
    };

function entryOriginFromURL(entryUrl: string): string | null {
  if (!entryUrl) return null;
  if (typeof window === "undefined") return null;
  try {
    return new URL(entryUrl, window.location.origin).origin;
  } catch {
    return null;
  }
}

type FederatedEntry = {
  remote: string;
  view?: string;
  appId?: string;
};

function parseFederatedEntry(entryUrl: string): FederatedEntry | null {
  const trimmed = entryUrl.trim();
  if (!trimmed.startsWith("mf://")) return null;
  try {
    const parsed = new URL(trimmed);
    const remote = parsed.hostname.trim();
    if (!remote) return null;
    const view = parsed.pathname.replace(/^\\/+/, "");
    const appId = parsed.searchParams.get("app") ?? undefined;
    return { remote, view: view || undefined, appId };
  } catch {
    return null;
  }
}

export default function Home() {
  const iframeRef = useRef<HTMLIFrameElement | null>(null);
  const pendingInvocationsRef = useRef<Map<string, InvocationIntent>>(new Map());

  const [entryUrl, setEntryUrl] = useState("");
  const [edgeBaseUrl, setEdgeBaseUrl] = useState("");
  const [authToken, setAuthToken] = useState("");
  const [apiKey, setAPIKey] = useState("");

  const [status, setStatus] = useState<string>("");
  const [actionStatus, setActionStatus] = useState<string>("");

  const [walletAddress, setWalletAddress] = useState<string>("");
  const [bindNonce, setBindNonce] = useState<string>("");
  const [bindMessage, setBindMessage] = useState<string>("");
  const [bindLabel, setBindLabel] = useState<string>("Primary");
  const [bindResult, setBindResult] = useState<string>("");

  const [payAppId, setPayAppId] = useState<string>("builtin-coin-flip");
  const [payAmount, setPayAmount] = useState<string>("1");
  const [payMemo, setPayMemo] = useState<string>("");
  const [payIntent, setPayIntent] = useState<PayGASResponse | null>(null);
  const [payTxResult, setPayTxResult] = useState<string>("");

  const [voteAppId, setVoteAppId] = useState<string>("");
  const [voteProposalId, setVoteProposalId] = useState<string>("proposal-1");
  const [voteAmount, setVoteAmount] = useState<string>("1");
  const [voteSupport, setVoteSupport] = useState<boolean>(true);
  const [voteIntent, setVoteIntent] = useState<VoteNEOResponse | null>(null);
  const [voteTxResult, setVoteTxResult] = useState<string>("");

  const [appManifest, setAppManifest] = useState<string>(() =>
    JSON.stringify(
      {
        app_id: "builtin-coin-flip",
        entry_url: "mf://builtin?app=builtin-coin-flip",
        name: "Neo Coin Flip",
        description: "Simple 50/50 coin flip game - double your GAS with provably fair on-chain randomness",
        version: "1.0.0",
        icon: "https://cdn.miniapps.com/miniapps/builtin/coin-flip/icon.png",
        developer_pubkey: "0x03f35d7ba09f0a14f0a0f8fdd2cd2db39647c80270f65a52d03d2cceb36b5250c5",
        permissions: {
          payments: true,
          governance: false,
          rng: true,
          datafeed: false,
        },
        assets_allowed: ["GAS"],
        governance_assets_allowed: ["NEO"],
        limits: {
          max_gas_per_tx: "20",
          daily_gas_cap_per_user: "200",
        },
        contracts_needed: ["PaymentHub", "RandomnessLog"],
        sandbox_flags: ["no-eval", "strict-csp"],
        attestation_required: true,
        category: "gaming",
        tags: ["coinflip", "gambling", "randomness", "simple"],
      },
      null,
      2,
    ),
  );
  const [appIntent, setAppIntent] = useState<AppIntentResponse | null>(null);
  const [appTxResult, setAppTxResult] = useState<string>("");

  useEffect(() => {
    if (typeof window === "undefined") return;

    const url = new URL(window.location.href);
    const entryFromQuery = (url.searchParams.get("entry_url") ?? "").trim();
    const edgeFromQuery = (url.searchParams.get("edge_base_url") ?? "").trim();

    const entryFromStorage = window.localStorage.getItem(storageKeys.entryUrl) ?? "";
    const edgeFromStorage = window.localStorage.getItem(storageKeys.edgeBaseUrl) ?? "";
    const tokenFromStorage = window.localStorage.getItem(storageKeys.authToken) ?? "";
    const apiKeyFromStorage = window.localStorage.getItem(storageKeys.apiKey) ?? "";

    setEntryUrl(entryFromQuery || entryFromStorage || "");
    setEdgeBaseUrl(edgeFromQuery || edgeFromStorage || "");
    setAuthToken(tokenFromStorage);
    setAPIKey(apiKeyFromStorage);
  }, []);

  useEffect(() => {
    if (typeof window === "undefined") return;
    window.localStorage.setItem(storageKeys.entryUrl, entryUrl);
  }, [entryUrl]);

  useEffect(() => {
    if (typeof window === "undefined") return;
    window.localStorage.setItem(storageKeys.edgeBaseUrl, edgeBaseUrl);
  }, [edgeBaseUrl]);

  useEffect(() => {
    if (typeof window === "undefined") return;
    window.localStorage.setItem(storageKeys.authToken, authToken);
  }, [authToken]);

  useEffect(() => {
    if (typeof window === "undefined") return;
    window.localStorage.setItem(storageKeys.apiKey, apiKey);
  }, [apiKey]);

  const federatedEntry = useMemo(() => parseFederatedEntry(entryUrl), [entryUrl]);
  const federatedUnsupported = useMemo(
    () => (federatedEntry ? federatedEntry.remote !== "builtin" : false),
    [federatedEntry],
  );

  const canInjectSDK = useMemo(() => {
    if (typeof window === "undefined") return false;
    if (federatedEntry) return false;
    return isSameOriginEntry(entryUrl);
  }, [entryUrl, federatedEntry]);

  const sdkCfg = useMemo((): SDKConfig | null => {
    const base = edgeBaseUrl.trim();
    if (!base) return null;
    return {
      edgeBaseUrl: base,
      getAuthToken: async () => authToken.trim() || undefined,
      getAPIKey: async () => apiKey.trim() || undefined,
    };
  }, [edgeBaseUrl, authToken, apiKey]);

  const sdk = useMemo(() => {
    if (!sdkCfg) return null;
    return createMiniAppSDK(sdkCfg, {
      rememberInvocation: (requestId, invocation) => {
        pendingInvocationsRef.current.set(requestId, invocation);
      },
      takeInvocation: (requestId) => {
        const inv = pendingInvocationsRef.current.get(requestId);
        pendingInvocationsRef.current.delete(requestId);
        return inv;
      },
    });
  }, [sdkCfg]);

  useEffect(() => {
    if (typeof window === "undefined") return;
    if (sdk) {
      (window as any).MiniAppSDK = sdk;
      window.dispatchEvent(new Event("miniapp-sdk-ready"));
      return;
    }
    if ((window as any).MiniAppSDK) {
      delete (window as any).MiniAppSDK;
    }
  }, [sdk]);

  useEffect(() => {
    if (typeof window === "undefined") return;

    const onMessage = async (event: MessageEvent) => {
      const data = event.data as Partial<MiniAppBridgeRequest> | null;
      if (!data || typeof data !== "object") return;
      if (data.type !== bridgeMessageTypes.request) return;

      const iframe = iframeRef.current;
      if (!iframe?.contentWindow) return;
      if (event.source !== iframe.contentWindow) return;

      const expectedOrigin = entryOriginFromURL(entryUrl);
      if (!expectedOrigin || event.origin !== expectedOrigin) return;

      const id = String(data.id ?? "").trim();
      const method = String(data.method ?? "").trim();
      const params = Array.isArray(data.params) ? data.params : [];

      const source = event.source as Window | null;
      if (!source || typeof source.postMessage !== "function") return;

      const reply = (resp: MiniAppBridgeResponse) => {
        try {
          source.postMessage(resp, event.origin);
        } catch {
          // Ignore postMessage failures (navigation/race).
        }
      };

      if (!id || !method) {
        reply({
          type: bridgeMessageTypes.response,
          id: id || "unknown",
          ok: false,
          error: "invalid rpc request",
        });
        return;
      }

      try {
        if (!sdk) throw new Error("MiniAppSDK not configured in host");

        let result: unknown;
        switch (method) {
          case "datafeed.getPrice": {
            const symbol = String(params[0] ?? "").trim();
            if (!symbol) throw new Error("symbol required");
            result = await (sdk as any).datafeed.getPrice(symbol);
            break;
          }
          case "rng.requestRandom": {
            const appId = String(params[0] ?? "").trim();
            if (!appId) throw new Error("app_id required");
            result = await (sdk as any).rng.requestRandom(appId);
            break;
          }
          case "payments.payGAS": {
            const appId = String(params[0] ?? "").trim();
            const amount = String(params[1] ?? "").trim();
            const memo = params.length >= 3 ? String(params[2] ?? "") : undefined;
            if (!appId) throw new Error("app_id required");
            if (!amount) throw new Error("amount required");
            result = await (sdk as any).payments.payGAS(appId, amount, memo);
            break;
          }
          case "governance.vote": {
            const appId = String(params[0] ?? "").trim();
            const proposalId = String(params[1] ?? "").trim();
            const neoAmount = String(params[2] ?? "").trim();
            const support = params.length >= 4 ? Boolean(params[3]) : undefined;
            if (!appId) throw new Error("app_id required");
            if (!proposalId) throw new Error("proposal_id required");
            if (!neoAmount) throw new Error("neo_amount required");
            result = await (sdk as any).governance.vote(appId, proposalId, neoAmount, support);
            break;
          }
          case "wallet.getAddress": {
            result = await (sdk as any).wallet.getAddress();
            break;
          }
          case "wallet.invokeIntent": {
            const requestId = String(params[0] ?? "").trim();
            if (!requestId) throw new Error("request_id required");
            result = await (sdk as any).wallet.invokeIntent(requestId);
            break;
          }
          default:
            throw new Error(`method not allowed: ${method}`);
        }

        reply({ type: bridgeMessageTypes.response, id, ok: true, result });
      } catch (err) {
        reply({
          type: bridgeMessageTypes.response,
          id,
          ok: false,
          error: String((err as any)?.message ?? err),
        });
      }
    };

    window.addEventListener("message", onMessage);
    return () => window.removeEventListener("message", onMessage);
  }, [entryUrl, sdk]);

  const injectSDK = () => {
    if (!sdk) return;
    if (!canInjectSDK) return;

    const iframe = iframeRef.current;
    if (!iframe?.contentWindow) return;

    try {
      // Safe only for same-origin MiniApps.
      (iframe.contentWindow as any).MiniAppSDK = sdk;
      setStatus("Injected MiniAppSDK into iframe (same-origin).");
    } catch (err) {
      setStatus(`Failed to inject SDK: ${String((err as any)?.message ?? err)}`);
    }
  };

  useEffect(() => {
    // If the SDK config changes while the iframe is already loaded, re-inject.
    injectSDK();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sdk, canInjectSDK]);

  const quickEntries = [
    {
      name: "Price Ticker (MF)",
      url: "mf://builtin?app=builtin-price-ticker",
    },
    {
      name: "Coin Flip (MF)",
      url: "mf://builtin?app=builtin-coin-flip",
    },
    {
      name: "Dice Game (MF)",
      url: "mf://builtin?app=builtin-dice-game",
    },
    {
      name: "Scratch Card (MF)",
      url: "mf://builtin?app=builtin-scratch-card",
    },
    {
      name: "Lottery (MF)",
      url: "mf://builtin?app=builtin-lottery",
    },
    {
      name: "Prediction Market (MF)",
      url: "mf://builtin?app=builtin-prediction-market",
    },
    {
      name: "Flashloan (MF)",
      url: "mf://builtin?app=builtin-flashloan",
    },
  ];

  const detectWallet = async () => {
    setActionStatus("");
    try {
      const addr = await getInjectedWalletAddress();
      setWalletAddress(addr);
      setActionStatus(`Detected wallet address: ${addr}`);
    } catch (err) {
      setActionStatus(`Wallet detection failed: ${String((err as any)?.message ?? err)}`);
    }
  };

  const issueBindMessage = async () => {
    setActionStatus("");
    setBindResult("");
    if (!sdkCfg) {
      setActionStatus("Set an Edge base URL first.");
      return;
    }
    try {
      const res = await requestJSON<WalletNonceResponse>(sdkCfg, "/wallet-nonce", {
        method: "POST",
        body: JSON.stringify({}),
      });
      setBindNonce(res.nonce);
      setBindMessage(res.message);
      setActionStatus("Issued bind nonce/message (now sign it in your wallet).");
    } catch (err) {
      setActionStatus(`wallet-nonce failed: ${String((err as any)?.message ?? err)}`);
    }
  };

  const bindWallet = async () => {
    setActionStatus("");
    setBindResult("");
    if (!sdkCfg) {
      setActionStatus("Set an Edge base URL first.");
      return;
    }
    try {
      const addr = walletAddress.trim() || (await getInjectedWalletAddress());
      if (!bindNonce.trim() || !bindMessage) {
        throw new Error("call wallet-nonce first");
      }

      const sig = await signNeoLineMessage(bindMessage);
      const res = await requestJSON<WalletBindResponse>(sdkCfg, "/wallet-bind", {
        method: "POST",
        body: JSON.stringify({
          address: addr,
          public_key: sig.publicKey,
          signature: sig.signature,
          message: bindMessage,
          nonce: bindNonce,
          label: bindLabel.trim() || undefined,
        }),
      });

      setWalletAddress(addr);
      setBindResult(JSON.stringify(res, null, 2));
      setActionStatus("Wallet bound successfully.");
    } catch (err) {
      setActionStatus(`wallet-bind failed: ${String((err as any)?.message ?? err)}`);
    }
  };

  const createPayIntent = async () => {
    setActionStatus("");
    setPayIntent(null);
    setPayTxResult("");
    if (!sdkCfg) {
      setActionStatus("Set an Edge base URL first.");
      return;
    }
    try {
      const res = await requestJSON<PayGASResponse>(sdkCfg, "/pay-gas", {
        method: "POST",
        body: JSON.stringify({
          app_id: payAppId,
          amount_gas: payAmount,
          memo: payMemo || undefined,
        }),
      });
      setPayIntent(res);
      setActionStatus("Created PayGAS invocation intent.");
    } catch (err) {
      setActionStatus(`pay-gas failed: ${String((err as any)?.message ?? err)}`);
    }
  };

  const submitPayIntent = async () => {
    setActionStatus("");
    setPayTxResult("");
    if (!payIntent) {
      setActionStatus("Create a PayGAS intent first.");
      return;
    }
    try {
      const tx = await invokeNeoLineInvocation(payIntent.invocation);
      setPayTxResult(JSON.stringify(tx, null, 2));
      setActionStatus("Submitted PayGAS invocation via wallet.");
    } catch (err) {
      setActionStatus(`wallet.invoke failed: ${String((err as any)?.message ?? err)}`);
    }
  };

  const createVoteIntent = async () => {
    setActionStatus("");
    setVoteIntent(null);
    setVoteTxResult("");
    if (!sdkCfg) {
      setActionStatus("Set an Edge base URL first.");
      return;
    }
    try {
      const res = await requestJSON<VoteNEOResponse>(sdkCfg, "/vote-neo", {
        method: "POST",
        body: JSON.stringify({
          app_id: voteAppId,
          proposal_id: voteProposalId,
          neo_amount: voteAmount,
          support: voteSupport,
        }),
      });
      setVoteIntent(res);
      setActionStatus("Created Vote invocation intent.");
    } catch (err) {
      setActionStatus(`vote-neo failed: ${String((err as any)?.message ?? err)}`);
    }
  };

  const submitVoteIntent = async () => {
    setActionStatus("");
    setVoteTxResult("");
    if (!voteIntent) {
      setActionStatus("Create a Vote intent first.");
      return;
    }
    try {
      const tx = await invokeNeoLineInvocation(voteIntent.invocation);
      setVoteTxResult(JSON.stringify(tx, null, 2));
      setActionStatus("Submitted Vote invocation via wallet.");
    } catch (err) {
      setActionStatus(`wallet.invoke failed: ${String((err as any)?.message ?? err)}`);
    }
  };

  const parseAppManifest = (): unknown => {
    const raw = appManifest.trim();
    if (!raw) throw new Error("manifest JSON required");
    let obj: unknown;
    try {
      obj = JSON.parse(raw);
    } catch (e) {
      throw new Error(`manifest must be valid JSON: ${(e as any)?.message ?? e}`);
    }
    if (!obj || typeof obj !== "object" || Array.isArray(obj)) {
      throw new Error("manifest must be a JSON object");
    }
    return obj;
  };

  const buildAppRegisterIntent = async () => {
    setActionStatus("");
    setAppIntent(null);
    setAppTxResult("");
    if (!sdkCfg) {
      setActionStatus("Set an Edge base URL first.");
      return;
    }
    if (!authToken.trim() && !apiKey.trim()) {
      setActionStatus("Set an Auth JWT or API key first.");
      return;
    }
    try {
      const manifest = parseAppManifest();
      const res = await requestJSON<AppIntentResponse>(sdkCfg, "/app-register", {
        method: "POST",
        body: JSON.stringify({ manifest }),
      });
      setAppIntent(res);
      setActionStatus("Created AppRegistry.register invocation intent.");
    } catch (err) {
      setActionStatus(`app-register failed: ${String((err as any)?.message ?? err)}`);
    }
  };

  const buildAppUpdateManifestIntent = async () => {
    setActionStatus("");
    setAppIntent(null);
    setAppTxResult("");
    if (!sdkCfg) {
      setActionStatus("Set an Edge base URL first.");
      return;
    }
    if (!authToken.trim() && !apiKey.trim()) {
      setActionStatus("Set an Auth JWT or API key first.");
      return;
    }
    try {
      const manifest = parseAppManifest();
      const res = await requestJSON<AppIntentResponse>(sdkCfg, "/app-update-manifest", {
        method: "POST",
        body: JSON.stringify({ manifest }),
      });
      setAppIntent(res);
      setActionStatus("Created AppRegistry.updateManifest invocation intent.");
    } catch (err) {
      setActionStatus(`app-update-manifest failed: ${String((err as any)?.message ?? err)}`);
    }
  };

  const submitAppIntent = async () => {
    setActionStatus("");
    setAppTxResult("");
    if (!appIntent) {
      setActionStatus("Create an app intent first.");
      return;
    }
    try {
      const tx = await invokeNeoLineInvocation(appIntent.invocation);
      setAppTxResult(JSON.stringify(tx, null, 2));
      setActionStatus("Submitted AppRegistry invocation via wallet.");
    } catch (err) {
      setActionStatus(`wallet.invoke failed: ${String((err as any)?.message ?? err)}`);
    }
  };

  return (
    <>
      <Head>
        <title>Neo MiniApp Host</title>
        <meta name="viewport" content="width=device-width, initial-scale=1" />
      </Head>
      <main style={{ padding: 24, fontFamily: "system-ui, sans-serif" }}>
        <h1 style={{ margin: "0 0 12px" }}>Neo MiniApp Host</h1>
        <p style={{ margin: "0 0 16px", maxWidth: 820 }}>
          This host embeds third-party MiniApps via <code>iframe</code> and loads trusted built-ins via Module Federation
          (use <code>mf://builtin?app=...</code> or the <code>/federated</code> route). For same-origin MiniApps (served
          from <code>/public</code>), it can inject a <code>MiniAppSDK</code> object into the iframe for local previews.
        </p>

        <section
          style={{
            border: "1px solid #ddd",
            borderRadius: 10,
            padding: 16,
            marginBottom: 16,
            maxWidth: 980,
          }}
        >
          <h2 style={{ margin: "0 0 10px", fontSize: 16 }}>Settings</h2>

          <div style={{ display: "flex", gap: 10, flexWrap: "wrap" }}>
            <label style={{ display: "flex", flexDirection: "column", gap: 6 }}>
              <span style={{ fontSize: 12, opacity: 0.8 }}>MiniApp URL</span>
              <input
                style={{ padding: "8px 10px", width: 520 }}
                value={entryUrl}
                onChange={(e) => setEntryUrl(e.target.value)}
              />
            </label>

            <label style={{ display: "flex", flexDirection: "column", gap: 6 }}>
              <span style={{ fontSize: 12, opacity: 0.8 }}>Supabase Edge base URL</span>
              <input
                style={{ padding: "8px 10px", width: 420 }}
                value={edgeBaseUrl}
                onChange={(e) => setEdgeBaseUrl(e.target.value)}
              />
            </label>
          </div>

          <div
            style={{
              display: "flex",
              gap: 10,
              flexWrap: "wrap",
              marginTop: 12,
            }}
          >
            <label style={{ display: "flex", flexDirection: "column", gap: 6 }}>
              <span style={{ fontSize: 12, opacity: 0.8 }}>Auth JWT (optional)</span>
              <input
                style={{ padding: "8px 10px", width: 520 }}
                type="password"
                autoComplete="off"
                value={authToken}
                onChange={(e) => setAuthToken(e.target.value)}
              />
            </label>

            <label style={{ display: "flex", flexDirection: "column", gap: 6 }}>
              <span style={{ fontSize: 12, opacity: 0.8 }}>API key (optional)</span>
              <input
                style={{ padding: "8px 10px", width: 420 }}
                type="password"
                autoComplete="off"
                value={apiKey}
                onChange={(e) => setAPIKey(e.target.value)}
              />
            </label>
          </div>

          <div
            style={{
              marginTop: 12,
              display: "flex",
              gap: 10,
              flexWrap: "wrap",
            }}
          >
            {quickEntries.map((d) => (
              <button
                key={d.url}
                style={{
                  padding: "8px 10px",
                  borderRadius: 8,
                  border: "1px solid #ddd",
                  background: "#fafafa",
                  cursor: "pointer",
                }}
                onClick={() => setEntryUrl(d.url)}
              >
                Load: {d.name}
              </button>
            ))}
            <button
              style={{
                padding: "8px 10px",
                borderRadius: 8,
                border: "1px solid #ddd",
                background: "#fff",
                cursor: "pointer",
              }}
              onClick={injectSDK}
              disabled={!sdk || !canInjectSDK}
              title={
                !sdk
                  ? "Set an Edge base URL first"
                  : federatedEntry
                    ? "Module Federation uses the host SDK directly"
                    : !canInjectSDK
                      ? "SDK injection only works for same-origin entry URLs"
                      : "Inject SDK into the iframe"
              }
            >
              Inject SDK
            </button>
          </div>

          <p style={{ margin: "10px 0 0", fontSize: 12, opacity: 0.85 }}>
            SDK injection:{" "}
            {entryUrl
              ? canInjectSDK
                ? "enabled (same-origin)"
                : "disabled (cross-origin; MiniApp must bundle the SDK or use postMessage bridge)"
              : "n/a"}
          </p>
          {status ? <p style={{ margin: "6px 0 0", fontSize: 12, opacity: 0.85 }}>Status: {status}</p> : null}
          {actionStatus ? (
            <p style={{ margin: "6px 0 0", fontSize: 12, opacity: 0.85 }}>Action: {actionStatus}</p>
          ) : null}
        </section>

        <section
          style={{
            border: "1px solid #ddd",
            borderRadius: 10,
            padding: 16,
            marginBottom: 16,
            maxWidth: 980,
          }}
        >
          <h2 style={{ margin: "0 0 10px", fontSize: 16 }}>Wallet Binding (NeoLine N3)</h2>
          <p
            style={{
              margin: "0 0 10px",
              fontSize: 12,
              opacity: 0.85,
              maxWidth: 860,
            }}
          >
            Wallet binding requires a Supabase Auth JWT (OAuth session). Use <code>wallet-nonce</code> to get a message,
            then sign it in your Neo wallet and submit <code>wallet-bind</code>.
          </p>

          <div
            style={{
              display: "flex",
              gap: 10,
              flexWrap: "wrap",
              alignItems: "flex-end",
            }}
          >
            <label style={{ display: "flex", flexDirection: "column", gap: 6 }}>
              <span style={{ fontSize: 12, opacity: 0.8 }}>Wallet address</span>
              <input
                style={{ padding: "8px 10px", width: 360 }}
                value={walletAddress}
                onChange={(e) => setWalletAddress(e.target.value)}
              />
            </label>

            <label style={{ display: "flex", flexDirection: "column", gap: 6 }}>
              <span style={{ fontSize: 12, opacity: 0.8 }}>Label (optional)</span>
              <input
                style={{ padding: "8px 10px", width: 240 }}
                value={bindLabel}
                onChange={(e) => setBindLabel(e.target.value)}
              />
            </label>

            <button
              style={{
                padding: "8px 10px",
                borderRadius: 8,
                border: "1px solid #ddd",
                background: "#fff",
                cursor: "pointer",
              }}
              onClick={detectWallet}
            >
              Detect Wallet
            </button>
            <button
              style={{
                padding: "8px 10px",
                borderRadius: 8,
                border: "1px solid #ddd",
                background: "#fff",
                cursor: "pointer",
              }}
              onClick={issueBindMessage}
              disabled={!sdkCfg || !authToken.trim()}
              title={!authToken.trim() ? "Set Auth JWT in Settings (wallet binding requires JWT)" : undefined}
            >
              Get Bind Message
            </button>
            <button
              style={{
                padding: "8px 10px",
                borderRadius: 8,
                border: "1px solid #ddd",
                background: "#fafafa",
                cursor: "pointer",
              }}
              onClick={bindWallet}
              disabled={!sdkCfg || !authToken.trim() || !bindNonce.trim()}
              title={!bindNonce.trim() ? "Call Get Bind Message first" : undefined}
            >
              Sign & Bind
            </button>
          </div>

          {bindMessage ? (
            <div style={{ marginTop: 10 }}>
              <div style={{ fontSize: 12, opacity: 0.8, marginBottom: 6 }}>Bind message</div>
              <pre
                style={{
                  background: "#f7f7f7",
                  padding: 12,
                  borderRadius: 8,
                  overflow: "auto",
                }}
              >
                {bindMessage}
              </pre>
            </div>
          ) : null}

          {bindResult ? (
            <div style={{ marginTop: 10 }}>
              <div style={{ fontSize: 12, opacity: 0.8, marginBottom: 6 }}>Bind result</div>
              <pre
                style={{
                  background: "#f7f7f7",
                  padding: 12,
                  borderRadius: 8,
                  overflow: "auto",
                }}
              >
                {bindResult}
              </pre>
            </div>
          ) : null}
        </section>

        <section
          style={{
            border: "1px solid #ddd",
            borderRadius: 10,
            padding: 16,
            marginBottom: 16,
            maxWidth: 980,
          }}
        >
          <h2 style={{ margin: "0 0 10px", fontSize: 16 }}>On-chain Intents</h2>
          <p
            style={{
              margin: "0 0 10px",
              fontSize: 12,
              opacity: 0.85,
              maxWidth: 860,
            }}
          >
            These endpoints return an <code>invocation</code> intent; the host then asks the user wallet to sign and
            submit it. <code>pay-gas</code>/<code>vote-neo</code> require a primary wallet binding.
          </p>

          <div style={{ display: "grid", gridTemplateColumns: "1fr", gap: 14 }}>
            <div style={{ border: "1px solid #eee", borderRadius: 10, padding: 12 }}>
              <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 8 }}>Pay GAS</div>
              <div style={{ display: "flex", gap: 10, flexWrap: "wrap" }}>
                <label style={{ display: "flex", flexDirection: "column", gap: 6 }}>
                  <span style={{ fontSize: 12, opacity: 0.8 }}>app_id</span>
                  <input
                    style={{ padding: "8px 10px", width: 280 }}
                    value={payAppId}
                    onChange={(e) => setPayAppId(e.target.value)}
                  />
                </label>
                <label style={{ display: "flex", flexDirection: "column", gap: 6 }}>
                  <span style={{ fontSize: 12, opacity: 0.8 }}>amount_gas</span>
                  <input
                    style={{ padding: "8px 10px", width: 120 }}
                    value={payAmount}
                    onChange={(e) => setPayAmount(e.target.value)}
                  />
                </label>
                <label style={{ display: "flex", flexDirection: "column", gap: 6 }}>
                  <span style={{ fontSize: 12, opacity: 0.8 }}>memo (optional)</span>
                  <input
                    style={{ padding: "8px 10px", width: 360 }}
                    value={payMemo}
                    onChange={(e) => setPayMemo(e.target.value)}
                  />
                </label>
                <button
                  style={{
                    padding: "8px 10px",
                    borderRadius: 8,
                    border: "1px solid #ddd",
                    background: "#fff",
                    cursor: "pointer",
                    height: 36,
                    alignSelf: "flex-end",
                  }}
                  onClick={createPayIntent}
                  disabled={!sdkCfg}
                >
                  Create Intent
                </button>
                <button
                  style={{
                    padding: "8px 10px",
                    borderRadius: 8,
                    border: "1px solid #ddd",
                    background: "#fafafa",
                    cursor: "pointer",
                    height: 36,
                    alignSelf: "flex-end",
                  }}
                  onClick={submitPayIntent}
                  disabled={!payIntent}
                  title={!payIntent ? "Create an intent first" : undefined}
                >
                  Submit via Wallet
                </button>
              </div>

              {payIntent ? (
                <pre
                  style={{
                    background: "#f7f7f7",
                    padding: 12,
                    borderRadius: 8,
                    overflow: "auto",
                    marginTop: 10,
                  }}
                >
                  {JSON.stringify(payIntent, null, 2)}
                </pre>
              ) : null}
              {payTxResult ? (
                <pre
                  style={{
                    background: "#f7f7f7",
                    padding: 12,
                    borderRadius: 8,
                    overflow: "auto",
                    marginTop: 10,
                  }}
                >
                  {payTxResult}
                </pre>
              ) : null}
            </div>

            <div style={{ border: "1px solid #eee", borderRadius: 10, padding: 12 }}>
              <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 8 }}>Vote (NEO)</div>
              <div style={{ display: "flex", gap: 10, flexWrap: "wrap" }}>
                <label style={{ display: "flex", flexDirection: "column", gap: 6 }}>
                  <span style={{ fontSize: 12, opacity: 0.8 }}>app_id</span>
                  <input
                    style={{ padding: "8px 10px", width: 280 }}
                    value={voteAppId}
                    onChange={(e) => setVoteAppId(e.target.value)}
                  />
                </label>
                <label style={{ display: "flex", flexDirection: "column", gap: 6 }}>
                  <span style={{ fontSize: 12, opacity: 0.8 }}>proposal_id</span>
                  <input
                    style={{ padding: "8px 10px", width: 200 }}
                    value={voteProposalId}
                    onChange={(e) => setVoteProposalId(e.target.value)}
                  />
                </label>
                <label style={{ display: "flex", flexDirection: "column", gap: 6 }}>
                  <span style={{ fontSize: 12, opacity: 0.8 }}>neo_amount</span>
                  <input
                    style={{ padding: "8px 10px", width: 120 }}
                    value={voteAmount}
                    onChange={(e) => setVoteAmount(e.target.value)}
                  />
                </label>
                <label style={{ display: "flex", flexDirection: "column", gap: 6 }}>
                  <span style={{ fontSize: 12, opacity: 0.8 }}>support</span>
                  <select
                    style={{ padding: "8px 10px", width: 140 }}
                    value={voteSupport ? "yes" : "no"}
                    onChange={(e) => setVoteSupport(e.target.value === "yes")}
                  >
                    <option value="yes">true</option>
                    <option value="no">false</option>
                  </select>
                </label>
                <button
                  style={{
                    padding: "8px 10px",
                    borderRadius: 8,
                    border: "1px solid #ddd",
                    background: "#fff",
                    cursor: "pointer",
                    height: 36,
                    alignSelf: "flex-end",
                  }}
                  onClick={createVoteIntent}
                  disabled={!sdkCfg}
                >
                  Create Intent
                </button>
                <button
                  style={{
                    padding: "8px 10px",
                    borderRadius: 8,
                    border: "1px solid #ddd",
                    background: "#fafafa",
                    cursor: "pointer",
                    height: 36,
                    alignSelf: "flex-end",
                  }}
                  onClick={submitVoteIntent}
                  disabled={!voteIntent}
                  title={!voteIntent ? "Create an intent first" : undefined}
                >
                  Submit via Wallet
                </button>
              </div>

              {voteIntent ? (
                <pre
                  style={{
                    background: "#f7f7f7",
                    padding: 12,
                    borderRadius: 8,
                    overflow: "auto",
                    marginTop: 10,
                  }}
                >
                  {JSON.stringify(voteIntent, null, 2)}
                </pre>
              ) : null}
              {voteTxResult ? (
                <pre
                  style={{
                    background: "#f7f7f7",
                    padding: 12,
                    borderRadius: 8,
                    overflow: "auto",
                    marginTop: 10,
                  }}
                >
                  {voteTxResult}
                </pre>
              ) : null}
            </div>
          </div>
        </section>

        <section
          style={{
            border: "1px solid #ddd",
            borderRadius: 10,
            padding: 16,
            marginBottom: 16,
            maxWidth: 980,
          }}
        >
          <h2 style={{ margin: "0 0 10px", fontSize: 16 }}>App Registry (Host-only)</h2>
          <p style={{ margin: "0 0 10px", fontSize: 12, opacity: 0.85 }}>
            Build an <code>AppRegistry</code> invocation intent from a manifest (hashing + asset policy enforced by
            Edge), then submit it via the wallet. Requires auth + a bound primary wallet.
          </p>

          <div style={{ display: "flex", gap: 10, flexWrap: "wrap" }}>
            <button
              style={{
                padding: "8px 10px",
                borderRadius: 8,
                border: "1px solid #ddd",
                background: "#fff",
                cursor: "pointer",
              }}
              onClick={buildAppRegisterIntent}
              disabled={!sdkCfg || (!authToken.trim() && !apiKey.trim())}
            >
              Build Register Intent
            </button>
            <button
              style={{
                padding: "8px 10px",
                borderRadius: 8,
                border: "1px solid #ddd",
                background: "#fff",
                cursor: "pointer",
              }}
              onClick={buildAppUpdateManifestIntent}
              disabled={!sdkCfg || (!authToken.trim() && !apiKey.trim())}
            >
              Build Update Intent
            </button>
            <button
              style={{
                padding: "8px 10px",
                borderRadius: 8,
                border: "1px solid #ddd",
                background: "#fafafa",
                cursor: "pointer",
              }}
              onClick={submitAppIntent}
              disabled={!appIntent}
              title={!appIntent ? "Build an intent first" : undefined}
            >
              Submit via Wallet
            </button>
          </div>

          <div style={{ marginTop: 10 }}>
            <div style={{ fontSize: 12, opacity: 0.8, marginBottom: 6 }}>Manifest JSON</div>
            <textarea
              style={{
                width: "100%",
                minHeight: 240,
                padding: 12,
                borderRadius: 8,
                border: "1px solid #ddd",
                fontFamily:
                  'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
                fontSize: 12,
              }}
              value={appManifest}
              onChange={(e) => setAppManifest(e.target.value)}
            />
          </div>

          {appIntent ? (
            <pre
              style={{
                background: "#f7f7f7",
                padding: 12,
                borderRadius: 8,
                overflow: "auto",
                marginTop: 10,
              }}
            >
              {JSON.stringify(appIntent, null, 2)}
            </pre>
          ) : null}
          {appTxResult ? (
            <pre
              style={{
                background: "#f7f7f7",
                padding: 12,
                borderRadius: 8,
                overflow: "auto",
                marginTop: 10,
              }}
            >
              {appTxResult}
            </pre>
          ) : null}
        </section>

        {!entryUrl ? (
          <div style={{ maxWidth: 900 }}>
            <p style={{ margin: "0 0 12px" }}>
              Pick a MiniApp above, or provide an <code>entry_url</code> query param:
            </p>
            <pre style={{ background: "#111", color: "#eee", padding: 12 }}>
              {`/?entry_url=mf://builtin?app=builtin-price-ticker`}
            </pre>
            <pre style={{ background: "#111", color: "#eee", padding: 12 }}>
              {`/?entry_url=/miniapps/builtin/coin-flip/index.html`}
            </pre>
            <pre style={{ background: "#111", color: "#eee", padding: 12 }}>
              {`/?entry_url=https%3A%2F%2Fcdn.miniapps.com%2Fapps%2Fneo-game%2Findex.html`}
            </pre>
          </div>
        ) : federatedEntry ? (
          federatedUnsupported ? (
            <div style={{ maxWidth: 900, padding: 12, border: "1px solid #f2c6c6", borderRadius: 8 }}>
              Unsupported Module Federation remote: <code>{federatedEntry.remote}</code>
            </div>
          ) : (
            <RemoteErrorBoundary>
              <BuiltinMiniApp appId={federatedEntry.appId} view={federatedEntry.view} />
            </RemoteErrorBoundary>
          )
        ) : (
          <iframe
            ref={iframeRef}
            title="MiniApp"
            src={entryUrl}
            onLoad={injectSDK}
            style={{
              width: "100%",
              height: "80vh",
              border: "1px solid #ddd",
              borderRadius: 8,
            }}
            sandbox="allow-scripts allow-same-origin"
          />
        )}
      </main>
    </>
  );
}
