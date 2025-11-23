import { useCallback, useState } from "react";
import { fetchSecrets } from "../api";
import { SecretsState } from "../components/SecretsPanel";

export function useSecretsResources(config: { baseUrl: string; token: string; tenant?: string }) {
  const [secrets, setSecrets] = useState<Record<string, SecretsState>>({});

  const resetSecrets = useCallback(() => setSecrets({}), []);

  const loadSecrets = useCallback(
    async (accountID: string) => {
      setSecrets((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
      try {
        const items = await fetchSecrets(config, accountID);
        setSecrets((prev) => ({ ...prev, [accountID]: { status: "ready", items } }));
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        setSecrets((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
      }
    },
    [config],
  );

  return { secrets, loadSecrets, resetSecrets };
}
