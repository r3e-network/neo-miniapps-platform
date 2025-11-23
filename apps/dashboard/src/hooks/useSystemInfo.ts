import { useCallback, useState } from "react";
import { Account, Descriptor, JamStatus, fetchAccounts, fetchDescriptors, fetchHealth, fetchSystemStatus, fetchVersion } from "../api";
import { MetricSample, MetricsConfig, promQuery, promQueryRange, TimeSeries } from "../metrics";

export type SystemState =
  | { status: "idle" }
  | { status: "loading" }
  | {
      status: "ready";
      descriptors: Descriptor[];
      accounts: Account[];
      version?: string;
      jam?: JamStatus;
      metrics?: {
        rps?: MetricSample[];
        duration?: TimeSeries[];
        oracleQueue?: MetricSample[];
        datafeedStaleness?: MetricSample[];
      };
    }
  | { status: "error"; message: string };

type ServerConfig = { baseUrl: string; token: string; tenant?: string };

export function useSystemInfo(config: ServerConfig, promConfig: MetricsConfig, canQuery: boolean) {
  const [state, setState] = useState<SystemState>({ status: "idle" });
  const [systemVersion, setSystemVersion] = useState<string>("");

  const load = useCallback(async () => {
    if (!canQuery) {
      setState({ status: "idle" });
      return;
    }
    setState({ status: "loading" });
    try {
      const [health, descriptors, accounts, version, systemStatus] = await Promise.all([
        fetchHealth(config),
        fetchDescriptors(config),
        fetchAccounts(config),
        fetchVersion(config),
        fetchSystemStatus(config),
      ]);
      let metrics:
        | { rps?: MetricSample[]; duration?: TimeSeries[]; oracleQueue?: MetricSample[]; datafeedStaleness?: MetricSample[] }
        | undefined;
      if (promConfig.prometheusBaseUrl) {
        try {
          const now = Date.now() / 1000;
          const [rps, duration, oracleQueue, datafeedStaleness] = await Promise.all([
            promQuery('sum(rate(http_requests_total[5m])) by (status)', promConfig),
            promQueryRange(
              "histogram_quantile(0.9, sum by (le) (rate(http_request_duration_seconds_bucket[5m])))",
              now - 1800,
              now,
              60,
              promConfig,
            ),
            promQuery("sum(service_layer_oracle_request_attempts_total) by (status)", promConfig),
            promQuery("service_layer_datafeeds_stale_seconds", promConfig),
          ]);
          metrics = { rps, duration, oracleQueue, datafeedStaleness };
        } catch {
          metrics = undefined;
        }
      }
      setState({ status: "ready", descriptors, accounts, version: health.version ?? version.version, metrics, jam: systemStatus.jam });
      setSystemVersion(version.version);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setState({ status: "error", message });
    }
  }, [canQuery, config, promConfig]);

  return { state, systemVersion, load };
}
