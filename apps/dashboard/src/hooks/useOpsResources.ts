import { useVrfResources } from "./useVrfResources";
import { useCcipResources } from "./useCcipResources";
import { useConfResources } from "./useConfResources";
import { useCreResources } from "./useCreResources";
import { useAutomationResources } from "./useAutomationResources";
import { useSecretsResources } from "./useSecretsResources";
import { useFunctionsResources } from "./useFunctionsResources";
import { useCallback, useState } from "react";
import { fetchRandomRequests } from "../api";
import { RandomState } from "../components/RandomPanel";

export function useOpsResources(config: { baseUrl: string; token: string }) {
  const vrfHook = useVrfResources(config);
  const ccipHook = useCcipResources(config);
  const confHook = useConfResources(config);
  const creHook = useCreResources(config);
  const automationHook = useAutomationResources(config);
  const secretsHook = useSecretsResources(config);
  const functionsHook = useFunctionsResources(config);
  const [random, setRandom] = useState<Record<string, RandomState>>({});

  const resetOps = useCallback(() => {
    vrfHook.resetVRF();
    ccipHook.resetCCIP();
    confHook.resetConf();
    creHook.resetCRE();
    automationHook.resetAutomation();
    secretsHook.resetSecrets();
    functionsHook.resetFunctions();
    setRandom({});
  }, [automationHook, ccipHook, confHook, creHook, functionsHook, secretsHook, vrfHook]);

  const loadRandom = useCallback(
    async (accountID: string) => {
      setRandom((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
      try {
        const requests = await fetchRandomRequests(config, accountID);
        setRandom((prev) => ({ ...prev, [accountID]: { status: "ready", requests } }));
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        setRandom((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
      }
    },
    [config],
  );

  return {
    vrf: vrfHook.vrf,
    ccip: ccipHook.ccip,
    conf: confHook.conf,
    cre: creHook.cre,
    automation: automationHook.automation,
    secrets: secretsHook.secrets,
    functionsState: functionsHook.functionsState,
    random,
    resetOps,
    loadVRF: vrfHook.loadVRF,
    loadCCIP: ccipHook.loadCCIP,
    loadConf: confHook.loadConf,
    loadCRE: creHook.loadCRE,
    loadAutomation: automationHook.loadAutomation,
    loadSecrets: secretsHook.loadSecrets,
    loadFunctions: functionsHook.loadFunctions,
    loadRandom,
  };
}
