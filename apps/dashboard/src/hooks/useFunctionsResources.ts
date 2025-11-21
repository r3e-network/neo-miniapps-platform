import { useCallback, useState } from "react";
import { fetchFunctionExecutions, fetchFunctions } from "../api";
import { FunctionsState } from "../components/FunctionsPanel";

export function useFunctionsResources(config: { baseUrl: string; token: string }) {
  const [functionsState, setFunctionsState] = useState<Record<string, FunctionsState>>({});

  const resetFunctions = useCallback(() => setFunctionsState({}), []);

  const loadFunctions = useCallback(
    async (accountID: string) => {
      setFunctionsState((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
      try {
        const funcs = await fetchFunctions(config, accountID);
        const enriched = await Promise.all(
          funcs.map(async (fn) => {
            const executions = await fetchFunctionExecutions(config, accountID, fn.ID, 5);
            return { fn, executions };
          }),
        );
        setFunctionsState((prev) => ({ ...prev, [accountID]: { status: "ready", items: enriched } }));
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        setFunctionsState((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
      }
    },
    [config],
  );

  return { functionsState, loadFunctions, resetFunctions };
}
