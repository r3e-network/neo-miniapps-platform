import { ComponentType, useCallback, useEffect, useMemo, useState } from "react";

import styles from "./BuiltinApp.module.css";

type PayIntent = {
  request_id: string;
};

type RandomnessResult = {
  randomness?: string;
  signature?: string;
  public_key?: string;
  attestation_hash?: string;
  request_id?: string;
};

type PriceResult = {
  feed_id?: string;
  pair?: string;
  price?: string | number;
  decimals?: number;
  timestamp?: string;
};

type MiniAppSDK = {
  wallet?: {
    getAddress?: () => Promise<string>;
    invokeIntent?: (requestId: string) => Promise<unknown>;
  };
  payments?: {
    payGAS: (appId: string, amount: string, memo?: string) => Promise<PayIntent>;
  };
  rng?: {
    requestRandom: (appId: string) => Promise<RandomnessResult>;
  };
  datafeed?: {
    getPrice: (symbol: string) => Promise<PriceResult>;
  };
};

export type BuiltinAppProps = {
  appId?: string;
  view?: string;
};

type PanelProps = {
  sdk: MiniAppSDK | null;
  appId: string;
};

type BuiltinDefinition = {
  id: string;
  view: string;
  name: string;
  summary: string;
  component: ComponentType<PanelProps>;
};

type StatusTone = "info" | "success" | "error";

type StatusState = {
  tone: StatusTone;
  message: string;
};

function useMiniAppSDK(): MiniAppSDK | null {
  const [sdk, setSdk] = useState<MiniAppSDK | null>(null);

  useEffect(() => {
    if (typeof window === "undefined") return;
    const update = () => setSdk((window as any).MiniAppSDK ?? null);
    update();
    window.addEventListener("miniapp-sdk-ready", update);
    return () => window.removeEventListener("miniapp-sdk-ready", update);
  }, []);

  return sdk;
}

function formatAmountInput(value: string, decimals = 8): string {
  const trimmed = value.trim();
  if (!trimmed) return "0";
  const normalized = trimmed.replace(/,/g, "");
  const numeric = Number(normalized);
  if (!Number.isFinite(numeric)) return trimmed;
  const fixed = numeric.toFixed(decimals);
  return fixed.replace(/\.0+$/, "").replace(/(\.\d+?)0+$/, "$1");
}

function formatDecimalString(rawValue: string, decimals: number): string {
  const normalized = rawValue.replace(/^0+/, "") || "0";
  if (decimals <= 0) return normalized;
  const padded = normalized.padStart(decimals + 1, "0");
  const whole = padded.slice(0, -decimals);
  const fraction = padded.slice(-decimals).replace(/0+$/, "");
  return fraction ? `${whole}.${fraction}` : whole;
}

function formatPrice(raw: string | number | undefined, decimals = 0): string {
  if (raw === undefined || raw === null) return "--";
  if (typeof raw === "number") {
    return decimals > 0 ? raw.toFixed(decimals) : raw.toString();
  }
  const cleaned = raw.trim();
  if (!/^\d+$/.test(cleaned)) return cleaned;
  return formatDecimalString(cleaned, decimals);
}

function parsePriceNumber(raw: string | number | undefined, decimals = 0): number | null {
  if (raw === undefined || raw === null) return null;
  if (typeof raw === "number") return Number.isFinite(raw) ? raw : null;
  const cleaned = raw.trim();
  if (!/^\d+$/.test(cleaned)) {
    const numeric = Number(cleaned);
    return Number.isFinite(numeric) ? numeric : null;
  }
  const formatted = formatDecimalString(cleaned, decimals);
  const numeric = Number(formatted);
  return Number.isFinite(numeric) ? numeric : null;
}

function formatTimestamp(raw: string | number | undefined): string {
  if (!raw) return "--";
  const date = new Date(raw);
  if (Number.isNaN(date.getTime())) return String(raw);
  return date.toLocaleString();
}

function hexToBytes(hex: string): number[] {
  const cleaned = hex.trim().replace(/^0x/i, "");
  if (!cleaned || cleaned.length % 2 !== 0) return [];
  const bytes: number[] = [];
  for (let index = 0; index < cleaned.length; index += 2) {
    const pair = cleaned.slice(index, index + 2);
    const value = Number.parseInt(pair, 16);
    if (Number.isNaN(value)) return [];
    bytes.push(value);
  }
  return bytes;
}

