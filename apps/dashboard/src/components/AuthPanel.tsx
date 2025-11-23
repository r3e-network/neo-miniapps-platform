import { useState } from "react";

type Props = {
  baseUrl: string;
  onLoggedIn: (token: string, role?: string) => void;
};

export function AuthPanel({ baseUrl, onLoggedIn }: Props) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [message, setMessage] = useState<string>();
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  const canLogin = baseUrl.trim().length > 0 && username.trim().length > 0 && password.trim().length > 0;

  return (
    <section className="card inner">
      <div className="row">
        <h3>Login (JWT)</h3>
        <span className="tag subdued">/auth/login</span>
      </div>
      <p className="muted">
        Production auth: use configured users (env <code>AUTH_USERS</code>) to obtain a JWT. Token auth remains supported for backwards
        compatibility. Local defaults: <code>admin/changeme</code> with <code>AUTH_JWT_SECRET</code> set in <code>.env</code>.
      </p>
      {message && <p className="muted">{message}</p>}
      {error && <p className="error">{error}</p>}
      <div className="form-grid">
        <input value={username} onChange={(e) => setUsername(e.target.value)} placeholder="Username" />
        <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="Password" />
        <button
          type="button"
          disabled={!canLogin || busy}
          onClick={async () => {
            setError(undefined);
            setMessage(undefined);
            try {
              setBusy(true);
              const resp = await fetch(`${baseUrl}/auth/login`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ username, password }),
              });
              if (!resp.ok) {
                const text = await resp.text();
                throw new Error(text || `login failed (${resp.status})`);
              }
              const data = (await resp.json()) as { token: string; role?: string; expires_at?: string };
              onLoggedIn(data.token, data.role);
              setMessage(`Logged in as ${username}${data.role ? ` (${data.role})` : ""}`);
            } catch (err) {
              setError(err instanceof Error ? err.message : String(err));
            } finally {
              setBusy(false);
            }
          }}
        >
          {busy ? "Logging in..." : "Login"}
        </button>
      </div>
    </section>
  );
}
