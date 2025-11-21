import { Enclave } from "../api";

export type ConfState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; enclaves: Enclave[] }
  | { status: "error"; message: string };

type Props = { confState: ConfState | undefined };

export function ConfPanel({ confState }: Props) {
  if (!confState || confState.status === "idle") return null;
  if (confState.status === "error") return <p className="error">Confidential: {confState.message}</p>;
  if (confState.status === "loading") return <p className="muted">Loading TEE...</p>;

  return (
    <div className="vrf">
      <div className="row">
        <h4 className="tight">Enclaves</h4>
        <span className="tag subdued">{confState.enclaves.length}</span>
      </div>
      <ul className="wallets">
        {confState.enclaves.map((e: Enclave) => (
          <li key={e.ID}>
            <div className="row">
              <div>
                <strong>{e.Name}</strong>
                <div className="muted mono">{e.Provider}</div>
              </div>
              <span className="tag subdued">{e.Status || "unknown"}</span>
            </div>
            <div className="muted mono">{e.Measurement}</div>
          </li>
        ))}
      </ul>
    </div>
  );
}
