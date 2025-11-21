import { CREExecutor, CREPlaybook, CRERun } from "../api";

export type CREState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; executors: CREExecutor[]; playbooks: CREPlaybook[]; runs: CRERun[] }
  | { status: "error"; message: string };

type Props = { creState: CREState | undefined };

export function CREPanel({ creState }: Props) {
  if (!creState || creState.status === "idle") return null;
  if (creState.status === "error") return <p className="error">CRE: {creState.message}</p>;
  if (creState.status === "loading") return <p className="muted">Loading CRE...</p>;

  return (
    <div className="vrf">
      <div className="row">
        <h4 className="tight">CRE Executors</h4>
        <span className="tag subdued">{creState.executors.length}</span>
      </div>
      <ul className="wallets">
        {creState.executors.map((ex: CREExecutor) => (
          <li key={ex.ID}>
            <div className="row">
              <div>
                <strong>{ex.Name}</strong>
                <div className="muted mono">{ex.Type}</div>
              </div>
              <div className="muted mono">{ex.Endpoint}</div>
            </div>
          </li>
        ))}
      </ul>
      <div className="row">
        <h4 className="tight">CRE Playbooks</h4>
        <span className="tag subdued">{creState.playbooks.length}</span>
      </div>
      <ul className="wallets">
        {creState.playbooks.map((pb: CREPlaybook) => (
          <li key={pb.ID}>
            <div className="row">
              <div>
                <strong>{pb.Name}</strong>
                {pb.Description && <div className="muted">{pb.Description}</div>}
              </div>
              {pb.Tags && pb.Tags.length > 0 && <span className="tag subdued">{pb.Tags.join(", ")}</span>}
            </div>
          </li>
        ))}
      </ul>
      <div className="row">
        <h4 className="tight">CRE Runs</h4>
        <span className="tag subdued">{creState.runs.length}</span>
      </div>
      <ul className="wallets">
        {creState.runs.map((run: CRERun) => (
          <li key={run.ID}>
            <div className="row">
              <div className="mono">{run.ID}</div>
              <span className="tag subdued">{run.Status}</span>
            </div>
            <div className="muted mono">Playbook: {run.PlaybookID}</div>
          </li>
        ))}
      </ul>
    </div>
  );
}
