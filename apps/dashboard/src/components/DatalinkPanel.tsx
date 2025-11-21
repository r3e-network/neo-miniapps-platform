import { DatalinkChannel, DatalinkDelivery } from "../api";

export type DatalinkState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; channels: DatalinkChannel[]; deliveries: DatalinkDelivery[] }
  | { status: "error"; message: string };

type Props = { datalinkState: DatalinkState | undefined };

export function DatalinkPanel({ datalinkState }: Props) {
  if (!datalinkState || datalinkState.status === "idle") return null;
  if (datalinkState.status === "error") return <p className="error">Datalink: {datalinkState.message}</p>;
  if (datalinkState.status === "loading") return <p className="muted">Loading link...</p>;

  return (
    <div className="vrf">
      <div className="row">
        <h4 className="tight">Datalink Channels</h4>
        <span className="tag subdued">{datalinkState.channels.length}</span>
      </div>
      <ul className="wallets">
        {datalinkState.channels.map((c: DatalinkChannel) => (
          <li key={c.ID}>
            <div className="row">
              <div>
                <strong>{c.Name}</strong>
                <div className="muted mono">{c.Endpoint}</div>
              </div>
              {c.SignerSet && c.SignerSet.length > 0 && <span className="tag subdued">{c.SignerSet.length} signers</span>}
            </div>
          </li>
        ))}
      </ul>
      <div className="row">
        <h4 className="tight">Datalink Deliveries</h4>
        <span className="tag subdued">{datalinkState.deliveries.length}</span>
      </div>
      <ul className="wallets">
        {datalinkState.deliveries.map((d: DatalinkDelivery) => (
          <li key={d.ID}>
            <div className="row">
              <div className="mono">{d.ID}</div>
              <span className="tag subdued">{d.Status}</span>
            </div>
            {d.ChannelID && <div className="muted mono">Channel: {d.ChannelID}</div>}
          </li>
        ))}
      </ul>
    </div>
  );
}