function randomIndexFromHex(hex: string | undefined, modulo: number): number {
  if (modulo <= 0) throw new Error("Invalid randomness modulus");
  if (!hex) throw new Error("Randomness output missing");
  const bytes = hexToBytes(hex);
  if (bytes.length === 0) throw new Error("Randomness output invalid");
  let value = 0;
  const limit = Math.min(bytes.length, 6);
  for (let index = 0; index < limit; index += 1) {
    value = (value << 8) + bytes[index];
  }
  return value % modulo;
}

function StatusBanner({ status }: { status: StatusState | null }) {
  if (!status) return null;
  let className = styles.status;
  if (status.tone === "error") className = `${styles.status} ${styles.statusError}`;
  if (status.tone === "success") className = `${styles.status} ${styles.statusSuccess}`;
  return <div className={className}>{status.message}</div>;
}

function PriceTickerPanel({ sdk }: PanelProps) {
  const [symbol, setSymbol] = useState("NEO-USD");
  const [priceResult, setPriceResult] = useState<PriceResult | null>(null);
  const [status, setStatus] = useState<StatusState | null>(null);
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [isLoading, setIsLoading] = useState(false);

  const refreshPrice = useCallback(async () => {
    if (!sdk?.datafeed?.getPrice) {
      setStatus({ tone: "error", message: "MiniAppSDK datafeed is not available in this host." });
      return;
    }
    if (!symbol.trim()) {
      setStatus({ tone: "error", message: "Enter a symbol like NEO-USD or BTC-USD." });
      return;
    }
    setIsLoading(true);
    setStatus({ tone: "info", message: "Fetching price feed…" });
    try {
      const res = await sdk.datafeed.getPrice(symbol.trim());
      setPriceResult(res);
      setStatus({ tone: "success", message: "Price updated." });
    } catch (err) {
      setStatus({ tone: "error", message: String((err as any)?.message ?? err) });
    } finally {
      setIsLoading(false);
    }
  }, [sdk, symbol]);

  useEffect(() => {
    if (!autoRefresh) return;
    const id = window.setInterval(() => {
      refreshPrice();
    }, 5000);
    return () => window.clearInterval(id);
  }, [autoRefresh, refreshPrice]);

  useEffect(() => {
    if (!sdk) return;
    refreshPrice();
  }, [sdk, refreshPrice]);

  const priceValue = priceResult
    ? formatPrice(priceResult.price, priceResult.decimals ?? 0)
    : "--";
  const timestamp = priceResult?.timestamp ? formatTimestamp(priceResult.timestamp) : "--";

  return (
    <div className={styles.section}>
      <div className={styles.grid}>
        <div className={styles.cardInset}>
          <div className={styles.kpiRow}>
            <div className={styles.kpiCard}>
              <div className={styles.kpiLabel}>Symbol</div>
              <div className={styles.kpiValue}>{priceResult?.pair ?? symbol}</div>
            </div>
            <div className={styles.kpiCard}>
              <div className={styles.kpiLabel}>Price</div>
              <div className={styles.kpiValue}>{priceValue}</div>
            </div>
            <div className={styles.kpiCard}>
              <div className={styles.kpiLabel}>Last update</div>
              <div className={styles.kpiValue}>{timestamp}</div>
            </div>
          </div>
        </div>

        <div className={styles.cardInset}>
          <div className={styles.inputGroup}>
            <label className={styles.label} htmlFor="price-symbol">
              Symbol
            </label>
            <input
              id="price-symbol"
              className={styles.input}
              value={symbol}
              onChange={(event) => setSymbol(event.target.value.toUpperCase())}
            />
          </div>
          <div className={styles.buttonRow}>
            <button
              className={styles.buttonPrimary}
              onClick={refreshPrice}
              disabled={isLoading || !sdk?.datafeed?.getPrice}
            >
              Refresh
            </button>
            <button
              className={styles.buttonSecondary}
              onClick={() => setAutoRefresh((prev) => !prev)}
            >
              Auto: {autoRefresh ? "On" : "Off"}
            </button>
          </div>
          <div className={styles.quickRow}>
            {["NEO-USD", "GAS-USD", "BTC-USD", "ETH-USD"].map((item) => (
              <button
                key={item}
                className={item === symbol ? styles.quickActive : styles.quickButton}
                onClick={() => setSymbol(item)}
              >
                {item}
              </button>
            ))}
          </div>
          <StatusBanner status={status} />
        </div>
      </div>
    </div>
  );
}

