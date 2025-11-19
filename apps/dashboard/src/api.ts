export type Descriptor = {
  name: string;
  domain: string;
  layer: string;
  capabilities?: string[];
};

export type HealthCheck = {
  status: string;
  version?: string;
};

export type Account = {
  ID: string;
  Owner: string;
  Metadata?: Record<string, string>;
  CreatedAt?: string;
  UpdatedAt?: string;
};

export type WorkspaceWallet = {
  ID: string;
  WorkspaceID: string;
  WalletAddress: string;
  Label?: string;
  Status?: string;
  CreatedAt?: string;
  UpdatedAt?: string;
};

export type VRFKey = {
  ID: string;
  AccountID: string;
  PublicKey: string;
  Status?: string;
  WalletAddress?: string;
  Attestation?: string;
  Metadata?: Record<string, string>;
};

export type VRFRequest = {
  ID: string;
  AccountID: string;
  KeyID: string;
  Consumer: string;
  Seed: string;
  Status: string;
  Metadata?: Record<string, string>;
};

export type Lane = {
  ID: string;
  AccountID: string;
  Name: string;
  SourceChain: string;
  DestChain: string;
  SignerSet?: string[];
  AllowedTokens?: string[];
  Metadata?: Record<string, string>;
  Tags?: string[];
};

export type CCIPMessage = {
  ID: string;
  AccountID: string;
  LaneID: string;
  Status: string;
  Payload?: Record<string, any>;
  TokenTransfers?: { Token: string; Amount: string; Recipient: string }[];
  Metadata?: Record<string, string>;
  Tags?: string[];
};

export type Datafeed = {
  ID: string;
  AccountID: string;
  Pair: string;
  Decimals: number;
  SignerSet?: string[];
  Metadata?: Record<string, string>;
  Tags?: string[];
};

export type DatafeedUpdate = {
  ID: string;
  RoundID: number;
  Price: string;
  Timestamp: string;
  Signature?: string;
};

export type DatalinkChannel = {
  ID: string;
  AccountID: string;
  Name: string;
  Endpoint: string;
  SignerSet?: string[];
  Metadata?: Record<string, string>;
  Tags?: string[];
};

export type DatalinkDelivery = {
  ID: string;
  AccountID: string;
  ChannelID: string;
  Status: string;
  Metadata?: Record<string, string>;
};

export type Datastream = {
  ID: string;
  AccountID: string;
  Name: string;
  Symbol: string;
  Frequency: string;
  SLAMs?: number;
  Status?: string;
  Metadata?: Record<string, string>;
};

export type DatastreamFrame = {
  ID: string;
  StreamID: string;
  Sequence: number;
  Payload: Record<string, any>;
  LatencyMs?: number;
  Status?: string;
};

export type DTAProduct = {
  ID: string;
  AccountID: string;
  Name: string;
  Symbol: string;
  Type: string;
  Status?: string;
  SettlementTerms?: string;
};

export type DTAOrder = {
  ID: string;
  AccountID: string;
  ProductID: string;
  Type: string;
  Amount: string;
  WalletAddress: string;
  Status?: string;
  Metadata?: Record<string, string>;
};

export type GasAccount = {
  ID: string;
  AccountID: string;
  WalletAddress: string;
  Available: number;
  Pending: number;
  Locked: number;
};

export type GasTransaction = {
  ID: string;
  AccountID: string;
  GasAccountID: string;
  Amount: number;
  Status: string;
  ToAddress: string;
  FromAddress: string;
};

export type Enclave = {
  ID: string;
  AccountID: string;
  Name: string;
  Provider: string;
  Measurement: string;
  Status?: string;
};

export type CREExecutor = {
  ID: string;
  AccountID: string;
  Name: string;
  Type: string;
  Endpoint: string;
  Metadata?: Record<string, string>;
};

export type CREPlaybook = {
  ID: string;
  AccountID: string;
  Name: string;
  Description?: string;
  Tags?: string[];
};

export type CRERun = {
  ID: string;
  AccountID: string;
  PlaybookID: string;
  ExecutorID?: string;
  Status: string;
};

export type AutomationJob = {
  ID: string;
  AccountID: string;
  Name: string;
  Schedule: string;
  Enabled: boolean;
};

export type Secret = {
  ID: string;
  AccountID: string;
  Name: string;
  CreatedAt: string;
  UpdatedAt: string;
  Metadata?: Record<string, string>;
};

