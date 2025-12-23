import { handleCorsPreflight } from "../_shared/cors.ts";
import { error, json } from "../_shared/response.ts";
import { requireRateLimit } from "../_shared/ratelimit.ts";
import { requireScope } from "../_shared/scopes.ts";
import { requireAuth } from "../_shared/supabase.ts";
import { queryEvents } from "../_shared/events.ts";

// Lists contract events for MiniApps with optional filtering and pagination.
// Supports polling via after_id parameter for real-time event monitoring.
export async function handler(req: Request): Promise<Response> {
  const preflight = handleCorsPreflight(req);
  if (preflight) return preflight;
  if (req.method !== "GET") return error(405, "method not allowed", "METHOD_NOT_ALLOWED", req);

  const auth = await requireAuth(req);
  if (auth instanceof Response) return auth;
  const rl = await requireRateLimit(req, "events-list", auth);
  if (rl) return rl;
  const scopeCheck = requireScope(req, auth, "events-list");
  if (scopeCheck) return scopeCheck;

  const url = new URL(req.url);
  const appId = url.searchParams.get("app_id") ?? undefined;
  const eventName = url.searchParams.get("event_name") ?? undefined;
  const limit = url.searchParams.get("limit") ?? undefined;
  const afterId = url.searchParams.get("after_id") ?? undefined;

  const result = await queryEvents(
    {
      app_id: appId,
      event_name: eventName,
      limit: limit ? Number.parseInt(limit, 10) : undefined,
      after_id: afterId,
    },
    req,
  );

  if (result instanceof Response) return result;
  return json(result, {}, req);
}

if (import.meta.main) {
  Deno.serve(handler);
}
