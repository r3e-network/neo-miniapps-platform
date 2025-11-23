import { useEffect, useState } from "react";
import { AuditEntry, ClientConfig, fetchAudit } from "../api";
import { SystemState } from "../hooks/useSystemInfo";

type Props = {
  systemState: SystemState;
  baseUrl: string;
  token: string;
};

export function AdminPanel({ systemState, baseUrl, token }: Props) {
  const [audit, setAudit] = useState<AuditEntry[]>([]);
  const [auditError, setAuditError] = useState<string>();

  useEffect(() => {
    if (!baseUrl || !token) return;
    const config: ClientConfig = { baseUrl, token };
    void fetchAudit(config, 200)
      .then(setAudit)
      .catch((err) => setAuditError(err instanceof Error ? err.message : String(err)));
  }, [baseUrl, token]);

  if (systemState.status !== "ready") return null;
  const { descriptors, version, jam, accounts } = systemState;
  const services = descriptors ?? [];
  const statusUrl = `${baseUrl}/system/status?token=${encodeURIComponent(token)}`;
  const metricsUrl = `${baseUrl}/metrics`;
  const versionUrl = `${baseUrl}/system/version?token=${encodeURIComponent(token)}`;

  return (
    <section className="card inner">
      <div className="row">
        <h3>Admin overview</h3>
        <span className="tag subdued">services {services.length}</span>
      </div>
      <p className="muted">
        Operator view across the platform. Use the links below for quick validation and monitoring. Admin actions (quotas, feature flags, tenants)
        can be added as the next step.
      </p>
      <div className="grid two-cols">
        <div className="card">
          <h4>Platform health</h4>
          <ul className="links">
            <li>
              <a href={statusUrl} target="_blank" rel="noreferrer">
                System status (auth: token query)
              </a>
            </li>
            <li>
              <a href={versionUrl} target="_blank" rel="noreferrer">
                Version & build
              </a>
            </li>
            <li>
              <a href={metricsUrl} target="_blank" rel="noreferrer">
                Prometheus metrics (bearer token required)
              </a>
            </li>
          </ul>
          <div className="muted mono">Version: {version || "unknown"}</div>
          <div className="muted mono">Accounts/projects: {accounts.length}</div>
        </div>
        <div className="card">
          <h4>JAM / services</h4>
          <div className="muted mono">JAM enabled: {jam?.enabled ? "true" : "false"}</div>
          <div className="muted mono">JAM store: {jam?.store || "memory"}</div>
          <div className="muted mono">Services advertised: {services.length}</div>
          <ul className="links">
            {services.slice(0, 8).map((s) => (
              <li key={`${s.domain}/${s.name}`} className="muted mono">
                {s.domain}/{s.name} ({s.layer})
              </li>
            ))}
          </ul>
        </div>
      </div>
      <div className="card">
        <div className="row">
          <h4>Recent API audit</h4>
          <span className="tag subdued">{audit.length}</span>
        </div>
        <p className="muted">
          Requires admin role (JWT). Use <code>admin/changeme</code> in local compose via <code>/auth/login</code>. Token-only auth is not admin.
        </p>
        {auditError && <p className="error">Audit error: {auditError}</p>}
        {!auditError && audit.length === 0 && <p className="muted">No entries yet.</p>}
        {audit.length > 0 && (
          <div className="table">
            <div className="table-head">
              <span>Time</span>
              <span>User</span>
              <span>Role</span>
              <span>Tenant</span>
              <span>IP</span>
              <span>Method</span>
              <span>Path</span>
              <span>Status</span>
            </div>
            {audit.slice(-20).reverse().map((entry, idx) => (
              <div key={`${entry.time}-${idx}`} className="table-row">
                <span className="mono">{new Date(entry.time).toLocaleString()}</span>
                <span className="mono">{entry.user || "-"}</span>
                <span className="mono">{entry.role || "-"}</span>
                <span className="mono">{entry.tenant || "-"}</span>
                <span className="mono">{entry.remote_addr || "-"}</span>
                <span className="mono">{entry.method}</span>
                <span className="mono">{entry.path}</span>
                <span className="mono">{entry.status}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </section>
  );
}