type GameConfig = {
  name: string;
  description: string;
  choices: string[];
  minBet: number;
  maxBet: number;
  multiplier: number;
  memoPrefix: string;
};

type GameStats = {
  plays: number;
  wins: number;
  losses: number;
  streak: number;
};

type GameResult = {
  choice: string;
  result: string;
  win: boolean;
  timestamp: string;
};

function RandomGamePanel({ sdk, appId, config }: PanelProps & { config: GameConfig }) {
  const [selectedChoice, setSelectedChoice] = useState(config.choices[0]);
  const [betAmount, setBetAmount] = useState(config.minBet.toString());
  const [status, setStatus] = useState<StatusState | null>(null);
  const [history, setHistory] = useState<GameResult[]>([]);
  const [stats, setStats] = useState<GameStats>({ plays: 0, wins: 0, losses: 0, streak: 0 });
  const [isBusy, setIsBusy] = useState(false);
  const [lastIntent, setLastIntent] = useState<PayIntent | null>(null);
  const [lastTx, setLastTx] = useState<string>("");

  useEffect(() => {
    setSelectedChoice(config.choices[0]);
    setBetAmount(config.minBet.toString());
    setStatus(null);
    setHistory([]);
    setStats({ plays: 0, wins: 0, losses: 0, streak: 0 });
  }, [config]);

  const handlePlay = useCallback(async () => {
    if (!sdk?.payments?.payGAS || !sdk?.rng?.requestRandom) {
      setStatus({ tone: "error", message: "MiniAppSDK payments + RNG are required for this game." });
      return;
    }
    const amountValue = Number(betAmount);
    if (!Number.isFinite(amountValue)) {
      setStatus({ tone: "error", message: "Enter a valid GAS amount." });
      return;
    }
    if (amountValue < config.minBet || amountValue > config.maxBet) {
      setStatus({
        tone: "error",
        message: `Bet must be between ${config.minBet} and ${config.maxBet} GAS.`,
      });
      return;
    }

    setIsBusy(true);
    setStatus({ tone: "info", message: "Creating payment intent…" });
    try {
      const formattedAmount = formatAmountInput(betAmount);
      const intent = await sdk.payments.payGAS(appId, formattedAmount, `${config.memoPrefix}:${selectedChoice}`);
      setLastIntent(intent);

      if (sdk.wallet?.invokeIntent) {
        try {
          const tx = await sdk.wallet.invokeIntent(intent.request_id);
          setLastTx(JSON.stringify(tx, null, 2));
        } catch (err) {
          setStatus({ tone: "error", message: String((err as any)?.message ?? err) });
        }
      }

      setStatus({ tone: "info", message: "Requesting randomness…" });
      const rngResult = await sdk.rng.requestRandom(appId);
      const index = randomIndexFromHex(rngResult.randomness, config.choices.length);
      const result = config.choices[index];
      const win = result === selectedChoice;

      setStats((prev) => {
        const nextStreak = win ? Math.max(1, prev.streak + 1) : Math.min(-1, prev.streak - 1);
        return {
          plays: prev.plays + 1,
          wins: prev.wins + (win ? 1 : 0),
          losses: prev.losses + (win ? 0 : 1),
          streak: nextStreak,
        };
      });

      setHistory((prev) => {
        const next: GameResult = {
          choice: selectedChoice,
          result,
          win,
          timestamp: new Date().toLocaleTimeString(),
        };
        return [next, ...prev].slice(0, 6);
      });

      setStatus({
        tone: win ? "success" : "error",
        message: win
          ? `Win! Result: ${result}. Payout multiplier ${config.multiplier}x.`
          : `Result: ${result}. Better luck next round.`,
      });
    } catch (err) {
      setStatus({ tone: "error", message: String((err as any)?.message ?? err) });
    } finally {
      setIsBusy(false);
    }
  }, [sdk, appId, betAmount, selectedChoice, config]);

  return (
    <div className={styles.section}>
      <div className={styles.grid}>
        <div className={styles.cardInset}>
          <div className={styles.sectionTitle}>{config.description}</div>
          <div className={styles.choiceGrid}>
            {config.choices.map((choice) => (
              <button
                key={choice}
                className={choice === selectedChoice ? styles.choiceActive : styles.choiceButton}
                onClick={() => setSelectedChoice(choice)}
              >
                {choice}
              </button>
            ))}
          </div>
          <div className={styles.inputGroup}>
            <label className={styles.label} htmlFor={`bet-${config.memoPrefix}`}>
              Bet amount (GAS)
            </label>
            <input
              id={`bet-${config.memoPrefix}`}
              className={styles.input}
              value={betAmount}
              onChange={(event) => setBetAmount(event.target.value)}
            />
          </div>
          <div className={styles.buttonRow}>
            <button className={styles.buttonPrimary} onClick={handlePlay} disabled={isBusy}>
              {isBusy ? "Playing…" : "Play"}
            </button>
            <button
              className={styles.buttonSecondary}
              onClick={() => setBetAmount(config.minBet.toString())}
            >
              Reset Bet
            </button>
          </div>
          <StatusBanner status={status} />
        </div>

        <div className={styles.cardInset}>
          <div className={styles.sectionTitle}>Game Stats</div>
          <div className={styles.kpiRow}>
            <div className={styles.kpiCard}>
              <div className={styles.kpiLabel}>Plays</div>
              <div className={styles.kpiValue}>{stats.plays}</div>
            </div>
            <div className={styles.kpiCard}>
              <div className={styles.kpiLabel}>Wins</div>
              <div className={styles.kpiValue}>{stats.wins}</div>
            </div>
            <div className={styles.kpiCard}>
              <div className={styles.kpiLabel}>Losses</div>
              <div className={styles.kpiValue}>{stats.losses}</div>
            </div>
            <div className={styles.kpiCard}>
              <div className={styles.kpiLabel}>Streak</div>
              <div className={styles.kpiValue}>{stats.streak}</div>
            </div>
          </div>

          {history.length > 0 ? (
            <div className={styles.historyList}>
              {history.map((item, index) => (
                <div key={`${item.timestamp}-${index}`} className={styles.historyItem}>
                  <span>
                    {item.timestamp} • {item.choice} → {item.result}
                  </span>
                  <span className={item.win ? styles.badgeWin : styles.badgeLoss}>
                    {item.win ? "Win" : "Loss"}
                  </span>
                </div>
              ))}
            </div>
          ) : (
            <div className={styles.emptyState}>No rounds played yet.</div>
          )}

          {lastIntent ? (
            <div className={styles.resultBox}>{JSON.stringify(lastIntent, null, 2)}</div>
          ) : null}
          {lastTx ? <div className={styles.resultBox}>{lastTx}</div> : null}
        </div>
      </div>
    </div>
  );
}

