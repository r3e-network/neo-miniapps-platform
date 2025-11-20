import { FormEvent, useEffect, useMemo, useState } from "react";
import {
  Account,
  AutomationJob,
  CCIPMessage,
  CREExecutor,
  CREPlaybook,
  CRERun,
  Datafeed,
  DatafeedUpdate,
  DatalinkChannel,
  DatalinkDelivery,
  Datastream,
  DatastreamFrame,
  Descriptor,
  DTAOrder,
  DTAProduct,
  Enclave,
  FunctionExecution,
  FunctionSummary,
  GasAccount,
  GasTransaction,
  GasbankSummary,
  GasbankDeadLetter,
  GasbankSettlementAttempt,
  PriceFeed,
  PriceSnapshot,
  Lane,
  OracleRequest,
  OracleSource,
  RandomRequest,
  Secret,
  Trigger,
  VRFKey,
  VRFRequest,
  WorkspaceWallet,
  fetchAccounts,
  fetchAutomationJobs,
  fetchCREExecutors,
  fetchCREPlaybooks,
  fetchCRERuns,
  fetchDatafeedUpdates,
  fetchDatafeeds,
  fetchDatalinkChannels,
  fetchDatalinkDeliveries,
  fetchDatastreamFrames,
  fetchDatastreams,
  fetchDescriptors,
  fetchDTAOrders,
  fetchDTAProducts,
  fetchEnclaves,
  fetchFunctionExecutions,
  fetchFunctions,
  fetchGasAccounts,
  fetchGasTransactions,
  fetchGasbankSummary,
  fetchGasWithdrawals,
  fetchGasDeadLetters,
  fetchWithdrawalAttempts,
  fetchPriceFeeds,
  fetchPriceSnapshots,
  fetchHealth,
  fetchLanes,
  fetchMessages,
  fetchOracleRequests,
  fetchOracleSources,
  fetchRandomRequests,
  fetchSecrets,
  fetchTriggers,
  fetchVRFKeys,
  fetchVRFRequests,
  fetchWorkspaceWallets,
  normaliseUrl,
} from "./api";
import { useLocalStorage } from "./useLocalStorage";
import { MetricSample, MetricsConfig, promQuery, promQueryRange, TimeSeries } from "./metrics";
import { Chart } from "./components/Chart";

const amountFormatter = new Intl.NumberFormat(undefined, { maximumFractionDigits: 3 });
const timeFormatter = new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" });

function formatAmount(value: number | undefined): string {
  if (typeof value !== "number" || Number.isNaN(value)) {
    return "0";
  }
  return amountFormatter.format(value);
}

function formatTimestamp(value?: string): string {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return timeFormatter.format(date);
}

type State =
  | { status: "idle" }
  | { status: "loading" }
  | {
      status: "ready";
      descriptors: Descriptor[];
      accounts: Account[];
      version?: string;
      metrics?: {
        rps?: MetricSample[];
        duration?: TimeSeries[];
      };
    }
  | { status: "error"; message: string };

type WalletState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; items: WorkspaceWallet[] }
  | { status: "error"; message: string };

type VRFState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; keys: VRFKey[]; requests: VRFRequest[] }
  | { status: "error"; message: string };

type CCIPState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; lanes: Lane[]; messages: CCIPMessage[] }
  | { status: "error"; message: string };

type DatafeedsState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; feeds: Datafeed[]; updates: Record<string, DatafeedUpdate[]> }
  | { status: "error"; message: string };

type PricefeedsState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; feeds: PriceFeed[]; snapshots: Record<string, PriceSnapshot[]> }
  | { status: "error"; message: string };

type DatalinkState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; channels: DatalinkChannel[]; deliveries: DatalinkDelivery[] }
  | { status: "error"; message: string };

type DatastreamsState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; streams: Datastream[]; frames: Record<string, DatastreamFrame[]> }
  | { status: "error"; message: string };

type DTAState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; products: DTAProduct[]; orders: DTAOrder[] }
  | { status: "error"; message: string };

type GasbankState =
  | { status: "idle" }
  | { status: "loading" }
  | {
      status: "ready";
      summary: GasbankSummary;
      accounts: GasAccount[];
      transactions: GasTransaction[];
      withdrawals: GasTransaction[];
      deadletters: GasbankDeadLetter[];
      attempts: Record<string, GasbankSettlementAttempt[]>;
    }
  | { status: "error"; message: string };

type ConfState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; enclaves: Enclave[] }
  | { status: "error"; message: string };

type CREState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; executors: CREExecutor[]; playbooks: CREPlaybook[]; runs: CRERun[] }
  | { status: "error"; message: string };

type AutomationState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; jobs: AutomationJob[]; triggers: Trigger[] }
  | { status: "error"; message: string };

type SecretsState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; items: Secret[] }
  | { status: "error"; message: string };

type FunctionsState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; items: { fn: FunctionSummary; executions: FunctionExecution[] }[] }
  | { status: "error"; message: string };

type OracleState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; sources: OracleSource[]; requests: OracleRequest[] }
  | { status: "error"; message: string };

type RandomState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; requests: RandomRequest[] }
  | { status: "error"; message: string };

function formatSnippet(value: string, limit = 32) {
  if (!value) {
    return "";
  }
  return value.length > limit ? `${value.slice(0, limit)}…` : value;
}

