import { useCallback, useState } from "react";
import { fetchLanes, fetchMessages } from "../api";
import { CCIPState } from "../components/CCIPPanel";

export function useCcipResources(config: { baseUrl: string; token: string }) {
  const [ccip, setCCIP] = useState<Record<string, CCIPState>>({});

  const resetCCIP = useCallback(() => setCCIP({}), []);

  const loadCCIP = useCallback(
    async (accountID: string) => {
      setCCIP((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
      try {
        const [lanes, messages] = await Promise.all([fetchLanes(config, accountID), fetchMessages(config, accountID)]);
        setCCIP((prev) => ({ ...prev, [accountID]: { status: "ready", lanes, messages } }));
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        setCCIP((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
      }
    },
    [config],
  );

  return { ccip, loadCCIP, resetCCIP };
}
