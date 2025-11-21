import { useCallback, useState } from "react";
import { fetchVRFKeys, fetchVRFRequests } from "../api";
import { VRFState } from "../components/VRFPanel";

export function useVrfResources(config: { baseUrl: string; token: string }) {
  const [vrf, setVRF] = useState<Record<string, VRFState>>({});

  const resetVRF = useCallback(() => setVRF({}), []);

  const loadVRF = useCallback(
    async (accountID: string) => {
      setVRF((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
      try {
        const [keys, requests] = await Promise.all([fetchVRFKeys(config, accountID), fetchVRFRequests(config, accountID)]);
        setVRF((prev) => ({ ...prev, [accountID]: { status: "ready", keys, requests } }));
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        setVRF((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
      }
    },
    [config],
  );

  return { vrf, loadVRF, resetVRF };
}