function LotteryPanel({ sdk, appId }: PanelProps) {
  const ticketPrice = 0.1;
  const [ticketCount, setTicketCount] = useState(1);
  const [ownedTickets, setOwnedTickets] = useState(0);
  const [status, setStatus] = useState<StatusState | null>(null);
  const [drawResult, setDrawResult] = useState<string>("");
  const [isBusy, setIsBusy] = useState(false);

  const totalCost = ticketCount * ticketPrice;

  const buyTickets = useCallback(async () => {
    if (!sdk?.payments?.payGAS) {
      setStatus({ tone: "error", message: "MiniAppSDK payments are required." });
      return;
    }
    if (ticketCount <= 0) {
      setStatus({ tone: "error", message: "Select at least one ticket." });
      return;
    }

    setIsBusy(true);
    setStatus({ tone: "info", message: "Creating payment intent…" });
    try {
      const amountText = formatAmountInput(totalCost.toString());
      const intent = await sdk.payments.payGAS(appId, amountText, `lottery:${ticketCount}`);
      if (sdk.wallet?.invokeIntent) {
        await sdk.wallet.invokeIntent(intent.request_id);
      }
      setOwnedTickets((prev) => prev + ticketCount);
      setStatus({ tone: "success", message: "Tickets purchased. Ready for draw." });
    } catch (err) {
      setStatus({ tone: "error", message: String((err as any)?.message ?? err) });
    } finally {
      setIsBusy(false);
    }
  }, [sdk, appId, ticketCount, totalCost]);

  const drawWinner = useCallback(async () => {
    if (!sdk?.rng?.requestRandom) {
      setStatus({ tone: "error", message: "MiniAppSDK RNG is required to draw." });
      return;
    }
    if (ownedTickets <= 0) {
      setStatus({ tone: "error", message: "Buy tickets before drawing." });
      return;
    }

    setIsBusy(true);
    setStatus({ tone: "info", message: "Drawing winner…" });
    try {
      const result = await sdk.rng.requestRandom(appId);
      const totalTickets = Math.max(10, ownedTickets + 4);
      const winningIndex = randomIndexFromHex(result.randomness, totalTickets);
      const isWinner = winningIndex < ownedTickets;
      const message = isWinner
        ? `Winning ticket #${winningIndex + 1}. You win the round!`
        : `Winning ticket #${winningIndex + 1}. Try again next round.`;
      setDrawResult(message);
      setStatus({ tone: isWinner ? "success" : "error", message });
    } catch (err) {
      setStatus({ tone: "error", message: String((err as any)?.message ?? err) });
    } finally {
      setIsBusy(false);
    }
  }, [sdk, appId, ownedTickets]);

  return (
    <div className={styles.section}>
      <div className={styles.grid}>
        <div className={styles.cardInset}>
          <div className={styles.sectionTitle}>Tickets</div>
          <div className={styles.inputGroup}>
            <label className={styles.label} htmlFor="lottery-count">
              Ticket count
            </label>
            <input
              id="lottery-count"
              className={styles.input}
              type="number"
              min={1}
              max={50}
              value={ticketCount}
              onChange={(event) => setTicketCount(Number(event.target.value) || 1)}
            />
          </div>
          <div className={styles.kpiRow}>
            <div className={styles.kpiCard}>
              <div className={styles.kpiLabel}>Ticket price</div>
              <div className={styles.kpiValue}>{ticketPrice} GAS</div>
            </div>
            <div className={styles.kpiCard}>
              <div className={styles.kpiLabel}>Total cost</div>
              <div className={styles.kpiValue}>{totalCost.toFixed(2)} GAS</div>
            </div>
            <div className={styles.kpiCard}>
              <div className={styles.kpiLabel}>Owned</div>
              <div className={styles.kpiValue}>{ownedTickets}</div>
            </div>
          </div>
          <div className={styles.buttonRow}>
            <button className={styles.buttonPrimary} onClick={buyTickets} disabled={isBusy}>
              Buy Tickets
            </button>
            <button className={styles.buttonSecondary} onClick={drawWinner} disabled={isBusy}>
              Draw Winner
            </button>
          </div>
          <StatusBanner status={status} />
        </div>

        <div className={styles.cardInset}>
          <div className={styles.sectionTitle}>Draw Result</div>
          <div className={styles.resultBox}>{drawResult || "No draw yet."}</div>
        </div>
      </div>
    </div>
  );
}

