import { useMemo, useState } from "react";
import { getMiniAppSDK } from "../miniapp";

export function App() {
  const sdk = useMemo(() => {
    try {
      return getMiniAppSDK();
    } catch {
      return null;
    }
  }, []);

  const [status, setStatus] = useState("");
  const [appId, setAppId] = useState("com.miniapps.react-starter");
  const [symbol, setSymbol] = useState("NEO-USD");
  const [amountGas, setAmountGas] = useState("0.1");
  const [proposalId, setProposalId] = useState("proposal-1");
  const [neoAmount, setNeoAmount] = useState("1");

  const run = async (fn: () => Promise<unknown>) => {
    setStatus("");
    try {
      const res = await fn();
      setStatus(JSON.stringify(res, null, 2));
    } catch (e) {
      setStatus(String((e as any)?.message ?? e));
    }
  };

  return (
    <div style={{ fontFamily: "ui-sans-serif, system-ui", padding: 16, maxWidth: 820 }}>
      <h1 style={{ margin: 0 }}>Neo MiniApp (React Starter Kit)</h1>
      <p style={{ marginTop: 8, color: "#444" }}>
        Uses <code>window.MiniAppSDK</code> (injected by the host or bridged via postMessage).
      </p>

      {!sdk ? (
        <div style={{ padding: 12, border: "1px solid #ddd", borderRadius: 8, background: "#fafafa" }}>
          <div style={{ fontWeight: 600 }}>MiniAppSDK not detected</div>
          <div style={{ marginTop: 6, color: "#555" }}>
            Run this app inside the platform host (iframe) or load the bridge script in your own host.
          </div>
        </div>
      ) : null}

      <div style={{ display: "grid", gap: 12, marginTop: 16 }}>
        <label style={{ display: "grid", gap: 4 }}>
          <span>app_id</span>
          <input value={appId} onChange={(e) => setAppId(e.target.value)} />
        </label>

        <div style={{ display: "flex", gap: 12, flexWrap: "wrap" }}>
          <button onClick={() => run(() => sdk?.wallet.getAddress())} disabled={!sdk}>
            wallet.getAddress
          </button>
          <button
            onClick={() =>
              run(async () => {
                const intent = await sdk!.payments.payGAS(appId, amountGas, "hello");
                if (sdk!.wallet.invokeIntent) {
                  const tx = await sdk!.wallet.invokeIntent(intent.request_id);
                  return { intent, tx };
                }
                if (!sdk!.wallet.invokeInvocation) return intent;
                const tx = await sdk!.wallet.invokeInvocation(intent.invocation);
                return { intent, tx };
              })}
            disabled={!sdk}
          >
            payments.payGAS (+ invoke if available)
          </button>
          <button onClick={() => run(() => sdk!.rng.requestRandom(appId))} disabled={!sdk}>
            rng.requestRandom
          </button>
        </div>

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
          <label style={{ display: "grid", gap: 4 }}>
            <span>amount_gas</span>
            <input value={amountGas} onChange={(e) => setAmountGas(e.target.value)} />
          </label>
          <label style={{ display: "grid", gap: 4 }}>
            <span>symbol</span>
            <input value={symbol} onChange={(e) => setSymbol(e.target.value)} />
          </label>
        </div>

        <div style={{ display: "flex", gap: 12, flexWrap: "wrap" }}>
          <button onClick={() => run(() => sdk!.datafeed.getPrice(symbol))} disabled={!sdk}>
            datafeed.getPrice
          </button>
        </div>

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
          <label style={{ display: "grid", gap: 4 }}>
            <span>proposal_id</span>
            <input value={proposalId} onChange={(e) => setProposalId(e.target.value)} />
          </label>
          <label style={{ display: "grid", gap: 4 }}>
            <span>neo_amount</span>
            <input value={neoAmount} onChange={(e) => setNeoAmount(e.target.value)} />
          </label>
        </div>

        <div style={{ display: "flex", gap: 12, flexWrap: "wrap" }}>
          <button
            onClick={() =>
              run(async () => {
                const intent = await sdk!.governance.vote(appId, proposalId, neoAmount, true);
                if (sdk!.wallet.invokeIntent) {
                  const tx = await sdk!.wallet.invokeIntent(intent.request_id);
                  return { intent, tx };
                }
                if (!sdk!.wallet.invokeInvocation) return intent;
                const tx = await sdk!.wallet.invokeInvocation(intent.invocation);
                return { intent, tx };
              })}
            disabled={!sdk}
          >
            governance.vote (+ invoke if available)
          </button>
        </div>
      </div>

      <pre
        style={{
          marginTop: 16,
          padding: 12,
          background: "#0b1020",
          color: "#e8e8e8",
          borderRadius: 8,
          overflowX: "auto",
          minHeight: 120,
        }}
      >
        {status || "…"}
      </pre>
    </div>
  );
}
