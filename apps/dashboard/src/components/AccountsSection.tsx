import { Account } from "../api";
import { AutomationState } from "./AutomationPanel";
import { AccountCard, WalletState } from "./AccountCard";
import { CCIPState } from "./CCIPPanel";
import { ConfState } from "./ConfPanel";
import { CREState } from "./CREPanel";
import { DatafeedsState } from "./DatafeedsPanel";
import { DatalinkState } from "./DatalinkPanel";
import { DatastreamsState } from "./DatastreamsPanel";
import { DTAState } from "./DTAPanel";
import { FunctionsState } from "./FunctionsPanel";
import { GasbankState } from "./GasbankPanel";
import { OracleState } from "./OraclePanel";
import { PricefeedsState } from "./PricefeedsPanel";
import { RandomState } from "./RandomPanel";
import { SecretsState } from "./SecretsPanel";
import { VRFState } from "./VRFPanel";

type Banner = { tone: "success" | "error"; message: string };

type Props = {
  accounts: Account[];
  wallets: Record<string, WalletState>;
  vrf: Record<string, VRFState>;
  ccip: Record<string, CCIPState>;
  datafeeds: Record<string, DatafeedsState>;
  pricefeeds: Record<string, PricefeedsState>;
  datalink: Record<string, DatalinkState>;
  datastreams: Record<string, DatastreamsState>;
  dta: Record<string, DTAState>;
  gasbank: Record<string, GasbankState>;
  conf: Record<string, ConfState>;
  cre: Record<string, CREState>;
  automation: Record<string, AutomationState>;
  secrets: Record<string, SecretsState>;
  functionsState: Record<string, FunctionsState>;
  oracle: Record<string, OracleState>;
  random: Record<string, RandomState>;
  oracleBanner: Record<string, Banner | undefined>;
  oracleCursors: Record<string, string | undefined>;
  oracleFailedCursor: Record<string, string | undefined>;
  loadingOraclePage: Record<string, boolean>;
  loadingOracleFailedPage: Record<string, boolean>;
  oracleFilters: Record<string, string>;
  retryingOracle: Record<string, boolean>;
  onLoadWallets: (accountID: string) => void;
  onLoadVRF: (accountID: string) => void;
  onLoadCCIP: (accountID: string) => void;
  onLoadDatafeeds: (accountID: string) => void;
  onLoadPricefeeds: (accountID: string) => void;
  onLoadDatalink: (accountID: string) => void;
  onLoadDatastreams: (accountID: string) => void;
  onLoadDTA: (accountID: string) => void;
  onLoadGasbank: (accountID: string) => void;
  onLoadConf: (accountID: string) => void;
  onLoadCRE: (accountID: string) => void;
  onLoadAutomation: (accountID: string) => void;
  onLoadSecrets: (accountID: string) => void;
  onLoadFunctions: (accountID: string) => void;
  onLoadOracle: (accountID: string) => void;
  onLoadRandom: (accountID: string) => void;
  onLoadMoreOracle: (accountID: string) => void;
  onLoadMoreFailedOracle: (accountID: string) => void;
  onRetryOracle: (accountID: string, requestID: string) => void;
  onCopyCursor: (accountID: string, cursor: string) => void;
  onSetAggregation: (accountID: string, feedId: string, aggregation: string) => void;
  onCreateChannel: (accountID: string, payload: { name: string; endpoint: string; signers: string[]; status?: string; metadata?: Record<string, string> }) => void;
  onCreateDelivery: (accountID: string, payload: { channelId: string; body: Record<string, any>; metadata?: Record<string, string> }) => void;
  onNotify: (type: "success" | "error", message: string) => void;
  setFilter: (accountID: string, value: string) => void;
  formatAmount: (value: number | undefined) => string;
  formatTimestamp: (value?: string) => string;
  formatDuration: (value?: number) => string;
  formatSnippet: (value: string, limit?: number) => string;
};