function PredictionMarketPanel({ sdk, appId }: PanelProps) {
  const [symbol, setSymbol] = useState("NEO-USD");
  const [direction, setDirection] = useState<"up" | "down">("up");
  const [stakeAmount, setStakeAmount] = useState("0.5");
  const [status, setStatus] = useState<StatusState | null>(null);
  const [result, setResult] = useState<string>("");
  const [isBusy, setIsBusy] = useState(false);

  const placePrediction = useCallback(async () => {
    if (!sdk?.payments?.payGAS || !sdk?.datafeed?.getPrice) {
      setStatus({ tone: "error", message: "Payments + datafeed must be enabled in this host." });
      return;
    }
    setIsBusy(true);
    setStatus({ tone: "info", message: "Creating prediction intent…" });
    try {
      const amountText = formatAmountInput(stakeAmount);
      const intent = await sdk.payments.payGAS(appId, amountText, `prediction:${direction}:${symbol}`);
      if (sdk.wallet?.invokeIntent) {
        await sdk.wallet.invokeIntent(intent.request_id);
      }

      setStatus({ tone: "info", message: "Capturing opening price…" });
      const start = await sdk.datafeed.getPrice(symbol);
      const startValue = parsePriceNumber(start.price, start.decimals ?? 0);
      if (startValue === null) throw new Error("Invalid start price response");

      await new Promise((resolve) => setTimeout(resolve, 4000));

      setStatus({ tone: "info", message: "Capturing closing price…" });
      const end = await sdk.datafeed.getPrice(symbol);
      const endValue = parsePriceNumber(end.price, end.decimals ?? 0);
      if (endValue === null) throw new Error("Invalid end price response");

      const won = direction === "up" ? endValue >= startValue : endValue <= startValue;
      const outcome = won ? "win" : "loss";
      const summary = `Open ${startValue} → Close ${endValue} (${direction}).`;
      setResult(`${summary} Outcome: ${outcome.toUpperCase()}.`);
      setStatus({ tone: won ? "success" : "error", message: summary });
    } catch (err) {
      setStatus({ tone: "error", message: String((err as any)?.message ?? err) });
    } finally {
      setIsBusy(false);
    }
  }, [sdk, appId, direction, stakeAmount, symbol]);

  return (
    <div className={styles.section}>
      <div className={styles.grid}>
        <div className={styles.cardInset}>
          <div className={styles.sectionTitle}>Prediction Setup</div>
          <div className={styles.inputGroup}>
            <label className={styles.label} htmlFor="prediction-symbol">
              Symbol
            </label>
            <input
              id="prediction-symbol"
              className={styles.input}
              value={symbol}
              onChange={(event) => setSymbol(event.target.value.toUpperCase())}
            />
          </div>
          <div className={styles.inputGroup}>
            <label className={styles.label} htmlFor="prediction-stake">
              Stake (GAS)
            </label>
            <input
              id="prediction-stake"
              className={styles.input}
              value={stakeAmount}
              onChange={(event) => setStakeAmount(event.target.value)}
            />
          </div>
          <div className={styles.buttonRow}>
            <button
              className={direction === "up" ? styles.choiceActive : styles.choiceButton}
              onClick={() => setDirection("up")}
            >
              Predict Up
            </button>
            <button
              className={direction === "down" ? styles.choiceActive : styles.choiceButton}
              onClick={() => setDirection("down")}
            >
              Predict Down
            </button>
          </div>
          <button className={styles.buttonPrimary} onClick={placePrediction} disabled={isBusy}>
            {isBusy ? "Running…" : "Place Prediction"}
          </button>
          <StatusBanner status={status} />
        </div>

        <div className={styles.cardInset}>
          <div className={styles.sectionTitle}>Outcome</div>
          <div className={styles.resultBox}>{result || "No prediction settled yet."}</div>
        </div>
      </div>
    </div>
  );
}

