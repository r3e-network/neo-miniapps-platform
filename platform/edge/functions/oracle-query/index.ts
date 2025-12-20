import { handleCorsPreflight } from "../_shared/cors.ts";
import { mustGetEnv } from "../_shared/env.ts";
import { error, json } from "../_shared/response.ts";
import { requireRateLimit } from "../_shared/ratelimit.ts";
import { requireHostScope } from "../_shared/scopes.ts";
import { requireAuth } from "../_shared/supabase.ts";
import { postJSON } from "../_shared/tee.ts";

type OracleQueryRequest = {
  url: string;
  method?: string;
  headers?: Record<string, string>;
  secret_name?: string;
  secret_as_key?: string;
  body?: string;
};

// Thin gateway to the NeoOracle service (/query):
// - validates auth + basic shape
// - forwards to the TEE service over optional mTLS
export async function handler(req: Request): Promise<Response> {
  const preflight = handleCorsPreflight(req);
  if (preflight) return preflight;
  if (req.method !== "POST") return error(405, "method not allowed", "METHOD_NOT_ALLOWED", req);

  const auth = await requireAuth(req);
  if (auth instanceof Response) return auth;
  const rl = await requireRateLimit(req, "oracle-query", auth);
  if (rl) return rl;
  const scopeCheck = requireHostScope(req, auth, "oracle-query");
  if (scopeCheck) return scopeCheck;

  let body: OracleQueryRequest;
  try {
    body = await req.json();
  } catch {
    return error(400, "invalid JSON body", "BAD_JSON", req);
  }

  const url = String(body.url ?? "").trim();
  if (!url) return error(400, "url required", "URL_REQUIRED", req);

  const neooracleURL = mustGetEnv("NEOORACLE_URL").replace(/\/$/, "");
  const result = await postJSON(
    `${neooracleURL}/query`,
    {
      url,
      method: body.method,
      headers: body.headers,
      secret_name: body.secret_name,
      secret_as_key: body.secret_as_key,
      body: body.body,
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
