import { Account, WorkspaceWallet } from "../api";
import { AutomationPanel, AutomationState } from "./AutomationPanel";
import { CCIPPanel, CCIPState } from "./CCIPPanel";
import { ConfPanel, ConfState } from "./ConfPanel";
import { CREPanel, CREState } from "./CREPanel";
import { DatafeedsPanel, DatafeedsState } from "./DatafeedsPanel";
import { DatalinkPanel, DatalinkState } from "./DatalinkPanel";
import { DatastreamsPanel, DatastreamsState } from "./DatastreamsPanel";
import { DTAPanel, DTAState } from "./DTAPanel";
import { FunctionsPanel, FunctionsState } from "./FunctionsPanel";
import { GasbankPanel, GasbankState } from "./GasbankPanel";
import { OraclePanel, OracleState } from "./OraclePanel";
import { PricefeedsPanel, PricefeedsState } from "./PricefeedsPanel";
import { RandomPanel, RandomState } from "./RandomPanel";
import { SecretsPanel, SecretsState } from "./SecretsPanel";
import { VRFPanel, VRFState } from "./VRFPanel";

export type WalletState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; items: WorkspaceWallet[] }
  | { status: "error"; message: string };

type Props = {
  account: Account;
  walletState: WalletState;
  vrfState: VRFState;
  ccipState: CCIPState;
  datafeedState?: DatafeedsState;
  pricefeedState: PricefeedsState;
  datalinkState?: DatalinkState;
  datastreamsState?: DatastreamsState;
  dtaState?: DTAState;
  gasbankState: GasbankState;
  confState?: ConfState;
  creState?: CREState;
  automationState?: AutomationState;
  secretState: SecretsState;
  funcState: FunctionsState;
  oracleState?: OracleState;
  randomState?: RandomState;
  oracleBanner?: { tone: "success" | "error"; message: string };
  cursor?: string;
  failedCursor?: string;
  loadingCursor?: boolean;
  loadingFailed?: boolean;
  filter?: string;
  retrying: Record<string, boolean>;
  onFilterChange: (value: string) => void;
  onLoadWallets: () => void;
  onLoadVRF: () => void;
  onLoadCCIP: () => void;
  onLoadDatafeeds: () => void;
  onLoadPricefeeds: () => void;
  onLoadDatalink: () => void;
  onLoadDatastreams: () => void;
  onLoadDTA: () => void;
  onLoadGasbank: () => void;
  onLoadConf: () => void;
  onLoadCRE: () => void;
  onLoadAutomation: () => void;
  onLoadSecrets: () => void;
  onLoadFunctions: () => void;
  onLoadOracle: () => void;
  onLoadRandom: () => void;
  onLoadMoreOracle: () => void;
  onLoadMoreFailed: () => void;
  onRetry: (requestID: string) => void;
  onCopyCursor: (cursor: string) => void;
  onSetAggregation: (feedId: string, aggregation: string) => void;
  onCreateChannel: (payload: { name: string; endpoint: string; signers: string[]; status?: string; metadata?: Record<string, string> }) => void;
  onCreateDelivery: (payload: { channelId: string; body: Record<string, any>; metadata?: Record<string, string> }) => void;
  onNotify: (type: "success" | "error", message: string) => void;
  formatAmount: (value: number | undefined) => string;
  formatTimestamp: (value?: string) => string;
  formatDuration: (value?: number) => string;
  formatSnippet: (value: string, limit?: number) => string;
};

