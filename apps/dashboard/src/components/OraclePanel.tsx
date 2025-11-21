import { OracleRequest, OracleSource } from "../api";

export type OracleState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; sources: OracleSource[]; requests: OracleRequest[]; failed: OracleRequest[] }
  | { status: "error"; message: string };

type Banner = { tone: "success" | "error"; message: string } | undefined;

type Props = {
  accountID: string;
  oracleState: OracleState | undefined;
  banner: Banner;
  cursor?: string;
  failedCursor?: string;
  loadingCursor?: boolean;
  loadingFailed?: boolean;
  filter?: string;
  onFilterChange: (value: string) => void;
  onReload: () => void;
  onLoadMore: () => void;
  onLoadMoreFailed: () => void;
  onRetry: (requestID: string) => void;
  onCopyCursor: (cursor: string) => void;
  retrying: Record<string, boolean>;
  formatSnippet: (value: string, limit?: number) => string;
  formatTimestamp: (value?: string) => string;
  formatDuration: (ms?: number) => string;
};

function summarizeOracleRequests(requests: OracleRequest[]) {
  let pending = 0;
  let running = 0;
  let failed = 0;
  let succeeded = 0;
  let oldestPendingMs: number | undefined;
  let maxAttempts = 0;
  const now = Date.now();
  requests.forEach((req) => {
    if (req.Status === "pending") {
      pending += 1;
      if (req.CreatedAt) {
        const created = new Date(req.CreatedAt).getTime();
        if (!Number.isNaN(created)) {
          const age = now - created;
          if (oldestPendingMs === undefined || age > oldestPendingMs) {
            oldestPendingMs = age;
          }
        }
      }
    } else if (req.Status === "running") {
      running += 1;
    } else if (req.Status === "failed") {
      failed += 1;
    } else if (req.Status === "succeeded") {
      succeeded += 1;
    }
    if (typeof req.Attempts === "number" && req.Attempts > maxAttempts) {
      maxAttempts = req.Attempts;
    }
  });
  return { pending, running, failed, succeeded, oldestPendingMs, maxAttempts };
}

function statusClass(status: string) {
  const normalized = status?.toLowerCase?.() || "";
  if (normalized === "failed") return "tag error";
  if (normalized === "running") return "tag";
  return "tag subdued";
}

