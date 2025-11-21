import { useAccountResources } from "./useAccountResources";
import { useOracleData } from "./useOracleData";

export function useAccountsData(config: { baseUrl: string; token: string }) {
  const resources = useAccountResources(config);
  const oracle = useOracleData(config);

  const resetAccounts = () => {
    resources.resetResources();
    oracle.resetOracle();
  };

  return {
    ...resources,
    ...oracle,
    resetAccounts,
  };
}
