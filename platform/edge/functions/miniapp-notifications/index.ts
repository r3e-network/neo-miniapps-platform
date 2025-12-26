import { handleCorsPreflight } from "../_shared/cors.ts";
import { error, json } from "../_shared/response.ts";
import { createClient } from "../_shared/supabase.ts";

export async function handler(req: Request): Promise<Response> {
  const preflight = handleCorsPreflight(req);
  if (preflight) return preflight;
  if (req.method !== "GET") return error(405, "method not allowed", "METHOD_NOT_ALLOWED", req);

  const url = new URL(req.url);
  const appId = url.searchParams.get("app_id");
  const limit = Math.min(parseInt(url.searchParams.get("limit") || "20"), 100);

  const supabase = createClient();
  let query = supabase.from("miniapp_notifications").select("*").order("created_at", { ascending: false }).limit(limit);

  if (appId) {
    query = query.eq("app_id", appId);
  }

  const { data, error: err } = await query;
  if (err) return error(500, err.message, "DB_ERROR", req);

  return json({ notifications: data }, req);
}

Deno.serve(handler);
