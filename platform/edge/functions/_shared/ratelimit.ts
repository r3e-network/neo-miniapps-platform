import { getEnv, isProductionEnv } from "./env.ts";
import { json } from "./response.ts";
import { supabaseServiceClient, type AuthContext } from "./supabase.ts";

type RateLimitHit = {
  window_start: string;
  request_count: number;
};

function parseIntEnv(name: string): number | undefined {
  const raw = getEnv(name);
  if (!raw) return undefined;
  const n = Number.parseInt(raw, 10);
  return Number.isFinite(n) ? n : undefined;
}

function endpointEnvKey(endpoint: string): string {
  return `EDGE_RATELIMIT_${endpoint.toUpperCase().replaceAll("-", "_")}_PER_MINUTE`;
}

function getRateLimitConfig(endpoint: string): { maxPerMinute: number; windowSeconds: number } {
  const windowSeconds = parseIntEnv("EDGE_RATELIMIT_WINDOW_SECONDS") ?? 60;
  const maxPerMinute =
    parseIntEnv(endpointEnvKey(endpoint)) ??
    parseIntEnv("EDGE_RATELIMIT_DEFAULT_PER_MINUTE") ??
    60;

  return {
    maxPerMinute: Math.max(1, maxPerMinute),
    windowSeconds: Math.max(1, windowSeconds),
  };
}

function getClientIP(req: Request): string | undefined {
  const candidates = [
    req.headers.get("cf-connecting-ip"),
    req.headers.get("x-real-ip"),
    req.headers.get("x-forwarded-for")?.split(",")[0],
  ]
    .map((v) => (v ?? "").trim())
    .filter(Boolean);
  return candidates[0];
}

function rateLimitedResponse(req: Request, params: {
  endpoint: string;
  max: number;
  count: number;
  windowSeconds: number;
  retryAfterSeconds: number;
  resetAtEpochSeconds: number;
}): Response {
  const headers = new Headers();
  headers.set("Retry-After", String(params.retryAfterSeconds));
  headers.set("X-RateLimit-Limit", String(params.max));
  headers.set("X-RateLimit-Remaining", String(Math.max(0, params.max - params.count)));
  headers.set("X-RateLimit-Reset", String(params.resetAtEpochSeconds));
  headers.set("X-RateLimit-Endpoint", params.endpoint);

  return json(
    {
      error: {
        code: "RATE_LIMITED",
        message: `rate limit exceeded for ${params.endpoint}`,
        limit_per_window: params.max,
        window_seconds: params.windowSeconds,
        retry_after_seconds: params.retryAfterSeconds,
      },
    },
    { status: 429, headers },
    req,
  );
}

async function bump(identifier: string, identifierType: string, windowSeconds: number): Promise<RateLimitHit> {
  const supabase = supabaseServiceClient();
  const { data, error } = await supabase.rpc("rate_limit_bump", {
    p_identifier: identifier,
    p_identifier_type: identifierType,
    p_window_seconds: windowSeconds,
  });
  if (error) {
    throw new Error(error.message);
  }
  const row = Array.isArray(data) ? data[0] : data;
  return {
    window_start: String((row as any)?.window_start ?? ""),
    request_count: Number((row as any)?.request_count ?? 0),
  };
}

export async function requireRateLimit(
  req: Request,
  endpoint: string,
  auth?: AuthContext,
): Promise<Response | null> {
  const { maxPerMinute, windowSeconds } = getRateLimitConfig(endpoint);

  let identifierType = "ip";
  let identifierValue = getClientIP(req) ?? "unknown";

  if (auth) {
    if (auth.authType === "api_key" && auth.apiKeyId) {
      identifierType = "api_key";
      identifierValue = auth.apiKeyId;
    } else {
      identifierType = "user";
      identifierValue = auth.userId;
    }
  }

  const identifier = `${identifierType}:${identifierValue}:${endpoint}`;

  let hit: RateLimitHit;
  try {
    hit = await bump(identifier, identifierType, windowSeconds);
  } catch (e) {
    // In local dev, do not hard-fail on missing DB/RPC plumbing.
    if (!isProductionEnv()) return null;
    return json(
      { error: { code: "RATE_LIMIT_UNAVAILABLE", message: (e as Error).message } },
      { status: 503 },
      req,
    );
  }

  const nowMs = Date.now();
  const startMs = Date.parse(hit.window_start);
  const resetMs = (Number.isFinite(startMs) ? startMs : nowMs) + windowSeconds * 1000;
  const retryAfterSeconds = Math.max(0, Math.ceil((resetMs - nowMs) / 1000));
  const resetAtEpochSeconds = Math.ceil(resetMs / 1000);

  if (hit.request_count > maxPerMinute) {
    return rateLimitedResponse(req, {
      endpoint,
      max: maxPerMinute,
      count: hit.request_count,
      windowSeconds,
      retryAfterSeconds,
      resetAtEpochSeconds,
    });
  }

  return null;
}