export function AccountCard({
  account,
  walletState,
  vrfState,
  ccipState,
  datafeedState,
  pricefeedState,
  datalinkState,
  datastreamsState,
  dtaState,
  gasbankState,
  confState,
  creState,
  automationState,
  secretState,
  funcState,
  oracleState,
  randomState,
  oracleBanner,
  cursor,
  failedCursor,
  loadingCursor,
  loadingFailed,
  filter,
  retrying,
  onFilterChange,
  onLoadWallets,
  onLoadVRF,
  onLoadCCIP,
  onLoadDatafeeds,
  onLoadPricefeeds,
  onLoadDatalink,
  onLoadDatastreams,
  onLoadDTA,
  onLoadGasbank,
  onLoadConf,
  onLoadCRE,
  onLoadAutomation,
  onLoadSecrets,
  onLoadFunctions,
  onLoadOracle,
  onLoadRandom,
  onLoadMoreOracle,
  onLoadMoreFailed,
  onRetry,
  onCopyCursor,
  onSetAggregation,
  onCreateChannel,
  onCreateDelivery,
  onNotify,
  formatAmount,
  formatTimestamp,
  formatDuration,
  formatSnippet,
}: Props) {
  return (
    <li className="account">
      <div className="row">
        <div>
          <strong>{account.Owner || "Unlabelled"}</strong>
          <div className="muted mono">{account.ID}</div>
        </div>
        <div className="row gap">
          {walletState.status === "ready" && <span className="tag">{walletState.items.length} wallets</span>}
          <button type="button" onClick={onLoadWallets} disabled={walletState.status === "loading"}>
            {walletState.status === "loading" ? "Loading..." : "Load wallets"}
          </button>
          <button type="button" onClick={onLoadVRF} disabled={vrfState.status === "loading"}>
            {vrfState.status === "loading" ? "Loading VRF..." : "VRF"}
          </button>
          <button type="button" onClick={onLoadCCIP} disabled={ccipState.status === "loading"}>
            {ccipState.status === "loading" ? "Loading CCIP..." : "CCIP"}
          </button>
          <button type="button" onClick={onLoadDatafeeds} disabled={datafeedState?.status === "loading"}>
            {datafeedState?.status === "loading" ? "Loading feeds..." : "Datafeeds"}
          </button>
          <button type="button" onClick={onLoadPricefeeds} disabled={pricefeedState.status === "loading"}>
            {pricefeedState.status === "loading" ? "Loading price feeds..." : "Price feeds"}
          </button>
          <button type="button" onClick={onLoadDatalink} disabled={datalinkState?.status === "loading"}>
            {datalinkState?.status === "loading" ? "Loading link..." : "Datalink"}
          </button>
          <button type="button" onClick={onLoadDatastreams} disabled={datastreamsState?.status === "loading"}>
            {datastreamsState?.status === "loading" ? "Loading streams..." : "Datastreams"}
          </button>
          <button type="button" onClick={onLoadDTA} disabled={dtaState?.status === "loading"}>
            {dtaState?.status === "loading" ? "Loading DTA..." : "DTA"}
          </button>
          <button type="button" onClick={onLoadGasbank} disabled={gasbankState.status === "loading"}>
            {gasbankState.status === "loading" ? "Loading gasbank..." : "Gasbank"}
          </button>
          <button type="button" onClick={onLoadConf} disabled={confState?.status === "loading"}>
            {confState?.status === "loading" ? "Loading TEE..." : "Confidential"}
          </button>
          <button type="button" onClick={onLoadCRE} disabled={creState?.status === "loading"}>
            {creState?.status === "loading" ? "Loading CRE..." : "CRE"}
          </button>
          <button type="button" onClick={onLoadAutomation} disabled={automationState?.status === "loading"}>
            {automationState?.status === "loading" ? "Loading automation..." : "Automation"}
          </button>
          <button type="button" onClick={onLoadSecrets} disabled={secretState.status === "loading"}>
            {secretState.status === "loading" ? "Loading secrets..." : "Secrets"}
          </button>
          <button type="button" onClick={onLoadFunctions} disabled={funcState.status === "loading"}>
            {funcState.status === "loading" ? "Loading functions..." : "Functions"}
          </button>
          <button type="button" onClick={onLoadOracle} disabled={oracleState?.status === "loading"}>
            {oracleState?.status === "loading" ? "Loading oracle..." : "Oracle"}
          </button>
          <button type="button" onClick={onLoadRandom} disabled={randomState?.status === "loading"}>
            {randomState?.status === "loading" ? "Loading randomness..." : "Randomness"}
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
      <VRFPanel vrfState={vrfState} />
      <CCIPPanel ccipState={ccipState} />
      <DatafeedsPanel
        datafeedState={datafeedState}
        formatDuration={formatDuration}
        onUpdateAggregation={(feedId, agg) => onSetAggregation(feedId, agg)}
        onNotify={onNotify}
      />
      <PricefeedsPanel pricefeedState={pricefeedState} formatTimestamp={formatTimestamp} />
      <DatalinkPanel
        datalinkState={datalinkState}
        onCreateChannel={(payload) => onCreateChannel(payload)}
        onCreateDelivery={(payload) => onCreateDelivery(payload)}
        onNotify={onNotify}
      />
      <DatastreamsPanel datastreamsState={datastreamsState} />
      <DTAPanel dtaState={dtaState} />
      <GasbankPanel gasbankState={gasbankState} formatAmount={formatAmount} formatTimestamp={formatTimestamp} />
      <ConfPanel confState={confState} />
      <CREPanel creState={creState} />
      <AutomationPanel automationState={automationState} />
      <SecretsPanel secretState={secretState} formatTimestamp={formatTimestamp} />
      <FunctionsPanel functionsState={funcState} />
      <OraclePanel
        accountID={account.ID}
        oracleState={oracleState}
        banner={oracleBanner}
        cursor={cursor}
        failedCursor={failedCursor}
        loadingCursor={loadingCursor}
        loadingFailed={loadingFailed}
        filter={filter}
        onFilterChange={onFilterChange}
        onReload={onLoadOracle}
        onLoadMore={onLoadMoreOracle}
        onLoadMoreFailed={onLoadMoreFailed}
        onRetry={onRetry}
        onCopyCursor={onCopyCursor}
        retrying={retrying}
        formatSnippet={formatSnippet}
        formatTimestamp={formatTimestamp}
        formatDuration={formatDuration}
      />
      <RandomPanel randomState={randomState} formatSnippet={formatSnippet} formatTimestamp={formatTimestamp} />
    </li>
  );
}
