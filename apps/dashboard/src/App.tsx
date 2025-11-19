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
  | { status: "ready"; accounts: GasAccount[]; transactions: GasTransaction[] }
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

  const canQuery = config.baseUrl.length > 0 && config.token.length > 0;

  async function load() {
    if (!canQuery) {
      setState({ status: "idle" });
      return;
    }
    setState({ status: "loading" });
    try {
      const [health, descriptors, accounts] = await Promise.all([fetchHealth(config), fetchDescriptors(config), fetchAccounts(config)]);
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
      setState({ status: "ready", descriptors, accounts, version: health.version, metrics });
      setWallets({});
      setVRF({});
      setCCIP({});
      setDatafeeds({});
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
      const accounts = await fetchGasAccounts(config, accountID);
      const transactions = await fetchGasTransactions(config, accountID, undefined, 20);
      setGasbank((prev) => ({ ...prev, [accountID]: { status: "ready", accounts, transactions } }));
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
                          <button type="button" onClick={() => loadDatalink(acct.ID)} disabled={datalink[acct.ID]?.status === "loading"}>
                            {datalink[acct.ID]?.status === "loading" ? "Loading link..." : "Datalink"}
                          </button>
                          <button type="button" onClick={() => loadDatastreams(acct.ID)} disabled={datastreams[acct.ID]?.status === "loading"}>
                            {datastreams[acct.ID]?.status === "loading" ? "Loading streams..." : "Datastreams"}
                          </button>
                          <button type="button" onClick={() => loadDTA(acct.ID)} disabled={dta[acct.ID]?.status === "loading"}>
                            {dta[acct.ID]?.status === "loading" ? "Loading DTA..." : "DTA"}
                          </button>
                          <button type="button" onClick={() => loadGasbank(acct.ID)} disabled={gasbank[acct.ID]?.status === "loading"}>
                            {gasbank[acct.ID]?.status === "loading" ? "Loading gasbank..." : "Gasbank"}
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
                      {gasbank[acct.ID]?.status === "error" && <p className="error">Gasbank: {gasbank[acct.ID]?.message}</p>}
                      {gasbank[acct.ID]?.status === "ready" && (
                        <div className="vrf">
                          <div className="row">
                            <h4 className="tight">Gas Accounts</h4>
                            <span className="tag subdued">{gasbank[acct.ID]?.accounts.length ?? 0}</span>
                          </div>
                          <ul className="wallets">
                            {gasbank[acct.ID]?.accounts.map((g) => (
                              <li key={g.ID}>
                                <div className="row">
                                  <div className="mono">{g.WalletAddress}</div>
                                  <span className="tag subdued">Avail {g.Available.toFixed(3)}</span>
                                </div>
                                <div className="muted mono">
                                  Pending {g.Pending.toFixed(3)} • Locked {g.Locked.toFixed(3)}
                                </div>
                              </li>
                            ))}
                          </ul>
                          <div className="row">
                            <h4 className="tight">Gas Transactions</h4>
                            <span className="tag subdued">{gasbank[acct.ID]?.transactions.length ?? 0}</span>
                          </div>
                          <ul className="wallets">
                            {gasbank[acct.ID]?.transactions.map((t) => (
                              <li key={t.ID}>
                                <div className="row">
                                  <div className="mono">{t.ID}</div>
                                  <span className="tag subdued">{t.Status}</span>
                                </div>
                                <div className="muted mono">
                                  {t.Amount} {t.FromAddress} → {t.ToAddress}
                                </div>
                              </li>
                            ))}
                          </ul>
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
                            {randomState.requests.map((req) => (
                              <li key={req.ID}>
                                <div className="row">
                                  <div className="mono">{req.ID}</div>
                                  <span className="tag subdued">{req.Status}</span>
                                </div>
                                <div className="muted mono">Length: {req.Length} bytes</div>
                              </li>
                            ))}
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
