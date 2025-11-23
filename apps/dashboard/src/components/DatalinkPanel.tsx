import { useState } from "react";
import { DatalinkChannel, DatalinkDelivery } from "../api";

export type DatalinkState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; channels: DatalinkChannel[]; deliveries: DatalinkDelivery[] }
  | { status: "error"; message: string };

type Props = {
  datalinkState: DatalinkState | undefined;
  onCreateChannel: (payload: { name: string; endpoint: string; signers: string[]; status?: string; metadata?: Record<string, string> }) => void;
  onCreateDelivery: (payload: { channelId: string; body: Record<string, any>; metadata?: Record<string, string> }) => void;
  onNotify?: (type: "success" | "error", message: string) => void;
};

export function DatalinkPanel({ datalinkState, onCreateChannel, onCreateDelivery, onNotify }: Props) {
  const [channelName, setChannelName] = useState("");
  const [channelEndpoint, setChannelEndpoint] = useState("");
  const [channelSigners, setChannelSigners] = useState("");
  const [channelMetadata, setChannelMetadata] = useState("");
  const [deliveryChannel, setDeliveryChannel] = useState("");
  const [deliveryPayload, setDeliveryPayload] = useState('{"data":"hello"}');
  const [deliveryMetadata, setDeliveryMetadata] = useState("");
  const [error, setError] = useState<string | undefined>();
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string | undefined>();

  if (!datalinkState || datalinkState.status === "idle") return null;
  if (datalinkState.status === "error") return <p className="error">Datalink: {datalinkState.message}</p>;
  if (datalinkState.status === "loading") return <p className="muted">Loading link...</p>;

  return (
    <div className="vrf">
      <div className="row">
        <h4 className="tight">Datalink Channels</h4>
        <span className="tag subdued">{datalinkState.channels.length}</span>
      </div>
      {message && <p className="muted">{message}</p>}
      {error && <p className="error">{error}</p>}
      <ul className="wallets">
        {datalinkState.channels.map((c: DatalinkChannel) => {
          const signers = c.SignerSet && c.SignerSet.length > 0 ? c.SignerSet.join(", ") : "none";
          return (
            <li key={c.ID}>
              <div className="row">
                <div>
                  <strong>{c.Name}</strong> {c.Status && <span className="tag subdued">{c.Status}</span>}
                  <div className="muted mono">{c.Endpoint}</div>
                  <div className="muted mono">Signers: {signers}</div>
                  {c.Metadata && Object.keys(c.Metadata).length > 0 && (
                    <div className="muted mono">
                      Meta:{" "}
                      {Object.entries(c.Metadata)
                        .map(([k, v]) => `${k}=${v}`)
                        .join(", ")}
                    </div>
                  )}
                </div>
                {c.SignerSet && c.SignerSet.length > 0 && <span className="tag subdued">{c.SignerSet.length}</span>}
              </div>
            </li>
          );
        })}
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
            {d.Metadata && Object.keys(d.Metadata).length > 0 && (
              <div className="muted mono">
                Meta:{" "}
                {Object.entries(d.Metadata)
                  .map(([k, v]) => `${k}=${v}`)
                  .join(", ")}
              </div>
            )}
          </li>
        ))}
      </ul>
      <div className="row">
        <h4 className="tight">Create Channel</h4>
      </div>
      <div className="form-grid">
        <input value={channelName} onChange={(e) => setChannelName(e.target.value)} placeholder="Name" />
        <input value={channelEndpoint} onChange={(e) => setChannelEndpoint(e.target.value)} placeholder="Endpoint" />
        <input value={channelSigners} onChange={(e) => setChannelSigners(e.target.value)} placeholder="Signer set (comma, required)" />
        <input value={channelMetadata} onChange={(e) => setChannelMetadata(e.target.value)} placeholder='Metadata JSON {"tier":"gold"}' />
        <button
          type="button"
          onClick={async () => {
            setError(undefined);
            setMessage(undefined);
            try {
              setBusy(true);
              const meta = channelMetadata.trim() ? (JSON.parse(channelMetadata) as Record<string, string>) : undefined;
              const signers = channelSigners.split(",").map((s) => s.trim()).filter(Boolean);
              if (signers.length === 0) {
                throw new Error("Signer set is required");
              }
              await onCreateChannel({ name: channelName, endpoint: channelEndpoint, signers, metadata: meta });
              setChannelName("");
              setChannelEndpoint("");
              setChannelSigners("");
              setChannelMetadata("");
              setMessage("Channel created");
              onNotify?.("success", "Channel created");
            } catch (err) {
              const msg = err instanceof Error ? err.message : String(err);
              setError(msg);
              onNotify?.("error", msg);
            } finally {
              setBusy(false);
            }
          }}
          disabled={busy}
        >
          Create
        </button>
      </div>
      <div className="row">
        <h4 className="tight">Queue Delivery</h4>
      </div>
      <div className="form-grid">
        <input value={deliveryChannel} onChange={(e) => setDeliveryChannel(e.target.value)} placeholder="Channel ID" />
        <input value={deliveryPayload} onChange={(e) => setDeliveryPayload(e.target.value)} placeholder='Payload JSON {"data":"hello"}' />
        <input value={deliveryMetadata} onChange={(e) => setDeliveryMetadata(e.target.value)} placeholder='Metadata JSON {"trace":"abc"}' />
        <button
          type="button"
          onClick={async () => {
            setError(undefined);
            setMessage(undefined);
            try {
              setBusy(true);
              const payload = deliveryPayload.trim() ? (JSON.parse(deliveryPayload) as Record<string, any>) : {};
              const meta = deliveryMetadata.trim() ? (JSON.parse(deliveryMetadata) as Record<string, string>) : undefined;
              await onCreateDelivery({ channelId: deliveryChannel, body: payload, metadata: meta });
              setDeliveryPayload('{"data":"hello"}');
              setDeliveryMetadata("");
              setMessage("Delivery queued");
              onNotify?.("success", "Delivery queued");
            } catch (err) {
              const msg = err instanceof Error ? err.message : String(err);
              setError(msg);
              onNotify?.("error", msg);
            } finally {
              setBusy(false);
            }
          }}
          disabled={busy}
        >
          Submit
        </button>
      </div>
    </div>
  );
}
