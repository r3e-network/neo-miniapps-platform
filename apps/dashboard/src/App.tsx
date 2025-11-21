import { FormEvent, useEffect, useMemo } from "react";
import { normaliseUrl } from "./api";
import { useLocalStorage } from "./useLocalStorage";
import { MetricsConfig } from "./metrics";
import { AccountsSection, SettingsForm, SystemOverview } from "./components";
import { useAccountsData, useSystemInfo } from "./hooks";
import { formatAmount, formatDuration, formatSnippet, formatTimestamp } from "./utils";

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

  const canQuery = config.baseUrl.length > 0 && config.token.length > 0;

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
        <SettingsForm
          baseUrl={baseUrl}
          token={token}
          promBase={promBase}
          canQuery={canQuery}
          status={state.status}
          onSubmit={onSubmit}
          onBaseUrlChange={setBaseUrl}
          onTokenChange={setToken}
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
              {state.accounts.length === 0 && <p className="muted">No accounts found.</p>}
              <AccountsSection
                accounts={state.accounts}
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
                setFilter={(accountID, value) => setOracleFilters((prev) => ({ ...prev, [accountID]: value }))}
                formatSnippet={formatSnippet}
                formatTimestamp={formatTimestamp}
                formatDuration={formatDuration}
                formatAmount={formatAmount}
              />
            </div>
          </>
        )}
        {state.status === "idle" && <p className="muted">Enter a base URL and token to connect.</p>}
      </section>
    </div>
  );
}
