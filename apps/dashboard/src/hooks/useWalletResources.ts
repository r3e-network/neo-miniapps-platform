import { useCallback, useState } from "react";
import { fetchWorkspaceWallets } from "../api";
import { WalletState } from "../components/AccountCard";

export function useWalletResources(config: { baseUrl: string; token: string }) {
  const [wallets, setWallets] = useState<Record<string, WalletState>>({});

  const resetWallets = useCallback(() => setWallets({}), []);

  const loadWallets = useCallback(
    async (accountID: string) => {
      setWallets((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
      try {
        const data = await fetchWorkspaceWallets(config, accountID);
        setWallets((prev) => ({ ...prev, [accountID]: { status: "ready", items: data } }));
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        setWallets((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
      }
    },
    [config],
  );

  return { wallets, loadWallets, resetWallets };
}
