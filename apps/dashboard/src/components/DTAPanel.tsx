import { DTAOrder, DTAProduct } from "../api";

export type DTAState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; products: DTAProduct[]; orders: DTAOrder[] }
  | { status: "error"; message: string };

type Props = { dtaState: DTAState | undefined };

export function DTAPanel({ dtaState }: Props) {
  if (!dtaState || dtaState.status === "idle") return null;
  if (dtaState.status === "error") return <p className="error">DTA: {dtaState.message}</p>;
  if (dtaState.status === "loading") return <p className="muted">Loading DTA...</p>;

  return (
    <div className="vrf">
      <div className="row">
        <h4 className="tight">DTA Products</h4>
        <span className="tag subdued">{dtaState.products.length}</span>
      </div>
      <ul className="wallets">
        {dtaState.products.map((p: DTAProduct) => (
          <li key={p.ID}>
            <div className="row">
              <div>
                <strong>{p.Name}</strong>
                <div className="muted mono">
                  {p.Symbol} • {p.Type}
                </div>
              </div>
              <span className="tag subdued">{p.Status || "unknown"}</span>
            </div>
            {p.SettlementTerms && <div className="muted mono">{p.SettlementTerms}</div>}
          </li>
        ))}
      </ul>
      <div className="row">
        <h4 className="tight">DTA Orders</h4>
        <span className="tag subdued">{dtaState.orders.length}</span>
      </div>
      <ul className="wallets">
        {dtaState.orders.map((o: DTAOrder) => (
          <li key={o.ID}>
            <div className="row">
              <div className="mono">{o.ID}</div>
              <span className="tag subdued">{o.Status || "unknown"}</span>
            </div>
            <div className="muted mono">
              {o.Type} {o.Amount} @ {o.WalletAddress}
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