export function App() {
  const [baseUrl, setBaseUrl] = useLocalStorage("sl-ui.baseUrl", "http://localhost:8080");
  const [token, setToken] = useLocalStorage("sl-ui.token", "");
  const config = useMemo(
    () => ({
      baseUrl: normaliseUrl(baseUrl),
      token: token.trim(),
    }),
    [baseUrl, token],
  );
  const [promBase, setPromBase] = useLocalStorage("sl-ui.prometheus", "http://localhost:9090");
  const promConfig: MetricsConfig = useMemo(
    () => ({
      baseUrl: config.baseUrl,
      token: config.token,
      prometheusBaseUrl: normaliseUrl(promBase),
    }),
    [config, promBase],
  );

  const [state, setState] = useState<State>({ status: "idle" });
  const [wallets, setWallets] = useState<Record<string, WalletState>>({});
  const [vrf, setVRF] = useState<Record<string, VRFState>>({});
  const [ccip, setCCIP] = useState<Record<string, CCIPState>>({});
  const [datafeeds, setDatafeeds] = useState<Record<string, DatafeedsState>>({});
  const [pricefeeds, setPricefeeds] = useState<Record<string, PricefeedsState>>({});
  const [datalink, setDatalink] = useState<Record<string, DatalinkState>>({});
  const [datastreams, setDatastreams] = useState<Record<string, DatastreamsState>>({});
  const [dta, setDTA] = useState<Record<string, DTAState>>({});
  const [gasbank, setGasbank] = useState<Record<string, GasbankState>>({});
  const [conf, setConf] = useState<Record<string, ConfState>>({});
  const [cre, setCRE] = useState<Record<string, CREState>>({});
  const [automation, setAutomation] = useState<Record<string, AutomationState>>({});
  const [secrets, setSecrets] = useState<Record<string, SecretsState>>({});
  const [functionsState, setFunctionsState] = useState<Record<string, FunctionsState>>({});
  const [oracle, setOracle] = useState<Record<string, OracleState>>({});
  const [random, setRandom] = useState<Record<string, RandomState>>({});
  const [systemVersion, setSystemVersion] = useState<string>("");

  const canQuery = config.baseUrl.length > 0 && config.token.length > 0;

  async function load() {
    if (!canQuery) {
      setState({ status: "idle" });
      return;
    }
    setState({ status: "loading" });
    try {
      const [health, descriptors, accounts, version] = await Promise.all([
        fetchHealth(config),
        fetchDescriptors(config),
        fetchAccounts(config),
        fetchVersion(config),
      ]);
      let metrics: { rps?: MetricSample[]; duration?: TimeSeries[] } | undefined;
      if (promConfig.prometheusBaseUrl) {
        try {
          const [rps, duration] = await Promise.all([
            promQuery('sum(rate(http_requests_total[5m])) by (status)', promConfig),
            promQueryRange("histogram_quantile(0.9, sum by (le) (rate(http_request_duration_seconds_bucket[5m])))", Date.now() / 1000 - 1800, Date.now() / 1000, 60, promConfig),
          ]);
          metrics = { rps, duration };
        } catch {
          metrics = undefined;
        }
      }
      setState({ status: "ready", descriptors, accounts, version: health.version ?? version.version, metrics });
      setSystemVersion(version.version);
      setWallets({});
      setVRF({});
      setCCIP({});
      setDatafeeds({});
      setPricefeeds({});
      setDatalink({});
      setDatastreams({});
      setDTA({});
      setGasbank({});
      setConf({});
      setCRE({});
      setAutomation({});
      setSecrets({});
      setFunctionsState({});
      setOracle({});
      setRandom({});
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setState({ status: "error", message });
    }
  }

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [config.baseUrl, config.token]);

  function onSubmit(event: FormEvent) {
    event.preventDefault();
    void load();
  }

  async function loadWallets(accountID: string) {
    setWallets((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
    try {
      const data = await fetchWorkspaceWallets(config, accountID);
      setWallets((prev) => ({ ...prev, [accountID]: { status: "ready", items: data } }));
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setWallets((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
    }
  }

  async function loadVRF(accountID: string) {
    setVRF((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
    try {
      const [keys, requests] = await Promise.all([fetchVRFKeys(config, accountID), fetchVRFRequests(config, accountID)]);
      setVRF((prev) => ({ ...prev, [accountID]: { status: "ready", keys, requests } }));
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setVRF((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
    }
  }

  async function loadCCIP(accountID: string) {
    setCCIP((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
    try {
      const [lanes, messages] = await Promise.all([fetchLanes(config, accountID), fetchMessages(config, accountID)]);
      setCCIP((prev) => ({ ...prev, [accountID]: { status: "ready", lanes, messages } }));
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setCCIP((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
    }
  }

  async function loadDatafeeds(accountID: string) {
    setDatafeeds((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
    try {
      const feeds = await fetchDatafeeds(config, accountID);
      const updates: Record<string, DatafeedUpdate[]> = {};
      for (const feed of feeds) {
        const resp = await fetchDatafeedUpdates(config, accountID, feed.ID, 5);
        updates[feed.ID] = resp;
      }
      setDatafeeds((prev) => ({ ...prev, [accountID]: { status: "ready", feeds, updates } }));
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setDatafeeds((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
    }
  }

  async function loadPricefeeds(accountID: string) {
    setPricefeeds((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
    try {
      const feeds = await fetchPriceFeeds(config, accountID);
      const snapshots: Record<string, PriceSnapshot[]> = {};
      for (const feed of feeds) {
        snapshots[feed.ID] = await fetchPriceSnapshots(config, accountID, feed.ID, 5);
      }
      setPricefeeds((prev) => ({ ...prev, [accountID]: { status: "ready", feeds, snapshots } }));
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setPricefeeds((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
    }
  }

  async function loadDatalink(accountID: string) {
    setDatalink((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
    try {
      const [channels, deliveries] = await Promise.all([fetchDatalinkChannels(config, accountID), fetchDatalinkDeliveries(config, accountID)]);
      setDatalink((prev) => ({ ...prev, [accountID]: { status: "ready", channels, deliveries } }));
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setDatalink((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
    }
  }

  async function loadDatastreams(accountID: string) {
    setDatastreams((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
    try {
      const streams = await fetchDatastreams(config, accountID);
      const frames: Record<string, DatastreamFrame[]> = {};
      for (const stream of streams) {
        frames[stream.ID] = await fetchDatastreamFrames(config, accountID, stream.ID, 5);
      }
      setDatastreams((prev) => ({ ...prev, [accountID]: { status: "ready", streams, frames } }));
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setDatastreams((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
    }
  }

  async function loadDTA(accountID: string) {
    setDTA((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
    try {
      const [products, orders] = await Promise.all([fetchDTAProducts(config, accountID), fetchDTAOrders(config, accountID)]);
      setDTA((prev) => ({ ...prev, [accountID]: { status: "ready", products, orders } }));
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setDTA((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
    }
  }

  async function loadGasbank(accountID: string) {
    setGasbank((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
    try {
      const [summary, accounts, deadletters] = await Promise.all([
        fetchGasbankSummary(config, accountID),
        fetchGasAccounts(config, accountID),
        fetchGasDeadLetters(config, accountID, 10),
      ]);
      let transactions: GasTransaction[] = [];
      let withdrawals: GasTransaction[] = [];
      let attempts: Record<string, GasbankSettlementAttempt[]> = {};
      const primaryAccountID = accounts[0]?.ID;
      if (primaryAccountID) {
        [transactions, withdrawals] = await Promise.all([
          fetchGasTransactions(config, accountID, primaryAccountID, 20),
          fetchGasWithdrawals(config, accountID, primaryAccountID, undefined, 15),
        ]);
        if (withdrawals.length) {
          const fetched = await Promise.all(
            withdrawals.map(async (w) => {
              try {
                const rows = await fetchWithdrawalAttempts(config, accountID, w.ID, 5);
                return { id: w.ID, attempts: rows };
              } catch {
                return { id: w.ID, attempts: [] };
              }
            }),
          );
          attempts = fetched.reduce<Record<string, GasbankSettlementAttempt[]>>((acc, curr) => {
            acc[curr.id] = curr.attempts;
            return acc;
          }, {});
        }
      }
      setGasbank((prev) => ({
        ...prev,
        [accountID]: { status: "ready", summary, accounts, transactions, withdrawals, deadletters, attempts },
      }));
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setGasbank((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
    }
  }

  async function loadConf(accountID: string) {
    setConf((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
    try {
      const enclaves = await fetchEnclaves(config, accountID);
      setConf((prev) => ({ ...prev, [accountID]: { status: "ready", enclaves } }));
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setConf((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
    }
  }

  async function loadCRE(accountID: string) {
    setCRE((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
    try {
      const [executors, playbooks, runs] = await Promise.all([
        fetchCREExecutors(config, accountID),
        fetchCREPlaybooks(config, accountID),
        fetchCRERuns(config, accountID),
      ]);
      setCRE((prev) => ({ ...prev, [accountID]: { status: "ready", executors, playbooks, runs } }));
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setCRE((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
    }
  }

  async function loadAutomation(accountID: string) {
    setAutomation((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
    try {
      const [jobs, triggers] = await Promise.all([fetchAutomationJobs(config, accountID), fetchTriggers(config, accountID)]);
      setAutomation((prev) => ({ ...prev, [accountID]: { status: "ready", jobs, triggers } }));
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setAutomation((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
    }
  }

  async function loadSecrets(accountID: string) {
    setSecrets((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
    try {
      const items = await fetchSecrets(config, accountID);
      setSecrets((prev) => ({ ...prev, [accountID]: { status: "ready", items } }));
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setSecrets((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
    }
  }

  async function loadFunctions(accountID: string) {
    setFunctionsState((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
    try {
      const funcs = await fetchFunctions(config, accountID);
      const enriched = await Promise.all(
        funcs.map(async (fn) => {
          const executions = await fetchFunctionExecutions(config, accountID, fn.ID, 5);
          return { fn, executions };
        }),
      );
      setFunctionsState((prev) => ({ ...prev, [accountID]: { status: "ready", items: enriched } }));
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setFunctionsState((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
    }
  }

  async function loadOracle(accountID: string) {
    setOracle((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
    try {
      const [sources, requests] = await Promise.all([fetchOracleSources(config, accountID), fetchOracleRequests(config, accountID)]);
      setOracle((prev) => ({ ...prev, [accountID]: { status: "ready", sources, requests } }));
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setOracle((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
    }
  }

  async function loadRandom(accountID: string) {
    setRandom((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
    try {
      const requests = await fetchRandomRequests(config, accountID);
      setRandom((prev) => ({ ...prev, [accountID]: { status: "ready", requests } }));
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setRandom((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
    }
  }

  return (
    <div className="app">
      <header className="hero">
        <div>
          <p className="eyebrow">Neo N3 Service Layer</p>
          <h1>Dashboard bootstrap</h1>
          <p className="muted">Configure the API endpoint and token, then explore system descriptors to toggle feature-aware views.</p>
        </div>
      </header>

      <section className="card">
        <form className="settings" onSubmit={onSubmit}>
          <label>
            API Base URL
            <input value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} placeholder="http://localhost:8080" />
          </label>
          <label>
            API Token
            <input value={token} onChange={(e) => setToken(e.target.value)} placeholder="Bearer token" />
          </label>
          <label>
            Prometheus URL
            <input value={promBase} onChange={(e) => setPromBase(e.target.value)} placeholder="http://localhost:9090" />
          </label>
          <button type="submit" disabled={!canQuery || state.status === "loading"}>
            {state.status === "loading" ? "Loading..." : "Connect"}
          </button>
        </form>
        {state.status === "error" && <p className="error">Failed to load: {state.message}</p>}
        {state.status === "ready" && (
          <>
            <div className="grid">
              <div className="card inner">
                <h3>System</h3>
                <p>
                  Version: <strong>{state.version ?? "unknown"}</strong>
                </p>
                <p>
                  Base URL: <code>{config.baseUrl}</code>
                </p>
                {promConfig.prometheusBaseUrl && (
                  <p>
                    Prometheus: <code>{promConfig.prometheusBaseUrl}</code>
                  </p>
                )}
              </div>
              <div className="card inner">
                <h3>Descriptors ({state.descriptors.length})</h3>
                <ul className="list">
                  {state.descriptors.map((d) => (
                    <li key={`${d.domain}:${d.name}`}>
                      <div className="row">
                        <div>
                          <strong>{d.name}</strong> <span className="tag">{d.domain}</span> <span className="tag subdued">{d.layer}</span>
                        </div>
                        {d.capabilities && <span className="cap">{d.capabilities.join(", ")}</span>}
                      </div>
                    </li>
                  ))}
                </ul>
              </div>
              {state.metrics && (
                <div className="card inner">
                  <h3>HTTP RPS (5m)</h3>
                  <ul className="list">
                    {state.metrics.rps?.map((m) => (
                      <li key={`${m.metric.status}`}>
                        <div className="row">
                          <span className="tag subdued">Status {m.metric.status ?? "all"}</span>
                          <strong>{Number(m.value[1]).toFixed(3)}</strong>
                        </div>
                      </li>
                    ))}
                  </ul>
                </div>
              )}
              {state.metrics?.duration && state.metrics.duration.length > 0 && (
                <div className="card inner">
                  <h3>HTTP p90 latency (past 30m)</h3>
                  <ul className="list">
                    {state.metrics.duration.map((ts, idx) => {
                      const latest = ts.values[ts.values.length - 1];
                      return (
                        <li key={idx}>
                          <div className="row">
                            <span className="tag subdued">p90</span>
                            <strong>{Number(latest[1]).toFixed(3)}s</strong>
                          </div>
                        </li>
                      );
                    })}
                  </ul>
                  <Chart
                    label="p90 latency"
                    data={state.metrics.duration[0].values.map(([x, y]) => ({ x, y: Number(y) }))}
                    color="#0f766e"
                    height={220}
                  />
                </div>
              )}
            </div>
            <div className="card inner accounts">
              <h3>Accounts ({state.accounts.length})</h3>
              {state.accounts.length === 0 && <p className="muted">No accounts found.</p>}
              <ul className="list">
                {state.accounts.map((acct) => {
                  const walletState = wallets[acct.ID] ?? { status: "idle" };
                  const vrfState = vrf[acct.ID] ?? { status: "idle" };
                  const ccipState = ccip[acct.ID] ?? { status: "idle" };
                  const pricefeedState = pricefeeds[acct.ID] ?? { status: "idle" };
                  const gasbankState = gasbank[acct.ID] ?? { status: "idle" };
                  return (
                    <li key={acct.ID} className="account">
                      <div className="row">
                        <div>
                          <strong>{acct.Owner || "Unlabelled"}</strong>
                          <div className="muted mono">{acct.ID}</div>
                        </div>
                      <div className="row gap">
                        {walletState.status === "ready" && <span className="tag">{walletState.items.length} wallets</span>}
                        <button type="button" onClick={() => loadWallets(acct.ID)} disabled={walletState.status === "loading"}>
                          {walletState.status === "loading" ? "Loading..." : "Load wallets"}
                        </button>
                        <button type="button" onClick={() => loadVRF(acct.ID)} disabled={vrfState.status === "loading"}>
                          {vrfState.status === "loading" ? "Loading VRF..." : "VRF"}
                        </button>
                          <button type="button" onClick={() => loadCCIP(acct.ID)} disabled={ccipState.status === "loading"}>
                            {ccipState.status === "loading" ? "Loading CCIP..." : "CCIP"}
                          </button>
                          <button type="button" onClick={() => loadDatafeeds(acct.ID)} disabled={datafeeds[acct.ID]?.status === "loading"}>
                            {datafeeds[acct.ID]?.status === "loading" ? "Loading feeds..." : "Datafeeds"}
                          </button>
                          <button type="button" onClick={() => loadPricefeeds(acct.ID)} disabled={pricefeedState.status === "loading"}>
                            {pricefeedState.status === "loading" ? "Loading price feeds..." : "Price feeds"}
                          </button>
                          <button type="button" onClick={() => loadDatalink(acct.ID)} disabled={datalink[acct.ID]?.status === "loading"}>
                            {datalink[acct.ID]?.status === "loading" ? "Loading link..." : "Datalink"}
                          </button>
                          <button type="button" onClick={() => loadDatastreams(acct.ID)} disabled={datastreams[acct.ID]?.status === "loading"}>
                            {datastreams[acct.ID]?.status === "loading" ? "Loading streams..." : "Datastreams"}
                          </button>
                          <button type="button" onClick={() => loadDTA(acct.ID)} disabled={dta[acct.ID]?.status === "loading"}>
                            {dta[acct.ID]?.status === "loading" ? "Loading DTA..." : "DTA"}
                          </button>
                          <button type="button" onClick={() => loadGasbank(acct.ID)} disabled={gasbankState.status === "loading"}>
                            {gasbankState.status === "loading" ? "Loading gasbank..." : "Gasbank"}
                          </button>
                          <button type="button" onClick={() => loadConf(acct.ID)} disabled={conf[acct.ID]?.status === "loading"}>
                            {conf[acct.ID]?.status === "loading" ? "Loading TEE..." : "Confidential"}
                          </button>
                          <button type="button" onClick={() => loadCRE(acct.ID)} disabled={cre[acct.ID]?.status === "loading"}>
                            {cre[acct.ID]?.status === "loading" ? "Loading CRE..." : "CRE"}
                          </button>
                          <button type="button" onClick={() => loadAutomation(acct.ID)} disabled={automation[acct.ID]?.status === "loading"}>
                            {automation[acct.ID]?.status === "loading" ? "Loading automation..." : "Automation"}
                          </button>
                          <button type="button" onClick={() => loadSecrets(acct.ID)} disabled={secretState.status === "loading"}>
                            {secretState.status === "loading" ? "Loading secrets..." : "Secrets"}
                          </button>
                          <button type="button" onClick={() => loadFunctions(acct.ID)} disabled={funcState.status === "loading"}>
                            {funcState.status === "loading" ? "Loading functions..." : "Functions"}
                          </button>
                          <button type="button" onClick={() => loadOracle(acct.ID)} disabled={oracleState.status === "loading"}>
                            {oracleState.status === "loading" ? "Loading oracle..." : "Oracle"}
                          </button>
                          <button type="button" onClick={() => loadRandom(acct.ID)} disabled={randomState.status === "loading"}>
                            {randomState.status === "loading" ? "Loading randomness..." : "Randomness"}
                          </button>
                        </div>
                      </div>
                    {walletState.status === "error" && <p className="error">Wallets: {walletState.message}</p>}
                    {walletState.status === "ready" && walletState.items.length > 0 && (
                      <ul className="wallets">
                          {walletState.items.map((w) => (
                            <li key={w.ID}>
                              <div className="row">
                                <div className="mono">{w.WalletAddress || w.ID}</div>
                                <span className="tag subdued">{w.Status || "unknown"}</span>
                              </div>
                            </li>
                          ))}
                        </ul>
                      )}
                      {vrfState.status === "error" && <p className="error">VRF: {vrfState.message}</p>}
                      {vrfState.status === "ready" && (
                        <div className="vrf">
                          <div className="row">
                            <h4 className="tight">VRF Keys</h4>
                            <span className="tag subdued">{vrfState.keys.length}</span>
                          </div>
                          <ul className="wallets">
                            {vrfState.keys.map((k) => (
                              <li key={k.ID}>
                                <div className="row">
                                  <div>
                                    <div className="mono">{k.PublicKey}</div>
                                    {k.WalletAddress && <div className="muted mono">{k.WalletAddress}</div>}
                                  </div>
                                  <span className="tag subdued">{k.Status || "unknown"}</span>
                                </div>
                              </li>
                            ))}
                          </ul>
                          <div className="row">
                            <h4 className="tight">VRF Requests</h4>
                            <span className="tag subdued">{vrfState.requests.length}</span>
                          </div>
                          <ul className="wallets">
                            {vrfState.requests.map((r) => (
                              <li key={r.ID}>
                                <div className="row">
                                  <div className="mono">{r.ID}</div>
                                  <span className="tag subdued">{r.Status}</span>
                                </div>
                                <div className="muted mono">{r.Consumer}</div>
                              </li>
                            ))}
                          </ul>
                        </div>
                      )}
                      {ccipState.status === "error" && <p className="error">CCIP: {ccipState.message}</p>}
                     {ccipState.status === "ready" && (
                        <div className="vrf">
                          <div className="row">
                            <h4 className="tight">CCIP Lanes</h4>
                            <span className="tag subdued">{ccipState.lanes.length}</span>
                          </div>
                          <ul className="wallets">
                            {ccipState.lanes.map((lane) => (
                              <li key={lane.ID}>
                                <div className="row">
                                  <div>
                                    <strong>{lane.Name}</strong>
                                    <div className="muted mono">
                                      {lane.SourceChain} → {lane.DestChain}
                                    </div>
                                  </div>
                                  {lane.Tags && lane.Tags.length > 0 && <span className="tag subdued">{lane.Tags.join(", ")}</span>}
                                </div>
                              </li>
                            ))}
                          </ul>
                          <div className="row">
                            <h4 className="tight">CCIP Messages</h4>
                            <span className="tag subdued">{ccipState.messages.length}</span>
                          </div>
                          <ul className="wallets">
                            {ccipState.messages.map((msg) => (
                              <li key={msg.ID}>
                                <div className="row">
                                  <div className="mono">{msg.ID}</div>
                                  <span className="tag subdued">{msg.Status}</span>
                                </div>
                                {msg.LaneID && <div className="muted mono">Lane: {msg.LaneID}</div>}
                              </li>
                            ))}
                          </ul>
                        </div>
                      )}
                      {datafeeds[acct.ID]?.status === "error" && <p className="error">Datafeeds: {datafeeds[acct.ID]?.message}</p>}
                      {datafeeds[acct.ID]?.status === "ready" && (
                        <div className="vrf">
                          <div className="row">
                            <h4 className="tight">Datafeeds</h4>
                            <span className="tag subdued">{datafeeds[acct.ID]?.feeds.length ?? 0}</span>
                          </div>
                          <ul className="wallets">
                            {datafeeds[acct.ID]?.feeds.map((f) => (
                              <li key={f.ID}>
                                <div className="row">
                                  <div>
                                    <strong>{f.Pair}</strong>
                                    {f.SignerSet && f.SignerSet.length > 0 && <div className="muted mono">Signers: {f.SignerSet.join(", ")}</div>}
                                  </div>
                                  <span className="tag subdued">{f.Decimals} dp</span>
                                </div>
                                {datafeeds[acct.ID]?.updates[f.ID]?.length ? (
                                  <div className="muted mono">
                                    Latest: {datafeeds[acct.ID]?.updates[f.ID][0]?.Price} @ round {datafeeds[acct.ID]?.updates[f.ID][0]?.RoundID}
                                  </div>
                                ) : (
                                  <div className="muted">No updates yet.</div>
                                )}
                              </li>
                            ))}
                          </ul>
                        </div>
                      )}
                      {pricefeedState.status === "error" && <p className="error">Price feeds: {pricefeedState.message}</p>}
                      {pricefeedState.status === "ready" && (
                        <div className="vrf">
                          <div className="row">
                            <h4 className="tight">Price feeds</h4>
                            <span className="tag subdued">{pricefeedState.feeds.length}</span>
                          </div>
                          <ul className="wallets">
                            {pricefeedState.feeds.map((feed) => {
                              const snapshotsForFeed = pricefeedState.snapshots[feed.ID] ?? [];
                              const latest = snapshotsForFeed[0];
                              const latestTimestamp = latest?.CollectedAt || latest?.CreatedAt;
                              const formattedTs = latestTimestamp ? new Date(latestTimestamp).toLocaleString() : undefined;
                              const deviation = Number.isFinite(feed.DeviationPercent) ? feed.DeviationPercent.toFixed(2) : "n/a";
                              const pairLabel = feed.Pair || `${feed.BaseAsset}/${feed.QuoteAsset}`;
                              return (
                                <li key={feed.ID}>
                                  <div className="row">
                                    <div>
                                      <strong>{pairLabel}</strong>
                                      <div className="muted mono">
                                        Update {feed.UpdateInterval || "n/a"} • Heartbeat {feed.Heartbeat || "n/a"} • Δ {deviation}%
                                      </div>
                                    </div>
                                    <span className={`tag ${feed.Active ? "" : "subdued"}`}>{feed.Active ? "active" : "paused"}</span>
                                  </div>
                                  {latest ? (
                                    <div className="muted mono">
                                      Latest {latest.Price} via {latest.Source || "unknown"}
                                      {formattedTs ? ` @ ${formattedTs}` : ""}
                                    </div>
                                  ) : (
                                    <div className="muted mono">No snapshots recorded</div>
                                  )}
                                </li>
                              );
                            })}
                          </ul>
                        </div>
                      )}
                      {datalink[acct.ID]?.status === "error" && <p className="error">Datalink: {datalink[acct.ID]?.message}</p>}
                      {datalink[acct.ID]?.status === "ready" && (
                        <div className="vrf">
                          <div className="row">
                            <h4 className="tight">Datalink Channels</h4>
                            <span className="tag subdued">{datalink[acct.ID]?.channels.length ?? 0}</span>
                          </div>
                          <ul className="wallets">
                            {datalink[acct.ID]?.channels.map((c) => (
                              <li key={c.ID}>
                                <div className="row">
                                  <div>
                                    <strong>{c.Name}</strong>
                                    <div className="muted mono">{c.Endpoint}</div>
                                  </div>
                                  {c.SignerSet && c.SignerSet.length > 0 && <span className="tag subdued">{c.SignerSet.length} signers</span>}
                                </div>
                              </li>
                            ))}
                          </ul>
                          <div className="row">
                            <h4 className="tight">Datalink Deliveries</h4>
                            <span className="tag subdued">{datalink[acct.ID]?.deliveries.length ?? 0}</span>
                          </div>
                          <ul className="wallets">
                            {datalink[acct.ID]?.deliveries.map((d) => (
                              <li key={d.ID}>
                                <div className="row">
                                  <div className="mono">{d.ID}</div>
                                  <span className="tag subdued">{d.Status}</span>
                                </div>
                                {d.ChannelID && <div className="muted mono">Channel: {d.ChannelID}</div>}
                              </li>
                            ))}
                          </ul>
                        </div>
                      )}
                      {datastreams[acct.ID]?.status === "error" && <p className="error">Datastreams: {datastreams[acct.ID]?.message}</p>}
                      {datastreams[acct.ID]?.status === "ready" && (
                        <div className="vrf">
                          <div className="row">
                            <h4 className="tight">Datastreams</h4>
                            <span className="tag subdued">{datastreams[acct.ID]?.streams.length ?? 0}</span>
                          </div>
                          <ul className="wallets">
                            {datastreams[acct.ID]?.streams.map((s) => (
                              <li key={s.ID}>
                                <div className="row">
                                  <div>
                                    <strong>{s.Name}</strong> <span className="muted">{s.Frequency}</span>
                                    <div className="muted mono">{s.Symbol}</div>
                                  </div>
                                  <span className="tag subdued">{s.Status || "unknown"}</span>
                                </div>
                                {datastreams[acct.ID]?.frames[s.ID]?.length ? (
                                  <div className="muted mono">
                                    Latest seq {datastreams[acct.ID]?.frames[s.ID][0]?.Sequence} — latency{" "}
                                    {datastreams[acct.ID]?.frames[s.ID][0]?.LatencyMs ?? "n/a"}ms
                                  </div>
                                ) : (
                                  <div className="muted">No frames yet.</div>
                                )}
                              </li>
                            ))}
                          </ul>
                        </div>
                      )}
                      {dta[acct.ID]?.status === "error" && <p className="error">DTA: {dta[acct.ID]?.message}</p>}
                      {dta[acct.ID]?.status === "ready" && (
                        <div className="vrf">
                          <div className="row">
                            <h4 className="tight">DTA Products</h4>
                            <span className="tag subdued">{dta[acct.ID]?.products.length ?? 0}</span>
                          </div>
                          <ul className="wallets">
                            {dta[acct.ID]?.products.map((p) => (
                              <li key={p.ID}>
                                <div className="row">
                                  <div>
                                    <strong>{p.Name}</strong>
                                    <div className="muted mono">
                                      {p.Symbol} • {p.Type}
                                    </div>
                                  </div>
                                  <span className="tag subdued">{p.Status || "unknown"}</span>
                                </div>
                                {p.SettlementTerms && <div className="muted mono">{p.SettlementTerms}</div>}
                              </li>
                            ))}
                          </ul>
                          <div className="row">
                            <h4 className="tight">DTA Orders</h4>
                            <span className="tag subdued">{dta[acct.ID]?.orders.length ?? 0}</span>
                          </div>
                          <ul className="wallets">
                            {dta[acct.ID]?.orders.map((o) => (
                              <li key={o.ID}>
                                <div className="row">
                                  <div className="mono">{o.ID}</div>
                                  <span className="tag subdued">{o.Status || "unknown"}</span>
                                </div>
                                <div className="muted mono">
                                  {o.Type} {o.Amount} @ {o.WalletAddress}
                                </div>
                              </li>
                            ))}
                          </ul>
                        </div>
                      )}
                      {gasbankState.status === "error" && <p className="error">Gasbank: {gasbankState.message}</p>}
                      {gasbankState.status === "ready" && (
                        <div className="card inner gasbank-panel">
                          <div className="section-header">
                            <div>
                              <h4 className="tight">Gasbank Overview</h4>
                              <p className="muted">Updated {formatTimestamp(gasbankState.summary.generated_at)}</p>
                            </div>
                          </div>
                          <div className="metrics-grid">
                            <div className="metric-card">
                              <p>Total Balance</p>
                              <strong>{formatAmount(gasbankState.summary.total_balance)}</strong>
                            </div>
                            <div className="metric-card">
                              <p>Available</p>
                              <strong>{formatAmount(gasbankState.summary.total_available)}</strong>
                            </div>
                            <div className="metric-card">
                              <p>Locked</p>
                              <strong>{formatAmount(gasbankState.summary.total_locked)}</strong>
                            </div>
                            <div className="metric-card">
                              <p>Pending Withdrawals</p>
                              <strong>{gasbankState.summary.pending_withdrawals}</strong>
                              <span className="muted">({formatAmount(gasbankState.summary.pending_amount)})</span>
                            </div>
                          </div>
                          <div className="timeline">
                            <div>
                              <p className="muted">Last Deposit</p>
                              {gasbankState.summary.last_deposit ? (
                                <>
                                  <div className="mono">{gasbankState.summary.last_deposit.id}</div>
                                  <div className="muted mono">
                                    {formatAmount(gasbankState.summary.last_deposit.amount)} →{" "}
                                    {gasbankState.summary.last_deposit.to_address ?? "n/a"}
                                  </div>
                                </>
                              ) : (
                                <div className="muted">No deposits recorded.</div>
                              )}
                            </div>
                            <div>
                              <p className="muted">Last Withdrawal</p>
                              {gasbankState.summary.last_withdrawal ? (
                                <>
                                  <div className="mono">{gasbankState.summary.last_withdrawal.id}</div>
                                  <div className="muted mono">
                                    {formatAmount(gasbankState.summary.last_withdrawal.amount)} →{" "}
                                    {gasbankState.summary.last_withdrawal.to_address ?? "n/a"}
                                  </div>
                                </>
                              ) : (
                                <div className="muted">No withdrawals recorded.</div>
                              )}
                            </div>
                          </div>
                          <div className="section">
                            <div className="row">
                              <h5 className="tight">Accounts</h5>
                              <span className="tag subdued">{gasbankState.accounts.length}</span>
                            </div>
                            {gasbankState.accounts.length ? (
                              <table className="data-table">
                                <thead>
                                  <tr>
                                    <th>Wallet</th>
                                    <th>Available</th>
                                    <th>Pending</th>
                                    <th>Locked</th>
                                  </tr>
                                </thead>
                                <tbody>
                                  {gasbankState.accounts.map((account) => (
                                    <tr key={account.ID}>
                                      <td className="mono">{account.WalletAddress || account.ID}</td>
                                      <td>{formatAmount(account.Available)}</td>
                                      <td>{formatAmount(account.Pending)}</td>
                                      <td>{formatAmount(account.Locked)}</td>
                                    </tr>
                                  ))}
                                </tbody>
                              </table>
                            ) : (
                              <p className="muted">No gas accounts yet.</p>
                            )}
                          </div>
                          <div className="two-column">
                            <div>
                              <div className="row">
                                <h5 className="tight">Recent Transactions</h5>
                                <span className="tag subdued">{gasbankState.transactions.length}</span>
                              </div>
                              {gasbankState.transactions.length ? (
                                <table className="data-table">
                                  <thead>
                                    <tr>
                                      <th>ID</th>
                                      <th>Amount</th>
                                      <th>Status</th>
                                      <th>To</th>
                                    </tr>
                                  </thead>
                                  <tbody>
                                    {gasbankState.transactions.map((tx) => (
                                      <tr key={tx.ID}>
                                        <td className="mono">{tx.ID}</td>
                                        <td>{formatAmount(tx.Amount)}</td>
                                        <td>{tx.Status}</td>
                                        <td>{tx.ToAddress || "—"}</td>
                                      </tr>
                                    ))}
                                  </tbody>
                                </table>
                              ) : (
                                <p className="muted">No transactions recorded.</p>
                              )}
                            </div>
                            <div>
                              <div className="row">
                                <h5 className="tight">Recent Withdrawals</h5>
                                <span className="tag subdued">{gasbankState.withdrawals.length}</span>
                              </div>
                              {gasbankState.withdrawals.length ? (
                                <table className="data-table">
                                  <thead>
                                    <tr>
                                      <th>ID</th>
                                      <th>Status</th>
                                      <th>Amount</th>
                                      <th>Created</th>
                                      <th>Attempts</th>
                                    </tr>
                                  </thead>
                                  <tbody>
                                    {gasbankState.withdrawals.map((withdrawal) => (
                                      <tr key={withdrawal.ID}>
                                        <td className="mono">{withdrawal.ID}</td>
                                        <td>{withdrawal.Status}</td>
                                        <td>{formatAmount(withdrawal.Amount)}</td>
                                        <td>{formatTimestamp(withdrawal.CreatedAt)}</td>
                                        <td className="mono">
                                          {(() => {
                                            const attempts = gasbankState.attempts[withdrawal.ID] || [];
                                            if (!attempts.length) return "—";
                                            const latest = attempts[0];
                                            const label = latest.Status || "attempted";
                                            return `${attempts.length} (${label})`;
                                          })()}
                                        </td>
                                      </tr>
                                    ))}
                                  </tbody>
                                </table>
                              ) : (
                                <p className="muted">No pending withdrawals.</p>
                              )}
                            </div>
                          </div>
                          <div className="section">
                            <div className="row">
                              <h5 className="tight">Dead Letters</h5>
                              <span className="tag subdued">{gasbankState.deadletters.length}</span>
                            </div>
                            {gasbankState.deadletters.length ? (
                              <ul className="wallets deadletters">
                                {gasbankState.deadletters.map((entry) => (
                                  <li key={entry.TransactionID}>
                                    <div className="row">
                                      <div className="mono">{entry.TransactionID}</div>
                                      <span className="tag subdued">{entry.Reason}</span>
                                    </div>
                                    <div className="muted mono">
                                      Retries {entry.Retries} • Last error {entry.LastError || "n/a"}
                                    </div>
                                  </li>
                                ))}
                              </ul>
                            ) : (
                              <p className="muted">No dead-letter entries.</p>
                            )}
                          </div>
                        </div>
                      )}
                      {conf[acct.ID]?.status === "error" && <p className="error">Confidential: {conf[acct.ID]?.message}</p>}
                      {conf[acct.ID]?.status === "ready" && (
                        <div className="vrf">
                          <div className="row">
                            <h4 className="tight">Enclaves</h4>
                            <span className="tag subdued">{conf[acct.ID]?.enclaves.length ?? 0}</span>
                          </div>
                          <ul className="wallets">
                            {conf[acct.ID]?.enclaves.map((e) => (
                              <li key={e.ID}>
                                <div className="row">
                                  <div>
                                    <strong>{e.Name}</strong>
                                    <div className="muted mono">{e.Provider}</div>
                                  </div>
                                  <span className="tag subdued">{e.Status || "unknown"}</span>
                                </div>
                              <div className="muted mono">{e.Measurement}</div>
                            </li>
                          ))}
                        </ul>
                      </div>
                    )}
                      {cre[acct.ID]?.status === "error" && <p className="error">CRE: {cre[acct.ID]?.message}</p>}
                      {cre[acct.ID]?.status === "ready" && (
                        <div className="vrf">
                          <div className="row">
                            <h4 className="tight">CRE Executors</h4>
                            <span className="tag subdued">{cre[acct.ID]?.executors.length ?? 0}</span>
                          </div>
                          <ul className="wallets">
                            {cre[acct.ID]?.executors.map((ex) => (
                              <li key={ex.ID}>
                                <div className="row">
                                  <div>
                                    <strong>{ex.Name}</strong>
                                    <div className="muted mono">{ex.Type}</div>
                                  </div>
                                  <div className="muted mono">{ex.Endpoint}</div>
                                </div>
                              </li>
                            ))}
                          </ul>
                          <div className="row">
                            <h4 className="tight">CRE Playbooks</h4>
                            <span className="tag subdued">{cre[acct.ID]?.playbooks.length ?? 0}</span>
                          </div>
                          <ul className="wallets">
                            {cre[acct.ID]?.playbooks.map((pb) => (
                              <li key={pb.ID}>
                                <div className="row">
                                  <div>
                                    <strong>{pb.Name}</strong>
                                    {pb.Description && <div className="muted">{pb.Description}</div>}
                                  </div>
                                  {pb.Tags && pb.Tags.length > 0 && <span className="tag subdued">{pb.Tags.join(", ")}</span>}
                                </div>
                              </li>
                            ))}
                          </ul>
                          <div className="row">
                            <h4 className="tight">CRE Runs</h4>
                            <span className="tag subdued">{cre[acct.ID]?.runs.length ?? 0}</span>
                          </div>
                          <ul className="wallets">
                            {cre[acct.ID]?.runs.map((run) => (
                              <li key={run.ID}>
                                <div className="row">
                                  <div className="mono">{run.ID}</div>
                                  <span className="tag subdued">{run.Status}</span>
                                </div>
                                <div className="muted mono">Playbook: {run.PlaybookID}</div>
                              </li>
                            ))}
                          </ul>
                        </div>
                      )}
                      {automation[acct.ID]?.status === "error" && <p className="error">Automation: {automation[acct.ID]?.message}</p>}
                      {automation[acct.ID]?.status === "ready" && (
                        <div className="vrf">
                          <div className="row">
                            <h4 className="tight">Automation Jobs</h4>
                            <span className="tag subdued">{automation[acct.ID]?.jobs.length ?? 0}</span>
                          </div>
                          <ul className="wallets">
                            {automation[acct.ID]?.jobs.map((job) => (
                              <li key={job.ID}>
                                <div className="row">
                                  <div>
                                    <strong>{job.Name}</strong>
                                    <div className="muted mono">{job.Schedule}</div>
                                  </div>
                                  <span className="tag subdued">{job.Enabled ? "enabled" : "disabled"}</span>
                                </div>
                              </li>
                            ))}
                          </ul>
                          <div className="row">
                            <h4 className="tight">Triggers</h4>
                            <span className="tag subdued">{automation[acct.ID]?.triggers.length ?? 0}</span>
                          </div>
                          <ul className="wallets">
                            {automation[acct.ID]?.triggers.map((tr) => (
                              <li key={tr.ID}>
                                <div className="row">
                                  <div className="mono">{tr.ID}</div>
                                  <span className="tag subdued">{tr.Type}</span>
                                </div>
                                <div className="muted mono">{tr.Rule}</div>
                              </li>
                            ))}
                          </ul>
                        </div>
                      )}
                      {secretState.status === "error" && <p className="error">Secrets: {secretState.message}</p>}
                      {secretState.status === "ready" && (
                        <div className="vrf">
                          <div className="row">
                            <h4 className="tight">Secrets</h4>
                            <span className="tag subdued">{secretState.items.length}</span>
                          </div>
                          <ul className="wallets">
                            {secretState.items.map((sec) => (
                              <li key={sec.ID}>
                                <div className="row">
                                  <div>
                                    <strong>{sec.Name}</strong>
                                    <div className="muted mono">{sec.ID}</div>
                                  </div>
                                  <span className="tag subdued">{new Date(sec.UpdatedAt).toLocaleString()}</span>
                                </div>
                              </li>
                            ))}
                          </ul>
                        </div>
                      )}
                      {funcState.status === "error" && <p className="error">Functions: {funcState.message}</p>}
                      {funcState.status === "ready" && (
                        <div className="vrf">
                          <div className="row">
                            <h4 className="tight">Functions</h4>
                            <span className="tag subdued">{funcState.items.length}</span>
                          </div>
                          <ul className="wallets">
                            {funcState.items.map(({ fn, executions }) => (
                              <li key={fn.ID}>
                                <div className="row">
                                  <div>
                                    <strong>{fn.Name}</strong>
                                    <div className="muted mono">{fn.Runtime}</div>
                                  </div>
                                  {fn.Status && <span className="tag subdued">{fn.Status}</span>}
                                </div>
                                {executions.length > 0 ? (
                                  <ul className="list mono">
                                    {executions.map((ex) => (
                                      <li key={ex.ID} className="row">
                                        <span>{ex.ID}</span>
                                        <span className="tag subdued">{ex.Status}</span>
                                      </li>
                                    ))}
                                  </ul>
                                ) : (
                                  <p className="muted">No executions yet.</p>
                                )}
                              </li>
                            ))}
                          </ul>
                        </div>
                      )}
                      {oracleState.status === "error" && <p className="error">Oracle: {oracleState.message}</p>}
                      {oracleState.status === "ready" && (
                        <div className="vrf">
                          <div className="row">
                            <h4 className="tight">Oracle Sources</h4>
                            <span className="tag subdued">{oracleState.sources.length}</span>
                          </div>
                          <ul className="wallets">
                            {oracleState.sources.map((src) => (
                              <li key={src.ID}>
                                <div className="row">
                                  <div>
                                    <strong>{src.Name}</strong>
                                    <div className="muted mono">{src.URL}</div>
                                  </div>
                                  {src.Status && <span className="tag subdued">{src.Status}</span>}
                                </div>
                              </li>
                            ))}
                          </ul>
                          <div className="row">
                            <h4 className="tight">Oracle Requests</h4>
                            <span className="tag subdued">{oracleState.requests.length}</span>
                          </div>
                          <ul className="wallets">
                            {oracleState.requests.map((req) => (
                              <li key={req.ID}>
                                <div className="row">
                                  <div className="mono">{req.ID}</div>
                                  <span className="tag subdued">{req.Status}</span>
                                </div>
                                <div className="muted mono">Source: {req.SourceID}</div>
                              </li>
                            ))}
                          </ul>
                        </div>
                      )}
                      {randomState.status === "error" && <p className="error">Randomness: {randomState.message}</p>}
                      {randomState.status === "ready" && (
                        <div className="vrf">
                          <div className="row">
                            <h4 className="tight">Random Requests</h4>
                            <span className="tag subdued">{randomState.requests.length}</span>
                          </div>
                          <ul className="wallets">
                            {randomState.requests.map((req) => {
                              const label = req.RequestID && req.RequestID.trim().length > 0 ? req.RequestID : `Counter ${req.Counter}`;
                              const timestamp = req.CreatedAt ? new Date(req.CreatedAt).toLocaleString() : undefined;
                              return (
                                <li key={`${label}-${req.Counter}`}>
                                  <div className="row">
                                    <div>
                                      <div className="mono">{label}</div>
                                      {timestamp && <div className="muted mono">{timestamp}</div>}
                                    </div>
                                    <span className="tag subdued">{req.Length} bytes</span>
                                  </div>
                                  <div className="muted mono">Value: {formatSnippet(req.Value, 28)}</div>
                                  <div className="muted mono">Signature: {formatSnippet(req.Signature, 28)}</div>
                                </li>
                              );
                            })}
                          </ul>
                        </div>
                      )}
                    </li>
                  );
                })}
              </ul>
            </div>
          </>
        )}
        {state.status === "idle" && <p className="muted">Enter a base URL and token to connect.</p>}
      </section>
    </div>
  );
}