export type FunctionSummary = {
  ID: string;
  AccountID: string;
  Name: string;
  Runtime: string;
  Status?: string;
  CreatedAt?: string;
  UpdatedAt?: string;
};

export type FunctionExecution = {
  ID: string;
  FunctionID: string;
  AccountID: string;
  Status: string;
  StartedAt?: string;
  CompletedAt?: string;
  Error?: string;
};

export type OracleSource = {
  ID: string;
  AccountID: string;
  Name: string;
  URL: string;
  AuthType?: string;
  Status?: string;
};

export type OracleRequest = {
  ID: string;
  AccountID: string;
  SourceID: string;
  Status: string;
  CreatedAt?: string;
  UpdatedAt?: string;
  Error?: string;
};

export type RandomRequest = {
  ID: string;
  AccountID: string;
  Status: string;
  Length: number;
  CreatedAt?: string;
  CompletedAt?: string;
};

export type Trigger = {
  ID: string;
  AccountID: string;
  Type: string;
  Rule: string;
  FunctionID: string;
};

export type ClientConfig = {
  baseUrl: string;
  token: string;
};

export function normaliseUrl(url: string) {
  return url.trim().replace(/\/$/, "");
}

export async function fetchJSON<T>(url: string, config: ClientConfig): Promise<T> {
  const resp = await fetch(url, {
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${config.token}`,
    },
  });
  if (!resp.ok) {
    const text = await resp.text();
    throw new Error(`${resp.status} ${resp.statusText}: ${text}`);
  }
  return resp.json() as Promise<T>;
}

export async function fetchDescriptors(config: ClientConfig): Promise<Descriptor[]> {
  const url = `${config.baseUrl}/system/descriptors`;
  return fetchJSON<Descriptor[]>(url, config);
}

export async function fetchHealth(config: ClientConfig): Promise<HealthCheck> {
  const url = `${config.baseUrl}/system/health`;
  return fetchJSON<HealthCheck>(url, config);
}

export async function fetchAccounts(config: ClientConfig, limit = 50): Promise<Account[]> {
  const url = `${config.baseUrl}/accounts?limit=${limit}`;
  return fetchJSON<Account[]>(url, config);
}

export async function fetchWorkspaceWallets(config: ClientConfig, accountID: string, limit = 50): Promise<WorkspaceWallet[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/workspace-wallets?limit=${limit}`;
  return fetchJSON<WorkspaceWallet[]>(url, config);
}

export async function fetchVRFKeys(config: ClientConfig, accountID: string, limit = 50): Promise<VRFKey[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/vrf/keys?limit=${limit}`;
  return fetchJSON<VRFKey[]>(url, config);
}

export async function fetchVRFRequests(config: ClientConfig, accountID: string, limit = 50): Promise<VRFRequest[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/vrf/requests?limit=${limit}`;
  return fetchJSON<VRFRequest[]>(url, config);
}

export async function fetchLanes(config: ClientConfig, accountID: string, limit = 50): Promise<Lane[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/ccip/lanes?limit=${limit}`;
  return fetchJSON<Lane[]>(url, config);
}

export async function fetchMessages(config: ClientConfig, accountID: string, limit = 50): Promise<CCIPMessage[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/ccip/messages?limit=${limit}`;
  return fetchJSON<CCIPMessage[]>(url, config);
}

export async function fetchDatafeeds(config: ClientConfig, accountID: string, limit = 50): Promise<Datafeed[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/datafeeds?limit=${limit}`;
  return fetchJSON<Datafeed[]>(url, config);
}

export async function fetchDatafeedUpdates(config: ClientConfig, accountID: string, feedID: string, limit = 20): Promise<DatafeedUpdate[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/datafeeds/${feedID}/updates?limit=${limit}`;
  return fetchJSON<DatafeedUpdate[]>(url, config);
}

export async function fetchDatalinkChannels(config: ClientConfig, accountID: string, limit = 50): Promise<DatalinkChannel[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/datalink/channels?limit=${limit}`;
  return fetchJSON<DatalinkChannel[]>(url, config);
}

export async function fetchDatalinkDeliveries(config: ClientConfig, accountID: string, limit = 50): Promise<DatalinkDelivery[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/datalink/deliveries?limit=${limit}`;
  return fetchJSON<DatalinkDelivery[]>(url, config);
}

export async function fetchDatastreams(config: ClientConfig, accountID: string, limit = 50): Promise<Datastream[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/datastreams?limit=${limit}`;
  return fetchJSON<Datastream[]>(url, config);
}

export async function fetchDatastreamFrames(config: ClientConfig, accountID: string, streamID: string, limit = 20): Promise<DatastreamFrame[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/datastreams/${streamID}/frames?limit=${limit}`;
  return fetchJSON<DatastreamFrame[]>(url, config);
}

export async function fetchDTAProducts(config: ClientConfig, accountID: string, limit = 50): Promise<DTAProduct[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/dta/products?limit=${limit}`;
  return fetchJSON<DTAProduct[]>(url, config);
}

export async function fetchDTAOrders(config: ClientConfig, accountID: string, limit = 50): Promise<DTAOrder[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/dta/orders?limit=${limit}`;
  return fetchJSON<DTAOrder[]>(url, config);
}

export async function fetchGasAccounts(config: ClientConfig, accountID: string, limit = 50): Promise<GasAccount[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/gasbank?limit=${limit}`;
  return fetchJSON<GasAccount[]>(url, config);
}

export async function fetchGasTransactions(config: ClientConfig, accountID: string, gasAccountID?: string, limit = 50): Promise<GasTransaction[]> {
  const param = gasAccountID ? `?gas_account_id=${encodeURIComponent(gasAccountID)}&limit=${limit}` : `?limit=${limit}`;
  const url = `${config.baseUrl}/accounts/${accountID}/gasbank/transactions${param}`;
  return fetchJSON<GasTransaction[]>(url, config);
}

export async function fetchEnclaves(config: ClientConfig, accountID: string, limit = 50): Promise<Enclave[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/confcompute/enclaves?limit=${limit}`;
  return fetchJSON<Enclave[]>(url, config);
}

export async function fetchCREExecutors(config: ClientConfig, accountID: string, limit = 50): Promise<CREExecutor[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/cre/executors?limit=${limit}`;
  return fetchJSON<CREExecutor[]>(url, config);
}

export async function fetchCREPlaybooks(config: ClientConfig, accountID: string, limit = 50): Promise<CREPlaybook[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/cre/playbooks?limit=${limit}`;
  return fetchJSON<CREPlaybook[]>(url, config);
}

export async function fetchCRERuns(config: ClientConfig, accountID: string, limit = 50): Promise<CRERun[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/cre/runs?limit=${limit}`;
  return fetchJSON<CRERun[]>(url, config);
}

export async function fetchAutomationJobs(config: ClientConfig, accountID: string, limit = 50): Promise<AutomationJob[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/automation/jobs?limit=${limit}`;
  return fetchJSON<AutomationJob[]>(url, config);
}

export async function fetchTriggers(config: ClientConfig, accountID: string, limit = 50): Promise<Trigger[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/triggers?limit=${limit}`;
  return fetchJSON<Trigger[]>(url, config);
}

export async function fetchSecrets(config: ClientConfig, accountID: string, limit = 50): Promise<Secret[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/secrets?limit=${limit}`;
  return fetchJSON<Secret[]>(url, config);
}

export async function fetchFunctions(config: ClientConfig, accountID: string, limit = 50): Promise<FunctionSummary[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/functions?limit=${limit}`;
  return fetchJSON<FunctionSummary[]>(url, config);
}

export async function fetchFunctionExecutions(config: ClientConfig, accountID: string, functionID: string, limit = 20): Promise<FunctionExecution[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/functions/${functionID}/executions?limit=${limit}`;
  return fetchJSON<FunctionExecution[]>(url, config);
}

export async function fetchOracleSources(config: ClientConfig, accountID: string, limit = 50): Promise<OracleSource[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/oracle/sources?limit=${limit}`;
  return fetchJSON<OracleSource[]>(url, config);
}

export async function fetchOracleRequests(config: ClientConfig, accountID: string, limit = 50): Promise<OracleRequest[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/oracle/requests?limit=${limit}`;
  return fetchJSON<OracleRequest[]>(url, config);
}

export async function fetchRandomRequests(config: ClientConfig, accountID: string, limit = 50): Promise<RandomRequest[]> {
  const url = `${config.baseUrl}/accounts/${accountID}/random/requests?limit=${limit}`;
  return fetchJSON<RandomRequest[]>(url, config);
}
