import { Datafeed, DatafeedUpdate } from "../api";

export type DatafeedsState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; feeds: Datafeed[]; updates: Record<string, DatafeedUpdate[]> }
  | { status: "error"; message: string };

type Props = {
  datafeedState: DatafeedsState | undefined;
  formatDuration: (ms?: number) => string;
};

function heartbeatMs(feed: Datafeed): number | undefined {
  if (typeof feed.Heartbeat === "number" && feed.Heartbeat > 0) {
    return feed.Heartbeat / 1_000_000; // ns -> ms
  }
  if (typeof (feed as any).HeartbeatSeconds === "number") {
    const seconds = (feed as any).HeartbeatSeconds;
    return seconds > 0 ? seconds * 1000 : undefined;
  }
  return undefined;
}

function datafeedHealth(feed: Datafeed, updates: DatafeedUpdate[] | undefined) {
  const hbMs = heartbeatMs(feed);
  const latest = updates?.[0];
  if (!latest || !latest.Timestamp) {
    return { status: "empty" as const, ageMs: undefined, heartbeatMs: hbMs };
  }
  const timestamp = new Date(latest.Timestamp).getTime();
  const ageMs = Number.isNaN(timestamp) ? undefined : Date.now() - timestamp;
  const stale = typeof hbMs === "number" && typeof ageMs === "number" && ageMs > hbMs;
  return { status: stale ? "stale" : "healthy", ageMs, heartbeatMs: hbMs };
}

export function DatafeedsPanel({ datafeedState, formatDuration }: Props) {
  if (!datafeedState || datafeedState.status === "idle") return null;
  if (datafeedState.status === "error") return <p className="error">Datafeeds: {datafeedState.message}</p>;
  if (datafeedState.status === "loading") return <p className="muted">Loading feeds...</p>;

  return (
    <div className="vrf">
      <div className="row">
        <h4 className="tight">Datafeeds</h4>
        <span className="tag subdued">{datafeedState.feeds.length}</span>
      </div>
      <ul className="wallets">
        {datafeedState.feeds.map((f: Datafeed) => {
          const updatesForFeed = datafeedState.updates[f.ID];
          const health = datafeedHealth(f, updatesForFeed);
          const heartbeatLabel =
            typeof health.heartbeatMs === "number" && health.heartbeatMs > 0 ? `hb ${formatDuration(health.heartbeatMs)}` : "hb n/a";
          const ageLabel =
            typeof health.ageMs === "number" ? `age ${formatDuration(health.ageMs)}` : updatesForFeed?.length ? "age n/a" : "no updates";
          const statusClass = health.status === "healthy" ? "tag subdued" : health.status === "stale" ? "tag error" : "tag";
          const latest = updatesForFeed?.[0];
          return (
            <li key={f.ID}>
              <div className="row">
                <div>
                  <strong>{f.Pair}</strong>
                  <div className="muted mono">
                    {heartbeatLabel} • Δ {f.ThresholdPPM !== undefined ? `${f.ThresholdPPM}ppm` : "n/a"} • {ageLabel}
                  </div>
                  {f.SignerSet && f.SignerSet.length > 0 && <div className="muted mono">Signers: {f.SignerSet.join(", ")}</div>}
                </div>
                <span className={statusClass}>{health.status}</span>
              </div>
              {latest ? (
                <div className="muted mono">
                  Latest: {latest.Price} @ round {latest.RoundID}
                </div>
              ) : (
                <div className="muted">No updates yet.</div>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}
