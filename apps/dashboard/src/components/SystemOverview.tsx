import { Descriptor } from "../api";
import { MetricSample, TimeSeries } from "../metrics";
import { Chart } from "./Chart";
import { JamStatus } from "../api";

type Metrics = {
  rps?: MetricSample[];
  duration?: TimeSeries[];
  oracleQueue?: MetricSample[];
  datafeedStaleness?: MetricSample[];
};

type Props = {
  descriptors: Descriptor[];
  version?: string;
  buildVersion?: string;
  baseUrl: string;
  promBase?: string;
  jam?: JamStatus;
  metrics?: Metrics;
  formatDuration: (value?: number) => string;
};

export function SystemOverview({ descriptors, version, buildVersion, baseUrl, promBase, jam, metrics, formatDuration }: Props) {
  return (
    <div className="grid">
      <div className="card inner">
        <h3>System</h3>
        <p>
          Version: <strong>{version ?? "unknown"}</strong>
        </p>
        {buildVersion && (
          <p className="muted mono">
            Build: <span>{buildVersion}</span>
          </p>
        )}
        <p>
          Base URL: <code>{baseUrl}</code>
        </p>
        {promBase && (
          <p>
            Prometheus: <code>{promBase}</code>
          </p>
        )}
      </div>
      {jam && (
        <div className="card inner">
          <h3>JAM</h3>
          <p>
            Status: <span className={`tag ${jam.enabled ? "subdued" : "error"}`}>{jam.enabled ? "enabled" : "disabled"}</span>
          </p>
          <p className="muted mono">
            Store: {jam.store || "n/a"} • Rate limit: {jam.rate_limit_per_min ?? 0}/min • Max preimage bytes: {jam.max_preimage_bytes ?? 0}
          </p>
          <p className="muted mono">
            Pending packages: {jam.max_pending_packages ?? 0} • Auth required: {jam.auth_required ? "yes" : "no"}
          </p>
          {jam.accumulators_enabled && (
            <p className="muted mono">
              Accumulators enabled • Hash: {jam.accumulator_hash || "n/a"}
              {jam.accumulator_roots && jam.accumulator_roots.length > 0 && (
                <>
                  <br />
                  Roots: {jam.accumulator_roots.map((r) => r.root).join(", ")}
                </>
              )}
            </p>
          )}
        </div>
      )}
      <div className="card inner">
        <h3>Descriptors ({descriptors.length})</h3>
        <ul className="list">
          {descriptors.map((d) => (
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
      {metrics && metrics.rps && (
        <div className="card inner">
          <h3>HTTP RPS (5m)</h3>
          <ul className="list">
            {metrics.rps.map((m) => {
              const label = m.metric.status ? `Status ${m.metric.status}` : "All status codes";
              return (
                <li key={`${m.metric.status || "all"}`}>
                  <div className="row">
                    <span className="tag subdued">{label}</span>
                    <strong>{Number(m.value[1]).toFixed(3)}</strong>
                  </div>
                </li>
              );
            })}
          </ul>
        </div>
      )}
      {metrics?.oracleQueue && metrics.oracleQueue.length > 0 && (
        <div className="card inner">
          <h3>Oracle Attempts</h3>
          <ul className="list">
            {metrics.oracleQueue.map((m) => {
              const statusLabel = m.metric.status || "all";
              return (
                <li key={`${statusLabel}`}>
                  <div className="row">
                    <span className="tag subdued">{statusLabel}</span>
                    <strong>{Number(m.value[1]).toFixed(0)}</strong>
                  </div>
                </li>
              );
            })}
          </ul>
        </div>
      )}
      {metrics?.datafeedStaleness && metrics.datafeedStaleness.length > 0 && (
        <div className="card inner">
          <h3>Datafeed Freshness</h3>
          <ul className="list">
            {metrics.datafeedStaleness.slice(0, 5).map((m) => {
              const feedId = m.metric.feed_id || "feed";
              const status = m.metric.status || "unknown";
              const ageMs = Number(m.value[1]) * 1000;
              return (
                <li key={`${feedId}-${status}`}>
                  <div className="row">
                    <span className="tag subdued">{feedId}</span>
                    <span className={`tag ${status === "stale" ? "error" : "subdued"}`}>{status}</span>
                  </div>
                  <div className="muted mono">age {formatDuration(ageMs)}</div>
                </li>
              );
            })}
          </ul>
        </div>
      )}
      {metrics?.duration && metrics.duration.length > 0 && (
        <div className="card inner">
          <h3>HTTP p90 latency (past 30m)</h3>
          <ul className="list">
            {metrics.duration.map((ts, idx) => {
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
            data={metrics.duration[0].values.map(([x, y]) => ({ x, y: Number(y) }))}
            color="#0f766e"
            height={220}
          />
        </div>
      )}
    </div>
  );
}