function FlashloanPanel({ sdk, appId }: PanelProps) {
  const [loanAmount, setLoanAmount] = useState("25");
  const [feeBps, setFeeBps] = useState(9);
  const [status, setStatus] = useState<StatusState | null>(null);
  const [result, setResult] = useState<string>("");
  const [isBusy, setIsBusy] = useState(false);

  const feeAmount = useMemo(() => {
    const amount = Number(loanAmount);
    if (!Number.isFinite(amount)) return "0";
    return (amount * (feeBps / 10000)).toFixed(4);
  }, [loanAmount, feeBps]);

  const requestFlashloan = useCallback(async () => {
    if (!sdk?.payments?.payGAS || !sdk?.rng?.requestRandom) {
      setStatus({ tone: "error", message: "MiniAppSDK payments + RNG are required." });
      return;
    }
    setIsBusy(true);
    setStatus({ tone: "info", message: "Submitting flashloan fee…" });
    try {
      const intent = await sdk.payments.payGAS(appId, formatAmountInput(feeAmount), `flashloan:${loanAmount}`);
      if (sdk.wallet?.invokeIntent) {
        await sdk.wallet.invokeIntent(intent.request_id);
      }
      const rng = await sdk.rng.requestRandom(appId);
      const loanId = rng.request_id ?? rng.randomness?.slice(0, 10) ?? "loan";
      const summary = `Loan request ${loanId} accepted. Fee: ${feeAmount} GAS.`;
      setResult(summary);
      setStatus({ tone: "success", message: summary });
    } catch (err) {
      setStatus({ tone: "error", message: String((err as any)?.message ?? err) });
    } finally {
      setIsBusy(false);
    }
  }, [sdk, appId, feeAmount, loanAmount]);

  return (
    <div className={styles.section}>
      <div className={styles.grid}>
        <div className={styles.cardInset}>
          <div className={styles.sectionTitle}>Flashloan Request</div>
          <div className={styles.inputGroup}>
            <label className={styles.label} htmlFor="flashloan-amount">
              Loan amount (GAS)
            </label>
            <input
              id="flashloan-amount"
              className={styles.input}
              value={loanAmount}
              onChange={(event) => setLoanAmount(event.target.value)}
            />
          </div>
          <div className={styles.inputGroup}>
            <label className={styles.label} htmlFor="flashloan-fee">
              Fee (bps)
            </label>
            <input
              id="flashloan-fee"
              className={styles.input}
              type="number"
              min={1}
              max={200}
              value={feeBps}
              onChange={(event) => setFeeBps(Number(event.target.value) || 0)}
            />
          </div>
          <div className={styles.kpiRow}>
            <div className={styles.kpiCard}>
              <div className={styles.kpiLabel}>Estimated fee</div>
              <div className={styles.kpiValue}>{feeAmount} GAS</div>
            </div>
          </div>
          <button className={styles.buttonPrimary} onClick={requestFlashloan} disabled={isBusy}>
            {isBusy ? "Submitting…" : "Request Flashloan"}
          </button>
          <StatusBanner status={status} />
        </div>

        <div className={styles.cardInset}>
          <div className={styles.sectionTitle}>Execution Log</div>
          <div className={styles.resultBox}>{result || "No flashloan executed yet."}</div>
        </div>
      </div>
    </div>
  );
}

