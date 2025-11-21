import { useCallback, useState } from "react";
import {
  GasTransaction,
  GasbankSettlementAttempt,
  fetchGasAccounts,
  fetchGasDeadLetters,
  fetchGasTransactions,
  fetchGasbankSummary,
  fetchGasWithdrawals,
  fetchWithdrawalAttempts,
} from "../api";
import { GasbankState } from "../components/GasbankPanel";

export function useGasbankResources(config: { baseUrl: string; token: string }) {
  const [gasbank, setGasbank] = useState<Record<string, GasbankState>>({});

  const resetGasbank = useCallback(() => setGasbank({}), []);

  const loadGasbank = useCallback(
    async (accountID: string) => {
      setGasbank((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
      try {
        const [summary, accounts, deadletters] = await Promise.all([
          fetchGasbankSummary(config, accountID),
          fetchGasAccounts(config, accountID),
          fetchGasDeadLetters(config, accountID, 10),
        ]);
        let transactions: GasTransaction[] = [];
        let withdrawals: GasTransaction[] = [];
        let attempts: Record<string, GasbankSettlementAttempt[]> = {};
        const primaryAccountID = accounts[0]?.ID;
        if (primaryAccountID) {
          [transactions, withdrawals] = await Promise.all([
            fetchGasTransactions(config, accountID, primaryAccountID, 20),
            fetchGasWithdrawals(config, accountID, primaryAccountID, undefined, 15),
          ]);
          if (withdrawals.length) {
            const fetched = await Promise.all(
              withdrawals.map(async (w) => {
                try {
                  const rows = await fetchWithdrawalAttempts(config, accountID, w.ID, 5);
                  return { id: w.ID, attempts: rows };
                } catch {
                  return { id: w.ID, attempts: [] };
                }
              }),
            );
            attempts = fetched.reduce<Record<string, GasbankSettlementAttempt[]>>((acc, curr) => {
              acc[curr.id] = curr.attempts;
              return acc;
            }, {});
          }
        }
        setGasbank((prev) => ({
          ...prev,
          [accountID]: { status: "ready", summary, accounts, transactions, withdrawals, deadletters, attempts },
        }));
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        setGasbank((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
      }
    },
    [config],
  );

  return { gasbank, loadGasbank, resetGasbank };
}
