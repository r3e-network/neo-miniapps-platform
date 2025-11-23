import { useCallback, useState } from "react";
import { useFeedsResources } from "./useFeedsResources";
import { useGasbankResources } from "./useGasbankResources";
import { useWalletResources } from "./useWalletResources";
import { useOpsResources } from "./useOpsResources";

export function useAccountResources(config: { baseUrl: string; token: string }) {
  const walletsHook = useWalletResources(config);
  const feeds = useFeedsResources(config);
  const gasbankHook = useGasbankResources(config);
  const ops = useOpsResources(config);

  const resetResources = useCallback(() => {
    feeds.resetFeeds();
    gasbankHook.resetGasbank();
    walletsHook.resetWallets();
    ops.resetOps();
  }, [feeds, gasbankHook, ops, walletsHook]);

  return {
    ...feeds,
    ...gasbankHook,
    wallets: walletsHook.wallets,
    vrf: ops.vrf,
    ccip: ops.ccip,
    conf: ops.conf,
    cre: ops.cre,
    automation: ops.automation,
    secrets: ops.secrets,
    functionsState: ops.functionsState,
    random: ops.random,
    resetResources,
    loadWallets: walletsHook.loadWallets,
    loadVRF: ops.loadVRF,
    loadCCIP: ops.loadCCIP,
    loadConf: ops.loadConf,
    loadCRE: ops.loadCRE,
    loadAutomation: ops.loadAutomation,
    loadSecrets: ops.loadSecrets,
    loadFunctions: ops.loadFunctions,
    loadRandom: ops.loadRandom,
    setAggregation: feeds.setAggregation,
    createChannel: feeds.createChannel,
    createDelivery: feeds.createDelivery,
  };
}
