import { Datastream, DatastreamFrame } from "../api";

export type DatastreamsState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; streams: Datastream[]; frames: Record<string, DatastreamFrame[]> }
  | { status: "error"; message: string };

type Props = {
  datastreamsState: DatastreamsState | undefined;
};

export function DatastreamsPanel({ datastreamsState }: Props) {
  if (!datastreamsState || datastreamsState.status === "idle") return null;
  if (datastreamsState.status === "error") return <p className="error">Datastreams: {datastreamsState.message}</p>;
  if (datastreamsState.status === "loading") return <p className="muted">Loading streams...</p>;

  return (
    <div className="vrf">
      <div className="row">
        <h4 className="tight">Datastreams</h4>
        <span className="tag subdued">{datastreamsState.streams.length}</span>
      </div>
      <ul className="wallets">
        {datastreamsState.streams.map((s: Datastream) => (
          <li key={s.ID}>
            <div className="row">
              <div>
                <strong>{s.Name}</strong> <span className="muted">{s.Frequency}</span>
                <div className="muted mono">{s.Symbol}</div>
              </div>
              <span className="tag subdued">{s.Status || "unknown"}</span>
            </div>
            {datastreamsState.frames[s.ID]?.length ? (
              <div className="muted mono">
                Latest seq {datastreamsState.frames[s.ID][0]?.Sequence} — latency {datastreamsState.frames[s.ID][0]?.LatencyMs ?? "n/a"}ms
              </div>
            ) : (
              <div className="muted">No frames yet.</div>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}
