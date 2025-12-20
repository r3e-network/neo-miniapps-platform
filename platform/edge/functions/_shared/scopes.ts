import { isProductionEnv } from "./env.ts";
import { error } from "./response.ts";
import type { AuthContext } from "./supabase.ts";

// API key scopes are optional (backward compatible). If a key has no scopes,
// it is treated as "full access" within the authenticated user context.
//
// If scopes are provided, endpoints should call `requireScope(...)` with the
// Edge function name (e.g. "pay-gas") to enforce least privilege.

export function requireScopes(req: Request, auth: AuthContext, requiredScopes: string[]): Response | null {
  if (auth.authType !== "api_key") return null;

  const scopes = Array.isArray(auth.scopes) ? auth.scopes : [];
  if (scopes.length === 0) return null; // default: allow all (legacy keys)
  if (scopes.includes("*")) return null;

  const missing = requiredScopes.filter((scope) => !scopes.includes(scope));
  if (missing.length === 0) return null;

  return error(403, `api key missing required scope(s): ${missing.join(", ")}`, "SCOPE_REQUIRED", req);
}

export function requireScope(req: Request, auth: AuthContext, requiredScope: string): Response | null {
  return requireScopes(req, auth, [requiredScope]);
}

// Host-gated endpoints require API keys with explicit scopes. In production,
// bearer tokens are rejected to prevent direct MiniApp access.
export function requireHostScopes(req: Request, auth: AuthContext, requiredScopes: string[]): Response | null {
  if (auth.authType === "api_key") {
    const scopes = Array.isArray(auth.scopes) ? auth.scopes : [];
    if (scopes.length === 0) {
      return error(403, `api key missing required scope(s): ${requiredScopes.join(", ")}`, "SCOPE_REQUIRED", req);
    }
    if (scopes.includes("*")) return null;
    const missing = requiredScopes.filter((scope) => !scopes.includes(scope));
    if (missing.length === 0) return null;
    return error(403, `api key missing required scope(s): ${missing.join(", ")}`, "SCOPE_REQUIRED", req);
  }

  if (isProductionEnv()) {
    return error(403, "api key required for host-only endpoint", "API_KEY_REQUIRED", req);
  }

  return null;
}

export function requireHostScope(req: Request, auth: AuthContext, requiredScope: string): Response | null {
  return requireHostScopes(req, auth, [requiredScope]);
}
