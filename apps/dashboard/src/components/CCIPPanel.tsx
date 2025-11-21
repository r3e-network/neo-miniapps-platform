import { CCIPMessage, Lane } from "../api";

export type CCIPState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; lanes: Lane[]; messages: CCIPMessage[] }
  | { status: "error"; message: string };

type Props = {
  ccipState: CCIPState | undefined;
};

export function CCIPPanel({ ccipState }: Props) {
  if (!ccipState || ccipState.status === "idle") return null;
  if (ccipState.status === "error") return <p className="error">CCIP: {ccipState.message}</p>;
  if (ccipState.status === "loading") return <p className="muted">Loading CCIP...</p>;

  return (
    <div className="vrf">
      <div className="row">
        <h4 className="tight">CCIP Lanes</h4>
        <span className="tag subdued">{ccipState.lanes.length}</span>
      </div>
      <ul className="wallets">
        {ccipState.lanes.map((lane: Lane) => (
          <li key={lane.ID}>
            <div className="row">
              <div>
                <strong>{lane.Name}</strong>
                <div className="muted mono">
                  {lane.SourceChain} → {lane.DestChain}
                </div>
              </div>
              {lane.Tags && lane.Tags.length > 0 && <span className="tag subdued">{lane.Tags.join(", ")}</span>}
            </div>
          </li>
        ))}
      </ul>
      <div className="row">
        <h4 className="tight">CCIP Messages</h4>
        <span className="tag subdued">{ccipState.messages.length}</span>
      </div>
      <ul className="wallets">
        {ccipState.messages.map((msg: CCIPMessage) => (
          <li key={msg.ID}>
            <div className="row">
              <div className="mono">{msg.ID}</div>
              <span className="tag subdued">{msg.Status}</span>
            </div>
            {msg.LaneID && <div className="muted mono">Lane: {msg.LaneID}</div>}
          </li>
        ))}
      </ul>
    </div>
  );
}