export function OraclePanel({
  accountID,
  oracleState,
  banner,
  cursor,
  failedCursor,
  loadingCursor,
  loadingFailed,
  filter,
  onFilterChange,
  onReload,
  onLoadMore,
  onLoadMoreFailed,
  onRetry,
  onCopyCursor,
  retrying,
  formatSnippet,
  formatTimestamp,
  formatDuration,
}: Props) {
  if (!oracleState || oracleState.status === "idle") return null;
  if (oracleState.status === "error") return <p className="error">Oracle: {oracleState.message}</p>;
  if (oracleState.status === "loading") return <p className="muted">Loading oracle...</p>;

  const summary = summarizeOracleRequests(oracleState.requests);
  const failedRequests = oracleState.failed;
  const filterValue = filter ?? "recent";
  const filteredRequests =
    filterValue === "recent"
      ? oracleState.requests.filter((req) => req.Status !== "failed")
      : filterValue === "all"
        ? oracleState.requests
        : oracleState.requests.filter((req) => req.Status === filterValue);

  return (
    <div className="vrf">
      <div className="row wrap">
        <div className="row gap">
          <h4 className="tight">Oracle Health</h4>
          <span className="tag subdued">pending {summary.pending}</span>
          <span className="tag subdued">running {summary.running}</span>
          <span className={`tag ${summary.failed ? "error" : "subdued"}`}>failed {summary.failed}</span>
          <span className="tag subdued">attempts↓ {summary.maxAttempts}</span>
          {summary.oldestPendingMs !== undefined && <span className="tag subdued">oldest {formatDuration(summary.oldestPendingMs)}</span>}
        </div>
        <button type="button" className="ghost small" onClick={onReload}>
          Refresh
        </button>
      </div>
      {banner && <p className={banner.tone === "error" ? "error" : "muted"}>{banner.message}</p>}

      <div className="row">
        <h4 className="tight">Oracle Sources</h4>
        <span className="tag subdued">{oracleState.sources.length}</span>
      </div>
      {oracleState.sources.length ? (
        <table className="data-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>URL</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {oracleState.sources.map((src: OracleSource) => (
              <tr key={src.ID}>
                <td>
                  <strong>{src.Name}</strong>
                </td>
                <td className="mono">{src.URL}</td>
                <td>{src.Status ? <span className="tag subdued">{src.Status}</span> : "—"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : (
        <p className="muted">
          No oracle sources configured. Use <code>slctl oracle sources create</code> or the API to add one.
        </p>
      )}

      <div className="section">
        <div className="row">
          <h5 className="tight">Failed / DLQ</h5>
          <span className={`tag ${failedRequests.length ? "error" : "subdued"}`}>{failedRequests.length}</span>
        </div>
        {failedRequests.length ? (
          <table className="data-table">
            <thead>
              <tr>
                <th>ID</th>
                <th>Status</th>
                <th>Attempts</th>
                <th>Source</th>
                <th>Updated</th>
                <th>Error</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {failedRequests.map((req) => {
                const retryKey = `${accountID}:${req.ID}`;
                return (
                  <tr key={req.ID}>
                    <td className="mono">{req.ID}</td>
                    <td>
                      <span className={statusClass(req.Status)}>{req.Status}</span>
                    </td>
                    <td className="mono">{req.Attempts ?? 0}</td>
                    <td className="mono">{req.DataSourceID}</td>
                    <td>{formatTimestamp(req.UpdatedAt || req.CompletedAt || req.CreatedAt)}</td>
                    <td className="muted mono">{req.Error ? formatSnippet(req.Error, 48) : "n/a"}</td>
                    <td>
                      <button type="button" className="ghost small" onClick={() => onRetry(req.ID)} disabled={Boolean(retrying[retryKey])}>
                        {retrying[retryKey] ? "Retrying..." : "Retry"}
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        ) : (
          <p className="muted">
            No failed oracle requests. Use <code>slctl oracle requests list --status failed</code> to cross-check the queue.
          </p>
        )}
        {failedCursor && (
          <div className="row">
            <span className="muted mono">next cursor: {failedCursor}</span>
            <button type="button" className="ghost small" onClick={onLoadMoreFailed} disabled={Boolean(loadingFailed)}>
              {loadingFailed ? "Loading..." : "Load more failed"}
            </button>
            <button type="button" className="ghost small" onClick={() => onCopyCursor(failedCursor)}>
              Copy cursor
            </button>
          </div>
        )}
      </div>

      <div className="section">
        <div className="row">
          <div className="row gap">
            <h5 className="tight">Recent requests</h5>
            <span className="tag subdued">{filteredRequests.length}</span>
          </div>
          <select className="select small" value={filterValue} onChange={(e) => onFilterChange(e.target.value)}>
            <option value="recent">Active</option>
            <option value="pending">Pending</option>
            <option value="running">Running</option>
            <option value="succeeded">Succeeded</option>
            <option value="failed">Failed</option>
            <option value="all">All</option>
          </select>
        </div>
        {filteredRequests.length ? (
          <table className="data-table">
            <thead>
              <tr>
                <th>ID</th>
                <th>Status</th>
                <th>Attempts</th>
                <th>Source</th>
                <th>Updated</th>
                <th>Data</th>
              </tr>
            </thead>
            <tbody>
              {filteredRequests.map((req) => (
                <tr key={req.ID}>
                  <td className="mono">{req.ID}</td>
                  <td>
                    <span className={statusClass(req.Status)}>{req.Status}</span>
                  </td>
                  <td className="mono">{req.Attempts ?? 0}</td>
                  <td className="mono">{req.DataSourceID}</td>
                  <td>{formatTimestamp(req.UpdatedAt || req.CreatedAt)}</td>
                  <td className="muted mono">
                    {req.Result ? `Result ${formatSnippet(req.Result, 36)}` : req.Payload ? `Payload ${formatSnippet(req.Payload, 36)}` : "—"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : (
          <p className="muted">
            No recent oracle activity. Queue a request via <code>slctl oracle requests create</code> or the API to see it here.
          </p>
        )}
        {cursor && (
          <div className="row">
            <span className="muted mono">next cursor: {cursor}</span>
            <button type="button" className="ghost small" onClick={onLoadMore} disabled={Boolean(loadingCursor)}>
              {loadingCursor ? "Loading..." : "Load more"}
            </button>
            <button type="button" className="ghost small" onClick={() => onCopyCursor(cursor)}>
              Copy cursor
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
