import {
  GasAccount,
  GasTransaction,
  GasbankDeadLetter,
  GasbankSettlementAttempt,
  GasbankSummary,
} from "../api";

export type GasbankState =
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

type Props = {
  gasbankState: GasbankState | undefined;
  formatAmount: (value: number | undefined) => string;
  formatTimestamp: (value?: string) => string;
};

export function GasbankPanel({ gasbankState, formatAmount, formatTimestamp }: Props) {
  if (!gasbankState || gasbankState.status === "idle") return null;
  if (gasbankState.status === "error") return <p className="error">Gasbank: {gasbankState.message}</p>;
  if (gasbankState.status === "loading") return <p className="muted">Loading gasbank...</p>;

  const summary: GasbankSummary = gasbankState.summary;
  const accounts: GasAccount[] = gasbankState.accounts;
  const transactions: GasTransaction[] = gasbankState.transactions;
  const withdrawals: GasTransaction[] = gasbankState.withdrawals;
  const deadletters: GasbankDeadLetter[] = gasbankState.deadletters;
  const attempts: Record<string, GasbankSettlementAttempt[]> = gasbankState.attempts;

  return (
    <div className="card inner gasbank-panel">
      <div className="section-header">
        <div>
          <h4 className="tight">Gasbank Overview</h4>
          <p className="muted">Updated {formatTimestamp(summary.generated_at)}</p>
        </div>
      </div>
      <div className="metrics-grid">
        <div className="metric-card">
          <p>Total Balance</p>
          <strong>{formatAmount(summary.total_balance)}</strong>
        </div>
        <div className="metric-card">
          <p>Available</p>
          <strong>{formatAmount(summary.total_available)}</strong>
        </div>
        <div className="metric-card">
          <p>Locked</p>
          <strong>{formatAmount(summary.total_locked)}</strong>
        </div>
        <div className="metric-card">
          <p>Pending Withdrawals</p>
          <strong>{summary.pending_withdrawals}</strong>
          <span className="muted">({formatAmount(summary.pending_amount)})</span>
        </div>
      </div>
      <div className="timeline">
        <div>
          <p className="muted">Last Deposit</p>
          {summary.last_deposit ? (
            <>
              <div className="mono">{summary.last_deposit.id}</div>
              <div className="muted mono">
                {formatAmount(summary.last_deposit.amount)} → {summary.last_deposit.to_address ?? "n/a"}
              </div>
            </>
          ) : (
            <div className="muted">No deposits recorded.</div>
          )}
        </div>
        <div>
          <p className="muted">Last Withdrawal</p>
          {summary.last_withdrawal ? (
            <>
              <div className="mono">{summary.last_withdrawal.id}</div>
              <div className="muted mono">
                {formatAmount(summary.last_withdrawal.amount)} → {summary.last_withdrawal.to_address ?? "n/a"}
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
          <span className="tag subdued">{accounts.length}</span>
        </div>
        {accounts.length ? (
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
              {accounts.map((account) => (
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
            <span className="tag subdued">{transactions.length}</span>
          </div>
          {transactions.length ? (
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
                {transactions.map((tx) => (
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
            <span className="tag subdued">{withdrawals.length}</span>
          </div>
          {withdrawals.length ? (
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
                {withdrawals.map((withdrawal) => (
                  <tr key={withdrawal.ID}>
                    <td className="mono">{withdrawal.ID}</td>
                    <td>{withdrawal.Status}</td>
                    <td>{formatAmount(withdrawal.Amount)}</td>
                    <td>{formatTimestamp(withdrawal.CreatedAt)}</td>
                    <td className="mono">
                      {(() => {
                        const rows = attempts[withdrawal.ID] || [];
                        if (!rows.length) return "—";
                        const latest = rows[0];
                        const label = latest.Status || "attempted";
                        return `${rows.length} (${label})`;
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
          <span className="tag subdued">{deadletters.length}</span>
        </div>
        {deadletters.length ? (
          <ul className="wallets deadletters">
            {deadletters.map((entry) => (
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
  );
}
