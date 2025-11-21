import { Secret } from "../api";

export type SecretsState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; items: Secret[] }
  | { status: "error"; message: string };

type Props = {
  secretState: SecretsState | undefined;
  formatTimestamp: (value?: string) => string;
};

export function SecretsPanel({ secretState, formatTimestamp }: Props) {
  if (!secretState || secretState.status === "idle") return null;
  if (secretState.status === "error") return <p className="error">Secrets: {secretState.message}</p>;
  if (secretState.status === "loading") return <p className="muted">Loading secrets...</p>;

  return (
    <div className="vrf">
      <div className="row">
        <h4 className="tight">Secrets</h4>
        <span className="tag subdued">{secretState.items.length}</span>
      </div>
      <ul className="wallets">
        {secretState.items.map((sec: Secret) => (
          <li key={sec.ID}>
            <div className="row">
              <div>
                <strong>{sec.Name}</strong>
                <div className="muted mono">{sec.ID}</div>
              </div>
              <span className="tag subdued">{formatTimestamp(sec.UpdatedAt)}</span>
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
