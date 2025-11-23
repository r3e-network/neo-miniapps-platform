import { useCallback, useState } from "react";
import { fetchEnclaves } from "../api";
import { ConfState } from "../components/ConfPanel";

export function useConfResources(config: { baseUrl: string; token: string; tenant?: string }) {
  const [conf, setConf] = useState<Record<string, ConfState>>({});

  const resetConf = useCallback(() => setConf({}), []);

  const loadConf = useCallback(
    async (accountID: string) => {
      setConf((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
      try {
        const enclaves = await fetchEnclaves(config, accountID);
        setConf((prev) => ({ ...prev, [accountID]: { status: "ready", enclaves } }));
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        setConf((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
      }
    },
    [config],
  );

  return { conf, loadConf, resetConf };
}
