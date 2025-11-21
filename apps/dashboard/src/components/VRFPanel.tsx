import { VRFKey, VRFRequest } from "../api";

export type VRFState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; keys: VRFKey[]; requests: VRFRequest[] }
  | { status: "error"; message: string };

type Props = {
  vrfState: VRFState | undefined;
};

export function VRFPanel({ vrfState }: Props) {
  if (!vrfState || vrfState.status === "idle") return null;
  if (vrfState.status === "error") return <p className="error">VRF: {vrfState.message}</p>;
  if (vrfState.status === "loading") return <p className="muted">Loading VRF...</p>;

  return (
    <div className="vrf">
      <div className="row">
        <h4 className="tight">VRF Keys</h4>
        <span className="tag subdued">{vrfState.keys.length}</span>
      </div>
      <ul className="wallets">
        {vrfState.keys.map((k: VRFKey) => (
          <li key={k.ID}>
            <div className="row">
              <div>
                <div className="mono">{k.PublicKey}</div>
                {k.WalletAddress && <div className="muted mono">{k.WalletAddress}</div>}
              </div>
              <span className="tag subdued">{k.Status || "unknown"}</span>
            </div>
          </li>
        ))}
      </ul>
      <div className="row">
        <h4 className="tight">VRF Requests</h4>
        <span className="tag subdued">{vrfState.requests.length}</span>
      </div>
      <ul className="wallets">
        {vrfState.requests.map((r: VRFRequest) => (
          <li key={r.ID}>
            <div className="row">
              <div className="mono">{r.ID}</div>
              <span className="tag subdued">{r.Status}</span>
            </div>
            <div className="muted mono">{r.Consumer}</div>
          </li>
        ))}
      </ul>
    </div>
  );
}