export function AccountsSection({
  accounts,
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
  oracleCursors,
  oracleFailedCursor,
  loadingOraclePage,
  loadingOracleFailedPage,
  oracleFilters,
  retryingOracle,
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
  onLoadMoreFailedOracle,
  onRetryOracle,
  onCopyCursor,
  onSetAggregation,
  onCreateChannel,
  onCreateDelivery,
  onNotify,
  setFilter,
  formatAmount,
  formatTimestamp,
  formatDuration,
  formatSnippet,
}: Props) {
  return (
    <ul className="list">
      {accounts.map((acct) => {
        const walletState = wallets[acct.ID] ?? { status: "idle" };
        const vrfState = vrf[acct.ID] ?? { status: "idle" };
        const ccipState = ccip[acct.ID] ?? { status: "idle" };
        const datafeedState = datafeeds[acct.ID];
        const pricefeedState = pricefeeds[acct.ID] ?? { status: "idle" };
        const datalinkState = datalink[acct.ID];
        const datastreamsState = datastreams[acct.ID];
        const dtaState = dta[acct.ID];
        const gasbankState = gasbank[acct.ID] ?? { status: "idle" };
        const confState = conf[acct.ID];
        const creState = cre[acct.ID];
        const automationState = automation[acct.ID];
        const secretState = secrets[acct.ID] ?? { status: "idle" };
        const funcState = functionsState[acct.ID] ?? { status: "idle" };
        const oracleState = oracle[acct.ID];
        const randomState = random[acct.ID];
        return (
          <AccountCard
            key={acct.ID}
            account={acct}
            walletState={walletState}
            vrfState={vrfState}
            ccipState={ccipState}
            datafeedState={datafeedState}
            pricefeedState={pricefeedState}
            datalinkState={datalinkState}
            datastreamsState={datastreamsState}
            dtaState={dtaState}
            gasbankState={gasbankState}
            confState={confState}
            creState={creState}
            automationState={automationState}
            secretState={secretState}
            funcState={funcState}
            oracleState={oracleState}
            randomState={randomState}
            oracleBanner={oracleBanner[acct.ID]}
            cursor={oracleCursors[acct.ID]}
            failedCursor={oracleFailedCursor[acct.ID]}
            loadingCursor={loadingOraclePage[acct.ID]}
            loadingFailed={loadingOracleFailedPage[acct.ID]}
            filter={oracleFilters[acct.ID]}
            retrying={retryingOracle}
            onFilterChange={(value) => setFilter(acct.ID, value)}
            onLoadWallets={() => onLoadWallets(acct.ID)}
            onLoadVRF={() => onLoadVRF(acct.ID)}
            onLoadCCIP={() => onLoadCCIP(acct.ID)}
            onLoadDatafeeds={() => onLoadDatafeeds(acct.ID)}
            onLoadPricefeeds={() => onLoadPricefeeds(acct.ID)}
            onLoadDatalink={() => onLoadDatalink(acct.ID)}
            onLoadDatastreams={() => onLoadDatastreams(acct.ID)}
            onLoadDTA={() => onLoadDTA(acct.ID)}
            onLoadGasbank={() => onLoadGasbank(acct.ID)}
            onLoadConf={() => onLoadConf(acct.ID)}
            onLoadCRE={() => onLoadCRE(acct.ID)}
            onLoadAutomation={() => onLoadAutomation(acct.ID)}
            onLoadSecrets={() => onLoadSecrets(acct.ID)}
            onLoadFunctions={() => onLoadFunctions(acct.ID)}
            onLoadOracle={() => onLoadOracle(acct.ID)}
            onLoadRandom={() => onLoadRandom(acct.ID)}
            onLoadMoreOracle={() => onLoadMoreOracle(acct.ID)}
            onLoadMoreFailed={() => onLoadMoreFailedOracle(acct.ID)}
            onRetry={(requestID) => onRetryOracle(acct.ID, requestID)}
            onCopyCursor={(c) => onCopyCursor(acct.ID, c)}
            onSetAggregation={(feedId, agg) => onSetAggregation(acct.ID, feedId, agg)}
            onCreateChannel={(payload) => onCreateChannel(acct.ID, payload)}
            onCreateDelivery={(payload) => onCreateDelivery(acct.ID, payload)}
            onNotify={onNotify}
            formatSnippet={formatSnippet}
            formatTimestamp={formatTimestamp}
            formatDuration={formatDuration}
            formatAmount={formatAmount}
          />
        );
      })}
    </ul>
  );
}
