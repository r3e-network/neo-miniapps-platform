import { FormEvent } from "react";

type Props = {
  baseUrl: string;
  token: string;
  promBase: string;
  canQuery: boolean;
  status: "idle" | "loading" | "ready" | "error";
  onSubmit: (event: FormEvent) => void;
  onBaseUrlChange: (value: string) => void;
  onTokenChange: (value: string) => void;
  onPromChange: (value: string) => void;
};

export function SettingsForm({ baseUrl, token, promBase, canQuery, status, onSubmit, onBaseUrlChange, onTokenChange, onPromChange }: Props) {
  return (
    <form className="settings" onSubmit={onSubmit}>
      <label>
        API Base URL
        <input value={baseUrl} onChange={(e) => onBaseUrlChange(e.target.value)} placeholder="http://localhost:8080" />
        <span className="hint">Point at the appserver HTTP endpoint.</span>
      </label>
      <label>
        API Token
        <input value={token} onChange={(e) => onTokenChange(e.target.value)} placeholder="Bearer token" />
        <span className="hint">Use the same bearer token you send to the API.</span>
      </label>
      <label>
        Prometheus URL
        <input value={promBase} onChange={(e) => onPromChange(e.target.value)} placeholder="http://localhost:9090" />
        <span className="hint">Optional: needed for the metrics cards.</span>
      </label>
      <button type="submit" disabled={!canQuery || status === "loading"}>
        {status === "loading" ? "Loading..." : "Connect"}
      </button>
    </form>
  );
}
