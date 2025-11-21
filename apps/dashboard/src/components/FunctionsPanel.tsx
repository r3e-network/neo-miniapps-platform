import { FunctionExecution, FunctionSummary } from "../api";

export type FunctionsState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; items: { fn: FunctionSummary; executions: FunctionExecution[] }[] }
  | { status: "error"; message: string };

type Props = { functionsState: FunctionsState | undefined };

export function FunctionsPanel({ functionsState }: Props) {
  if (!functionsState || functionsState.status === "idle") return null;
  if (functionsState.status === "error") return <p className="error">Functions: {functionsState.message}</p>;
  if (functionsState.status === "loading") return <p className="muted">Loading functions...</p>;

  return (
    <div className="vrf">
      <div className="row">
        <h4 className="tight">Functions</h4>
        <span className="tag subdued">{functionsState.items.length}</span>
      </div>
      <ul className="wallets">
        {functionsState.items.map(({ fn, executions }) => (
          <li key={fn.ID}>
            <div className="row">
              <div>
                <strong>{fn.Name}</strong>
                <div className="muted mono">{fn.Runtime}</div>
              </div>
              {fn.Status && <span className="tag subdued">{fn.Status}</span>}
            </div>
            {executions.length > 0 ? (
              <ul className="list mono">
                {executions.map((ex: FunctionExecution) => (
                  <li key={ex.ID} className="row">
                    <span>{ex.ID}</span>
                    <span className="tag subdued">{ex.Status}</span>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="muted">No executions yet.</p>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}