const randomGameConfigs = {
  "builtin-coin-flip": {
    name: "Coin Flip",
    description: "Pick heads or tails and let the TEE RNG decide.",
    choices: ["heads", "tails"],
    minBet: 0.1,
    maxBet: 10,
    multiplier: 2,
    memoPrefix: "coin-flip",
  },
  "builtin-dice-game": {
    name: "Dice Game",
    description: "Choose a number 1-6 for a higher multiplier.",
    choices: ["1", "2", "3", "4", "5", "6"],
    minBet: 0.1,
    maxBet: 5,
    multiplier: 6,
    memoPrefix: "dice",
  },
  "builtin-scratch-card": {
    name: "Scratch Card",
    description: "Scratch to reveal a lucky icon.",
    choices: ["neo", "gas", "star", "diamond", "bonus"],
    minBet: 0.2,
    maxBet: 8,
    multiplier: 4,
    memoPrefix: "scratch",
  },
} as const satisfies Record<string, GameConfig>;

const builtinDefinitions: BuiltinDefinition[] = [
  {
    id: "builtin-price-ticker",
    view: "price-ticker",
    name: "Price Ticker",
    summary: "Live oracle prices with signed datafeed responses.",
    component: PriceTickerPanel,
  },
  {
    id: "builtin-coin-flip",
    view: "coin-flip",
    name: "Coin Flip",
    summary: "50/50 RNG game powered by GAS payments.",
    component: (props) => <RandomGamePanel {...props} config={randomGameConfigs["builtin-coin-flip"]} />,
  },
  {
    id: "builtin-dice-game",
    view: "dice-game",
    name: "Dice Game",
    summary: "Six-sided RNG with higher payout multiplier.",
    component: (props) => <RandomGamePanel {...props} config={randomGameConfigs["builtin-dice-game"]} />,
  },
  {
    id: "builtin-scratch-card",
    view: "scratch-card",
    name: "Scratch Card",
    summary: "Reveal lucky symbols using attested randomness.",
    component: (props) => <RandomGamePanel {...props} config={randomGameConfigs["builtin-scratch-card"]} />,
  },
  {
    id: "builtin-lottery",
    view: "lottery",
    name: "Lottery",
    summary: "Buy tickets and draw a winner with TEE RNG.",
    component: LotteryPanel,
  },
  {
    id: "builtin-prediction-market",
    view: "prediction-market",
    name: "Prediction Market",
    summary: "Predict price direction using datafeed snapshots.",
    component: PredictionMarketPanel,
  },
  {
    id: "builtin-flashloan",
    view: "flashloan",
    name: "Flashloan",
    summary: "Request a flashloan quote with GAS fee settlement.",
    component: FlashloanPanel,
  },
];

