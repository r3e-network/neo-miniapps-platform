import { AutomationJob, Trigger } from "../api";

export type AutomationState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; jobs: AutomationJob[]; triggers: Trigger[] }
  | { status: "error"; message: string };

type Props = { automationState: AutomationState | undefined };

export function AutomationPanel({ automationState }: Props) {
  if (!automationState || automationState.status === "idle") return null;
  if (automationState.status === "error") return <p className="error">Automation: {automationState.message}</p>;
  if (automationState.status === "loading") return <p className="muted">Loading automation...</p>;

  return (
    <div className="vrf">
      <div className="row">
        <h4 className="tight">Automation Jobs</h4>
        <span className="tag subdued">{automationState.jobs.length}</span>
      </div>
      <ul className="wallets">
        {automationState.jobs.map((job: AutomationJob) => (
          <li key={job.ID}>
            <div className="row">
              <div>
                <strong>{job.Name}</strong>
                <div className="muted mono">{job.Schedule}</div>
              </div>
              <span className="tag subdued">{job.Enabled ? "enabled" : "disabled"}</span>
            </div>
          </li>
        ))}
      </ul>
      <div className="row">
        <h4 className="tight">Triggers</h4>
        <span className="tag subdued">{automationState.triggers.length}</span>
      </div>
      <ul className="wallets">
        {automationState.triggers.map((tr: Trigger) => (
          <li key={tr.ID}>
            <div className="row">
              <div className="mono">{tr.ID}</div>
              <span className="tag subdued">{tr.Type}</span>
            </div>
            <div className="muted mono">{tr.Rule}</div>
          </li>
        ))}
      </ul>
    </div>
  );
}
