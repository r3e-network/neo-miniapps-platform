import { useCallback, useState } from "react";
import { fetchAutomationJobs, fetchTriggers } from "../api";
import { AutomationState } from "../components/AutomationPanel";

export function useAutomationResources(config: { baseUrl: string; token: string; tenant?: string }) {
  const [automation, setAutomation] = useState<Record<string, AutomationState>>({});

  const resetAutomation = useCallback(() => setAutomation({}), []);

  const loadAutomation = useCallback(
    async (accountID: string) => {
      setAutomation((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
      try {
        const [jobs, triggers] = await Promise.all([fetchAutomationJobs(config, accountID), fetchTriggers(config, accountID)]);
        setAutomation((prev) => ({ ...prev, [accountID]: { status: "ready", jobs, triggers } }));
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        setAutomation((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
      }
    },
    [config],
  );

  return { automation, loadAutomation, resetAutomation };
}
