import { useCallback, useState } from "react";
import { fetchCREExecutors, fetchCREPlaybooks, fetchCRERuns } from "../api";
import { CREState } from "../components/CREPanel";

export function useCreResources(config: { baseUrl: string; token: string; tenant?: string }) {
  const [cre, setCRE] = useState<Record<string, CREState>>({});

  const resetCRE = useCallback(() => setCRE({}), []);

  const loadCRE = useCallback(
    async (accountID: string) => {
      setCRE((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
      try {
        const [executors, playbooks, runs] = await Promise.all([
          fetchCREExecutors(config, accountID),
          fetchCREPlaybooks(config, accountID),
          fetchCRERuns(config, accountID),
        ]);
        setCRE((prev) => ({ ...prev, [accountID]: { status: "ready", executors, playbooks, runs } }));
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        setCRE((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
      }
    },
    [config],
  );

  return { cre, loadCRE, resetCRE };
}
