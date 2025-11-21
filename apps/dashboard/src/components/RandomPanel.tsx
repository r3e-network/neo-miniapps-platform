import { RandomRequest } from "../api";

export type RandomState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; requests: RandomRequest[] }
  | { status: "error"; message: string };

type Props = {
  randomState: RandomState | undefined;
  formatTimestamp: (value?: string) => string;
  formatSnippet: (value: string, limit?: number) => string;
};

export function RandomPanel({ randomState, formatSnippet, formatTimestamp }: Props) {
  if (!randomState || randomState.status === "idle") return null;
  if (randomState.status === "error") return <p className="error">Randomness: {randomState.message}</p>;
  if (randomState.status === "loading") return <p className="muted">Loading randomness...</p>;

  return (
    <div className="vrf">
      <div className="row">
        <h4 className="tight">Random Requests</h4>
        <span className="tag subdued">{randomState.requests.length}</span>
      </div>
      <ul className="wallets">
        {randomState.requests.map((req: RandomRequest) => {
          const label = req.RequestID && req.RequestID.trim().length > 0 ? req.RequestID : `Counter ${req.Counter}`;
          const timestamp = req.CreatedAt ? formatTimestamp(req.CreatedAt) : undefined;
          return (
            <li key={`${label}-${req.Counter}`}>
              <div className="row">
                <div>
                  <div className="mono">{label}</div>
                  {timestamp && <div className="muted mono">{timestamp}</div>}
                </div>
                <span className="tag subdued">{req.Length} bytes</span>
              </div>
              <div className="muted mono">Value: {formatSnippet(req.Value, 28)}</div>
              <div className="muted mono">Signature: {formatSnippet(req.Signature, 28)}</div>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
