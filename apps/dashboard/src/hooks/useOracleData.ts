import { useCallback, useState } from "react";
import { OracleRequest, fetchOracleRequests, fetchOracleSources, retryOracleRequest } from "../api";
import { OracleState } from "../components/OraclePanel";

type Banner = { tone: "success" | "error"; message: string };

function mergeRequestLists(...lists: OracleRequest[][]) {
  const merged = new Map<string, OracleRequest>();
  for (const list of lists) {
    for (const req of list) {
      merged.set(req.ID, req);
    }
  }
  return Array.from(merged.values()).sort((a, b) => {
    const aTime = new Date(a.UpdatedAt || a.CompletedAt || a.CreatedAt || "").getTime();
    const bTime = new Date(b.UpdatedAt || b.CompletedAt || b.CreatedAt || "").getTime();
    const safeA = Number.isNaN(aTime) ? 0 : aTime;
    const safeB = Number.isNaN(bTime) ? 0 : bTime;
    return safeB - safeA;
  });
}

export function useOracleData(config: { baseUrl: string; token: string; tenant?: string }) {
  const [oracle, setOracle] = useState<Record<string, OracleState>>({});
  const [oracleBanner, setOracleBanner] = useState<Record<string, Banner | undefined>>({});
  const [retryingOracle, setRetryingOracle] = useState<Record<string, boolean>>({});
  const [oracleFilters, setOracleFilters] = useState<Record<string, string>>({});
  const [oracleCursors, setOracleCursors] = useState<Record<string, string | undefined>>({});
  const [oracleFailedCursor, setOracleFailedCursor] = useState<Record<string, string | undefined>>({});
  const [loadingOraclePage, setLoadingOraclePage] = useState<Record<string, boolean>>({});
  const [loadingOracleFailedPage, setLoadingOracleFailedPage] = useState<Record<string, boolean>>({});

  const resetOracle = useCallback(() => {
    setOracle({});
    setOracleBanner({});
    setRetryingOracle({});
    setOracleFilters({});
    setOracleCursors({});
    setOracleFailedCursor({});
    setLoadingOraclePage({});
    setLoadingOracleFailedPage({});
  }, []);

  const loadOracle = useCallback(
    async (accountID: string) => {
      setOracle((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
      try {
        const [sources, recent, failed] = await Promise.all([
          fetchOracleSources(config, accountID),
          fetchOracleRequests(config, accountID, 100),
          fetchOracleRequests(config, accountID, 100, "failed"),
        ]);
        const merged = mergeRequestLists(recent.items, failed.items);
        setOracle((prev) => ({ ...prev, [accountID]: { status: "ready", sources, requests: merged, failed: failed.items } }));
        setOracleCursors((prev) => ({ ...prev, [accountID]: recent.nextCursor }));
        setOracleFailedCursor((prev) => ({ ...prev, [accountID]: failed.nextCursor }));
        setOracleBanner((prev) => ({ ...prev, [accountID]: undefined }));
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        setOracle((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
      }
    },
    [config],
  );

  const loadMoreOracle = useCallback(
    async (accountID: string) => {
      const cursor = oracleCursors[accountID];
      const filterValue = oracleFilters[accountID] ?? "recent";
      const statusFilter = filterValue === "all" ? undefined : filterValue === "recent" ? undefined : filterValue;
      if (!cursor) return;
      if (loadingOraclePage[accountID]) return;
      setLoadingOraclePage((prev) => ({ ...prev, [accountID]: true }));
      try {
        const page = await fetchOracleRequests(config, accountID, 100, statusFilter as OracleRequest["Status"] | undefined, cursor);
        setOracle((prev) => {
          const acctState = prev[accountID];
          if (!acctState || acctState.status !== "ready") {
            return prev;
          }
          const merged = mergeRequestLists(acctState.requests, page.items);
          return { ...prev, [accountID]: { ...acctState, requests: merged } };
        });
        setOracleCursors((prev) => ({ ...prev, [accountID]: page.nextCursor }));
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        setOracleBanner((prev) => ({ ...prev, [accountID]: { tone: "error", message: `Load more failed: ${message}` } }));
      } finally {
        setLoadingOraclePage((prev) => {
          const next = { ...prev };
          delete next[accountID];
          return next;
        });
      }
    },
    [config, loadingOraclePage, oracleCursors, oracleFilters],
  );

  const loadMoreFailedOracle = useCallback(
    async (accountID: string) => {
      const cursor = oracleFailedCursor[accountID];
      if (!cursor) return;
      if (loadingOracleFailedPage[accountID]) return;
      setLoadingOracleFailedPage((prev) => ({ ...prev, [accountID]: true }));
      try {
        const page = await fetchOracleRequests(config, accountID, 100, "failed", cursor);
        setOracle((prev) => {
          const acctState = prev[accountID];
          if (!acctState || acctState.status !== "ready") {
            return prev;
          }
          const mergedFailed = mergeRequestLists(acctState.failed, page.items);
          const mergedAll = mergeRequestLists(acctState.requests, page.items);
          return { ...prev, [accountID]: { ...acctState, failed: mergedFailed, requests: mergedAll } };
        });
        setOracleFailedCursor((prev) => ({ ...prev, [accountID]: page.nextCursor }));
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        setOracleBanner((prev) => ({ ...prev, [accountID]: { tone: "error", message: `Load failed DLQ: ${message}` } }));
      } finally {
        setLoadingOracleFailedPage((prev) => {
          const next = { ...prev };
          delete next[accountID];
          return next;
        });
      }
    },
    [config, loadingOracleFailedPage, oracleFailedCursor],
  );

  const retryOracle = useCallback(
    async (accountID: string, requestID: string) => {
      const key = `${accountID}:${requestID}`;
      setRetryingOracle((prev) => ({ ...prev, [key]: true }));
      try {
        const updated = await retryOracleRequest(config, accountID, requestID);
        setOracle((prev) => {
          const acctState = prev[accountID];
          if (!acctState || acctState.status !== "ready") {
            return prev;
          }
          const nextFailed = acctState.failed.filter((req) => req.ID !== updated.ID && req.Status === "failed");
          const nextRequests = mergeRequestLists(acctState.requests, [updated]);
          return { ...prev, [accountID]: { ...acctState, requests: nextRequests, failed: nextFailed } };
        });
        setOracleBanner((prev) => ({ ...prev, [accountID]: { tone: "success", message: `Requeued request ${requestID}` } }));
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        setOracleBanner((prev) => ({ ...prev, [accountID]: { tone: "error", message: `Retry failed: ${message}` } }));
      } finally {
        setRetryingOracle((prev) => {
          const next = { ...prev };
          delete next[key];
          return next;
        });
      }
    },
    [config],
  );

  const copyCursor = useCallback(async (accountID: string, cursor: string) => {
    try {
      await navigator.clipboard.writeText(cursor);
      setOracleBanner((prev) => ({ ...prev, [accountID]: { tone: "success", message: "Cursor copied" } }));
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setOracleBanner((prev) => ({ ...prev, [accountID]: { tone: "error", message: `Copy failed: ${message}` } }));
    }
  }, []);

  return {
    oracle,
    oracleBanner,
    retryingOracle,
    oracleFilters,
    oracleCursors,
    oracleFailedCursor,
    loadingOraclePage,
    loadingOracleFailedPage,
    setOracleFilters,
    resetOracle,
    loadOracle,
    loadMoreOracle,
    loadMoreFailedOracle,
    retryOracle,
    copyCursor,
  };
}
