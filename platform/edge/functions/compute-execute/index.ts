import { handleCorsPreflight } from "../_shared/cors.ts";
import { mustGetEnv } from "../_shared/env.ts";
import { error, json } from "../_shared/response.ts";
import { requireRateLimit } from "../_shared/ratelimit.ts";
import { requireHostScope } from "../_shared/scopes.ts";
import { requireAuth, requirePrimaryWallet } from "../_shared/supabase.ts";
import { postJSON } from "../_shared/tee.ts";

type ComputeExecuteRequest = {
  script: string;
  entry_point?: string;
  input?: Record<string, unknown>;
  secret_refs?: string[];
  timeout?: number;
};

// Thin gateway to the NeoCompute service (/execute):
// - validates auth + wallet binding + basic shape
// - forwards to the TEE service over optional mTLS
export async function handler(req: Request): Promise<Response> {
  const preflight = handleCorsPreflight(req);
  if (preflight) return preflight;
  if (req.method !== "POST") return error(405, "method not allowed", "METHOD_NOT_ALLOWED", req);

  const auth = await requireAuth(req);
  if (auth instanceof Response) return auth;
  const rl = await requireRateLimit(req, "compute-execute", auth);
  if (rl) return rl;
  const scopeCheck = requireHostScope(req, auth, "compute-execute");
  if (scopeCheck) return scopeCheck;
  const walletCheck = await requirePrimaryWallet(auth.userId, req);
  if (walletCheck instanceof Response) return walletCheck;

  let body: ComputeExecuteRequest;
  try {
    body = await req.json();
  } catch {
    return error(400, "invalid JSON body", "BAD_JSON", req);
  }

  const script = String(body.script ?? "").trim();
  if (!script) return error(400, "script required", "SCRIPT_REQUIRED", req);

  const neocomputeURL = mustGetEnv("NEOCOMPUTE_URL").replace(/\/$/, "");
  const result = await postJSON(
    `${neocomputeURL}/execute`,
    {
      script,
      entry_point: body.entry_point,
      input: body.input,
      secret_refs: body.secret_refs,
      timeout: body.timeout,
    },
    { "X-User-ID": auth.userId },
    req,
  );
  if (result instanceof Response) return result;
  return json(result, {}, req);
}

if (import.meta.main) {
  Deno.serve(handler);
}
