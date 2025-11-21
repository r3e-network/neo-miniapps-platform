import { PriceFeed, PriceSnapshot } from "../api";

export type PricefeedsState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; feeds: PriceFeed[]; snapshots: Record<string, PriceSnapshot[]> }
  | { status: "error"; message: string };

type Props = {
  pricefeedState: PricefeedsState | undefined;
  formatTimestamp: (value?: string) => string;
};

export function PricefeedsPanel({ pricefeedState, formatTimestamp }: Props) {
  if (!pricefeedState || pricefeedState.status === "idle") return null;
  if (pricefeedState.status === "error") return <p className="error">Price feeds: {pricefeedState.message}</p>;
  if (pricefeedState.status === "loading") return <p className="muted">Loading price feeds...</p>;

  return (
    <div className="vrf">
      <div className="row">
        <h4 className="tight">Price feeds</h4>
        <span className="tag subdued">{pricefeedState.feeds.length}</span>
      </div>
      <ul className="wallets">
        {pricefeedState.feeds.map((feed: PriceFeed) => {
          const snapshotsForFeed = pricefeedState.snapshots[feed.ID] ?? [];
          const latest = snapshotsForFeed[0];
          const latestTimestamp = latest?.CollectedAt || latest?.CreatedAt;
          const formattedTs = latestTimestamp ? formatTimestamp(latestTimestamp) : undefined;
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
  );
}
