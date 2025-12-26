import { handleCorsPreflight } from "../_shared/cors.ts";
import { error, json } from "../_shared/response.ts";
import { createClient } from "../_shared/supabase.ts";

export async function handler(req: Request): Promise<Response> {
  const preflight = handleCorsPreflight(req);
  if (preflight) return preflight;
  if (req.method !== "GET") return error(405, "method not allowed", "METHOD_NOT_ALLOWED", req);

  const url = new URL(req.url);
  const appId = url.searchParams.get("app_id");

  const supabase = createClient();

  if (appId) {
    // Single app stats
    const { data, error: err } = await supabase.from("miniapp_stats").select("*").eq("app_id", appId).single();

    if (err) return error(404, "app not found", "NOT_FOUND", req);
    return json(data, req);
  }

  // All apps stats
  const { data, error: err } = await supabase
    .from("miniapp_stats")
    .select("*")
    .order("total_transactions", { ascending: false })
    .limit(50);

  if (err) return error(500, err.message, "DB_ERROR", req);
  return json({ stats: data }, req);
}

Deno.serve(handler);