const builtinMap = new Map(builtinDefinitions.map((item) => [item.id, item]));

function resolveBuiltin(appId?: string, view?: string): BuiltinDefinition {
  if (appId && builtinMap.has(appId)) {
    return builtinMap.get(appId)!;
  }
  if (view) {
    const match = builtinDefinitions.find((item) => item.view === view || item.id === view);
    if (match) return match;
  }
  return builtinDefinitions[0];
}

export default function BuiltinApp({ appId, view }: BuiltinAppProps) {
  const sdk = useMiniAppSDK();
  const resolved = useMemo(() => resolveBuiltin(appId, view), [appId, view]);
  const [activeId, setActiveId] = useState(resolved.id);
  const [walletAddress, setWalletAddress] = useState<string>("");
  const [walletStatus, setWalletStatus] = useState<StatusState | null>(null);

  useEffect(() => {
    setActiveId(resolved.id);
  }, [resolved.id]);

  const active = builtinDefinitions.find((item) => item.id === activeId) ?? builtinDefinitions[0];
  const ActiveComponent = active.component;

  const requestAddress = useCallback(async () => {
    if (!sdk?.wallet?.getAddress) {
      setWalletStatus({ tone: "error", message: "Wallet address not available in this host." });
      return;
    }
    setWalletStatus({ tone: "info", message: "Requesting wallet address…" });
    try {
      const address = await sdk.wallet.getAddress();
      setWalletAddress(address);
      setWalletStatus({ tone: "success", message: "Wallet connected." });
    } catch (err) {
      setWalletStatus({ tone: "error", message: String((err as any)?.message ?? err) });
    }
  }, [sdk]);

  return (
    <main className={styles.root}>
      <header className={styles.header}>
        <div>
          <h1 className={styles.title}>Neo Built-in MiniApps</h1>
          <p className={styles.subtitle}>
            Module Federation micro-frontends aligned with GAS-only settlement and NEO-only governance.
          </p>
        </div>
        <div className={styles.sdkPanel}>
          <div className={styles.sdkLabel}>SDK Status</div>
          <div className={sdk ? `${styles.sdkBadge} ${styles.sdkReady}` : `${styles.sdkBadge} ${styles.sdkMissing}`}>
            {sdk ? "Connected" : "Not Detected"}
          </div>
          <div className={styles.walletRow}>
            <button
              className={styles.buttonSecondary}
              onClick={requestAddress}
              disabled={!sdk?.wallet?.getAddress}
            >
              Get Wallet Address
            </button>
            {walletAddress ? <div className={styles.walletAddress}>{walletAddress}</div> : null}
            <StatusBanner status={walletStatus} />
          </div>
        </div>
      </header>

      <nav className={styles.nav}>
        {builtinDefinitions.map((item) => (
          <button
            key={item.id}
            className={item.id === activeId ? styles.navButtonActive : styles.navButton}
            onClick={() => setActiveId(item.id)}
          >
            {item.name}
          </button>
        ))}
      </nav>

      <section className={styles.card}>
        <div className={styles.cardHeader}>
          <div>
            <h2 className={styles.cardTitle}>{active.name}</h2>
            <div className={styles.cardSummary}>{active.summary}</div>
          </div>
          <div className={styles.appId}>{active.id}</div>
        </div>
        <ActiveComponent sdk={sdk} appId={active.id} />
      </section>
    </main>
  );
}
