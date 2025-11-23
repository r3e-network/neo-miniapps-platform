import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { normaliseUrl } from "./api";
import { useLocalStorage } from "./useLocalStorage";
import { MetricsConfig } from "./metrics";
import { AccountsSection, AdminPanel, AuthPanel, JamPanel, Notifications, SettingsForm, SystemOverview, WalletGate } from "./components";
import { useAccountsData, useSystemInfo } from "./hooks";
import { formatAmount, formatDuration, formatSnippet, formatTimestamp } from "./utils";
import type { Notification } from "./components/Notifications";

type WalletSession = { address: string; label?: string; signature?: string };

export function App() {
  const [baseUrl, setBaseUrl] = useLocalStorage("sl-ui.baseUrl", "http://localhost:8080");
  const [token, setToken] = useLocalStorage("sl-ui.token", "dev-token");
  const [tenant, setTenant] = useLocalStorage("sl-ui.tenant", "");
  const [wallet, setWallet] = useState<WalletSession>(() => {
    try {
      const raw = window.localStorage.getItem("sl-ui.wallet");
      return raw ? (JSON.parse(raw) as WalletSession) : { address: "", label: "", signature: "" };
    } catch {
      return { address: "", label: "", signature: "" };
    }
  });
  useEffect(() => {
    window.localStorage.setItem("sl-ui.wallet", JSON.stringify(wallet));
  }, [wallet]);
  const config = useMemo(
    () => ({
      baseUrl: normaliseUrl(baseUrl),
      token: token.trim(),
      tenant: tenant.trim() || undefined,
      wallet,
    }),
    [baseUrl, token, tenant, wallet],
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

  const canQuery = config.baseUrl.length > 0 && config.token.length > 0;
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const qsBase = params.get("api") || params.get("base") || params.get("baseUrl") || params.get("endpoint");
    const qsToken = params.get("token") || params.get("api_token");
    const qsTenant = params.get("tenant");
    if (qsBase) {
      setBaseUrl(normaliseUrl(qsBase));
    }
    if (qsToken) {
      setToken(qsToken);
    }
    if (qsTenant) {
      setTenant(qsTenant);
    }
  }, [setBaseUrl, setTenant, setToken]);

  const {
    wallets,
    vrf,
    ccip,
    datafeeds,
    pricefeeds,
    datalink,
    datastreams,
    dta,
    gasbank,
    conf,
    cre,
    automation,
    secrets,
    functionsState,
    oracle,
    random,
    oracleBanner,
    retryingOracle,
    oracleFilters,
    oracleCursors,
    oracleFailedCursor,
    loadingOraclePage,
    loadingOracleFailedPage,
    setOracleFilters,
    resetAccounts,
    loadWallets,
    loadVRF,
    loadCCIP,
    loadDatafeeds,
    loadPricefeeds,
    loadDatalink,
    loadDatastreams,
    loadDTA,
    loadGasbank,
    loadConf,
    loadCRE,
    loadAutomation,
    loadSecrets,
    loadFunctions,
    loadOracle,
    loadRandom,
    loadMoreOracle,
    loadMoreFailedOracle,
    retryOracle,
    copyCursor,
    setAggregation,
    createChannel,
    createDelivery,
  } = useAccountsData(config);
  const { state, systemVersion, load } = useSystemInfo(config, promConfig, canQuery);

  useEffect(() => {
    resetAccounts();
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [config.baseUrl, config.token]);

  function onSubmit(event: FormEvent) {
    event.preventDefault();
    resetAccounts();
    void load();
  }

  const docsLinks = [
    { label: "Data Feeds Quickstart", href: "https://github.com/R3E-Network/service_layer/blob/master/docs/examples/datafeeds.md" },
    { label: "DataLink Quickstart", href: "https://github.com/R3E-Network/service_layer/blob/master/docs/examples/datalink.md" },
    { label: "JAM Quickstart", href: "https://github.com/R3E-Network/service_layer/blob/master/docs/examples/jam.md" },
    { label: "Dashboard Smoke Checklist", href: "https://github.com/R3E-Network/service_layer/blob/master/docs/dashboard-smoke.md" },
  ];

  const [notifications, setNotifications] = useState<Notification[]>([]);
  const notify = useCallback((type: "success" | "error", message: string) => {
    const id = `${Date.now()}-${Math.random().toString(36).slice(2)}`;
    setNotifications((prev) => [...prev, { id, type, message }]);
    setTimeout(() => {
      setNotifications((prev) => prev.filter((n) => n.id != id));
    }, 4000);
  }, []);
  const dismissNotification = useCallback((id: string) => {
    setNotifications((prev) => prev.filter((n) => n.id !== id));
  }, []);

  const handleSetAggregation = useCallback(
    async (accountID: string, feedID: string, aggregation: string) => {
      const state = datafeeds[accountID];
      if (!state || state.status !== "ready") return;
      const feed = state.feeds.find((f) => f.ID === feedID);
      if (!feed) return;
      await setAggregation(accountID, feed, aggregation);
    },
    [datafeeds, setAggregation],
  );

  return (
    <div className="app">
      <header className="hero">
        <div>
          <p className="eyebrow">Neo N3 Service Layer</p>
          <h1>Dashboard bootstrap</h1>
          <p className="muted">
            Configure the API endpoint and token, then explore system descriptors to toggle feature-aware views. Defaults (local compose):
            API <code>http://localhost:8080</code>, token <code>dev-token</code>, or login via admin/changeme.
          </p>
          <div className="row" style={{ gap: "8px" }}>
            <span className="tag">Tenant: {config.tenant || "none"}</span>
            <span className="tag subdued">Auth: Bearer token</span>
          </div>
        </div>
      </header>

      <section className="card">
        <WalletGate
          wallet={wallet}
          onConnect={(w) => setWallet(w)}
          onDisconnect={() => setWallet({ address: "", label: "", signature: "" })}
        />
        <AuthPanel
          baseUrl={config.baseUrl}
          onLoggedIn={(tok, role) => {
            setToken(tok);
            notify("success", `Logged in${role ? ` (${role})` : ""}`);
          }}
        />
        <SettingsForm
          baseUrl={baseUrl}
          token={token}
          tenant={tenant}
          promBase={promBase}
          canQuery={canQuery}
          status={state.status}
          onSubmit={onSubmit}
          onBaseUrlChange={setBaseUrl}
          onTokenChange={setToken}
          onTenantChange={setTenant}
          onPromChange={setPromBase}
        />
        {state.status === "error" && <p className="error">Failed to load: {state.message}</p>}
        {state.status === "ready" && (
          <>
            <SystemOverview
              descriptors={state.descriptors}
              version={state.version}
              buildVersion={systemVersion}
              baseUrl={config.baseUrl}
              promBase={promConfig.prometheusBaseUrl}
              metrics={state.metrics}
              formatDuration={formatDuration}
            />
            <div className="card inner accounts">
              <h3>Accounts ({state.accounts.length})</h3>
              {state.accounts.some((a) => a.Metadata?.tenant) && !config.tenant && (
                <p className="warning">Tenant-scoped accounts detected. Set Tenant in Settings to avoid 403s.</p>
              )}
              {state.accounts.length === 0 && <p className="muted">No accounts found.</p>}
              <AccountsSection
                accounts={state.accounts}
                activeTenant={config.tenant}
                linkBase={window.location.origin + window.location.pathname}
                wallets={wallets}
                vrf={vrf}
                ccip={ccip}
                datafeeds={datafeeds}
                pricefeeds={pricefeeds}
                datalink={datalink}
                datastreams={datastreams}
                dta={dta}
                gasbank={gasbank}
                conf={conf}
                cre={cre}
                automation={automation}
                secrets={secrets}
                functionsState={functionsState}
                oracle={oracle}
                random={random}
                oracleBanner={oracleBanner}
                oracleCursors={oracleCursors}
                oracleFailedCursor={oracleFailedCursor}
                loadingOraclePage={loadingOraclePage}
                loadingOracleFailedPage={loadingOracleFailedPage}
                oracleFilters={oracleFilters}
                retryingOracle={retryingOracle}
                onLoadWallets={loadWallets}
                onLoadVRF={loadVRF}
                onLoadCCIP={loadCCIP}
                onLoadDatafeeds={loadDatafeeds}
                onLoadPricefeeds={loadPricefeeds}
                onLoadDatalink={loadDatalink}
                onLoadDatastreams={loadDatastreams}
                onLoadDTA={loadDTA}
                onLoadGasbank={loadGasbank}
                onLoadConf={loadConf}
                onLoadCRE={loadCRE}
                onLoadAutomation={loadAutomation}
                onLoadSecrets={loadSecrets}
                onLoadFunctions={loadFunctions}
                onLoadOracle={loadOracle}
                onLoadRandom={loadRandom}
                onLoadMoreOracle={loadMoreOracle}
                onLoadMoreFailedOracle={loadMoreFailedOracle}
                onRetryOracle={retryOracle}
                onCopyCursor={copyCursor}
                onSetAggregation={handleSetAggregation}
                onCreateChannel={(accountID, payload) => createChannel(accountID, payload)}
                onCreateDelivery={(accountID, payload) => createDelivery(accountID, payload.channelId, { body: payload.body, metadata: payload.metadata })}
                onNotify={notify}
                setFilter={(accountID, value) => setOracleFilters((prev) => ({ ...prev, [accountID]: value }))}
                formatSnippet={formatSnippet}
                formatTimestamp={formatTimestamp}
                formatDuration={formatDuration}
                formatAmount={formatAmount}
              />
            </div>
            {canQuery && <JamPanel baseUrl={config.baseUrl} token={config.token} onNotify={notify} />}
          </>
        )}
        {state.status === "idle" && <p className="muted">Enter a base URL and token to connect.</p>}
        <AdminPanel systemState={state} baseUrl={config.baseUrl} token={config.token} tenant={config.tenant} />
      </section>

      <section className="card inner">
        <h3>Docs</h3>
        <p className="muted">Quick references to keep the dashboard in sync with the backend.</p>
        <ul className="wallets">
          {docsLinks.map((link) => (
            <li key={link.href}>
              <a href={link.href} target="_blank" rel="noreferrer">
                {link.label}
              </a>
            </li>
          ))}
        </ul>
      </section>
      <Notifications items={notifications} onDismiss={dismissNotification} />
    </div>
  );
}
