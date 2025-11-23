import { useState } from "react";
import { jamSubmitPackage, jamUploadPreimage } from "../api";

type Props = {
  baseUrl: string;
  token: string;
  onNotify?: (type: "success" | "error", message: string) => void;
};

async function sha256Hex(input: string): Promise<string> {
  const data = new TextEncoder().encode(input);
  const digest = await crypto.subtle.digest("SHA-256", data);
  return Array.from(new Uint8Array(digest))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

export function JamPanel({ baseUrl, token, onNotify }: Props) {
  const [preimage, setPreimage] = useState("hello jam");
  const [preimageHash, setPreimageHash] = useState<string>("");
  const [pkgService, setPkgService] = useState("demo");
  const [pkgKind, setPkgKind] = useState("example");
  const [pkgParamsHash, setPkgParamsHash] = useState("");
  const [pkgPreimages, setPkgPreimages] = useState("");
  const [message, setMessage] = useState<string | undefined>();
  const [error, setError] = useState<string | undefined>();
  const [busy, setBusy] = useState(false);

  const config = { baseUrl, token };

  return (
    <div className="card inner">
      <h3>JAM Actions</h3>
      {message && <p className="muted">{message}</p>}
      {error && <p className="error">{error}</p>}
      <div className="row">
        <h4 className="tight">Upload Preimage</h4>
      </div>
      <textarea value={preimage} onChange={(e) => setPreimage(e.target.value)} rows={3} />
      <button
        type="button"
        onClick={async () => {
          setError(undefined);
          try {
            setBusy(true);
            const hash = await sha256Hex(preimage);
            const data = new TextEncoder().encode(preimage);
            await jamUploadPreimage(config, hash, data.buffer, "text/plain");
            setPreimageHash(hash);
            setMessage(`Uploaded preimage ${hash}`);
            onNotify?.("success", `Uploaded preimage ${hash}`);
          } catch (err) {
            setError(err instanceof Error ? err.message : String(err));
            onNotify?.("error", err instanceof Error ? err.message : String(err));
          } finally {
            setBusy(false);
          }
        }}
        disabled={busy}
      >
        Upload
      </button>

      <div className="row">
        <h4 className="tight">Submit Package</h4>
      </div>
      <div className="form-grid">
        <input value={pkgService} onChange={(e) => setPkgService(e.target.value)} placeholder="Service ID" />
        <input value={pkgKind} onChange={(e) => setPkgKind(e.target.value)} placeholder="Kind" />
        <input value={pkgParamsHash} onChange={(e) => setPkgParamsHash(e.target.value)} placeholder="Params hash" />
        <input
          value={pkgPreimages}
          onChange={(e) => setPkgPreimages(e.target.value)}
          placeholder="Preimage hashes (comma, optional)"
        />
        <button
          type="button"
        onClick={async () => {
          setError(undefined);
          try {
            setBusy(true);
            const payload: any = {
              service_id: pkgService,
              items: [{ kind: pkgKind, params_hash: pkgParamsHash || preimageHash }],
            };
            const hashes = pkgPreimages
              .split(",")
              .map((h) => h.trim())
              .filter(Boolean);
            if (hashes.length > 0) {
              payload.preimage_hashes = hashes;
            } else if (preimageHash) {
              payload.preimage_hashes = [preimageHash];
            }
            await jamSubmitPackage(config, payload);
            setMessage("Package submitted");
            onNotify?.("success", "Package submitted");
          } catch (err) {
            setError(err instanceof Error ? err.message : String(err));
            onNotify?.("error", err instanceof Error ? err.message : String(err));
          } finally {
            setBusy(false);
          }
        }}
        disabled={busy}
        >
          Use last preimage
        </button>
        <button
          type="button"
        onClick={async () => {
          setError(undefined);
          try {
            setBusy(true);
            if (!pkgParamsHash) {
              setError("params hash required");
              onNotify?.("error", "params hash required");
              return;
            }
            const payload: any = {
              service_id: pkgService,
                items: [{ kind: pkgKind, params_hash: pkgParamsHash }],
              };
              const hashes = pkgPreimages
                .split(",")
                .map((h) => h.trim())
                .filter(Boolean);
              if (hashes.length > 0) {
                payload.preimage_hashes = hashes;
              } else if (preimageHash) {
                payload.preimage_hashes = [preimageHash];
              }
              await jamSubmitPackage(config, payload);
              setMessage("Package submitted");
              onNotify?.("success", "Package submitted");
            } catch (err) {
              setError(err instanceof Error ? err.message : String(err));
              onNotify?.("error", err instanceof Error ? err.message : String(err));
            } finally {
              setBusy(false);
            }
          }}
          disabled={busy}
        >
          Submit Package
        </button>
      </div>
    </div>
  );
}
