import { parseDecimalToInt } from "./amount.ts";
import { isProductionEnv } from "./env.ts";
import { canonicalizeMiniAppManifest, enforceMiniAppAssetPolicy, type MiniAppManifestCore } from "./manifest.ts";
import { error } from "./response.ts";
import { supabaseServiceClient } from "./supabase.ts";

export type MiniAppPolicy = {
  appId: string;
  manifestHash: string;
  status: string;
  permissions: Record<string, unknown>;
  limits: {
    maxGasPerTx?: bigint;
    dailyGasCapPerUser?: bigint;
    governanceCap?: bigint;
  };
};

type UsageCapInput = {
  appId: string;
  userId: string;
  gasDelta?: bigint;
  governanceDelta?: bigint;
  gasCap?: bigint;
  governanceCap?: bigint;
  req?: Request;
};

type MiniAppRow = {
  app_id: string;
  developer_user_id: string;
  manifest_hash: string;
  manifest: Record<string, unknown>;
  status: string;
};

function parseGasLimit(raw: unknown, label: string): bigint | undefined {
  const value = String(raw ?? "").trim();
  if (!value) return undefined;
  const parsed = parseDecimalToInt(value, 8);
  if (parsed <= 0n) {
    throw new Error(`${label} must be > 0`);
  }
  return parsed;
}

function parseNeoLimit(raw: unknown, label: string): bigint | undefined {
  const value = String(raw ?? "").trim();
  if (!value) return undefined;
  if (!/^\d+$/.test(value)) {
    throw new Error(`${label} must be an integer string`);
  }
  const parsed = BigInt(value);
  if (parsed <= 0n) {
    throw new Error(`${label} must be > 0`);
  }
  return parsed;
}

export function permissionEnabled(permissions: Record<string, unknown> | undefined, key: string): boolean {
  if (!permissions) return false;
  const value = permissions[key];
  if (typeof value === "boolean") return value;
  if (Array.isArray(value)) return value.length > 0;
  return false;
}

export async function enforceUsageCaps(input: UsageCapInput): Promise<Response | null> {
  const hasGas = typeof input.gasCap === "bigint" && input.gasCap > 0n;
  const hasGovernance = typeof input.governanceCap === "bigint" && input.governanceCap > 0n;
  if (!hasGas && !hasGovernance) return null;

  const supabase = supabaseServiceClient();
  const { error: bumpErr } = await supabase.rpc("miniapp_usage_bump", {
    p_user_id: input.userId,
    p_app_id: input.appId,
    p_gas_delta: input.gasDelta ? input.gasDelta.toString() : "0",
    p_governance_delta: input.governanceDelta ? input.governanceDelta.toString() : "0",
    p_gas_cap: hasGas ? input.gasCap?.toString() : null,
    p_governance_cap: hasGovernance ? input.governanceCap?.toString() : null,
  });

  if (bumpErr) {
    const message = bumpErr.message ?? "usage cap enforcement failed";
    if (message.toLowerCase().includes("cap_exceeded")) {
      return error(403, "usage cap exceeded", "LIMIT_EXCEEDED", input.req);
    }
    if (!isProductionEnv()) return null;
    return error(503, `usage tracking unavailable: ${message}`, "USAGE_UNAVAILABLE", input.req);
  }

  return null;
}

export async function upsertMiniAppManifest(input: {
  core: MiniAppManifestCore;
  canonicalManifest: Record<string, unknown>;
  developerUserId: string;
  mode: "register" | "update";
  req?: Request;
}): Promise<Response | null> {
  const supabase = supabaseServiceClient();

  const { data: existing, error: loadErr } = await supabase
    .from("miniapps")
    .select("app_id,developer_user_id")
    .eq("app_id", input.core.appId)
    .maybeSingle();

  if (loadErr) return error(500, `failed to load app registry: ${loadErr.message}`, "DB_ERROR", input.req);

  if (existing) {
    if (String((existing as any)?.developer_user_id ?? "") !== input.developerUserId) {
      return error(403, "app_id already registered by another developer", "APP_OWNER_MISMATCH", input.req);
    }
    if (input.mode === "register") {
      return error(409, "app_id already registered", "APP_ALREADY_REGISTERED", input.req);
    }
  } else if (input.mode === "update") {
    return error(404, "app_id not registered", "APP_NOT_FOUND", input.req);
  }

  let canonical: Record<string, unknown>;
  try {
    enforceMiniAppAssetPolicy(input.canonicalManifest);
    canonical = canonicalizeMiniAppManifest(input.canonicalManifest);
  } catch (e) {
    return error(400, (e as Error).message, "MANIFEST_INVALID", input.req);
  }
  const permissions = (canonical.permissions as Record<string, unknown> | undefined) ?? {};
  const limits = (canonical.limits as Record<string, unknown> | undefined) ?? {};
  const assetsAllowed = Array.isArray(canonical.assets_allowed) ? (canonical.assets_allowed as string[]) : [];
  const governanceAssetsAllowed = Array.isArray(canonical.governance_assets_allowed)
    ? (canonical.governance_assets_allowed as string[])
    : [];

  const payload: Record<string, unknown> = {
    app_id: input.core.appId,
    developer_user_id: input.developerUserId,
    manifest_hash: input.core.manifestHashHex,
    entry_url: input.core.entryUrl,
    developer_pubkey: input.core.developerPubKeyHex,
    manifest: canonical,
    permissions,
    limits,
    assets_allowed: assetsAllowed,
    governance_assets_allowed: governanceAssetsAllowed,
    updated_at: new Date().toISOString(),
  };
  if (!existing) {
    payload.status = "active";
  }

  const { error: upsertErr } = await supabase
    .from("miniapps")
    .upsert(payload, { onConflict: "app_id" });

  if (upsertErr) {
    return error(500, `failed to store miniapp manifest: ${upsertErr.message}`, "DB_ERROR", input.req);
  }

  return null;
}

export async function fetchMiniAppPolicy(appId: string, req?: Request): Promise<MiniAppPolicy | Response | null> {
  const supabase = supabaseServiceClient();
  const { data, error: loadErr } = await supabase
    .from("miniapps")
    .select("app_id,manifest_hash,manifest,status")
    .eq("app_id", appId)
    .maybeSingle();

  if (loadErr) return error(500, `failed to load app registry: ${loadErr.message}`, "DB_ERROR", req);

  if (!data) {
    if (isProductionEnv()) {
      return error(404, "app_id not registered", "APP_NOT_FOUND", req);
    }
    return null;
  }

  const row = data as MiniAppRow;
  const status = String(row.status ?? "").toLowerCase();
  if (status && status !== "active") {
    return error(403, "app is not active", "APP_INACTIVE", req);
  }

  let canonical: Record<string, unknown>;
  try {
    enforceMiniAppAssetPolicy(row.manifest ?? {});
    canonical = canonicalizeMiniAppManifest(row.manifest ?? {});
  } catch (e) {
    const detail = (e as Error).message;
    return error(500, `stored manifest invalid: ${detail}`, "APP_MANIFEST_INVALID", req);
  }

  const permissions = (canonical.permissions as Record<string, unknown> | undefined) ?? {};
  const limitsRaw = (canonical.limits as Record<string, unknown> | undefined) ?? {};

  let limits: MiniAppPolicy["limits"];
  try {
    limits = {
      maxGasPerTx: parseGasLimit(limitsRaw.max_gas_per_tx, "manifest.limits.max_gas_per_tx"),
      dailyGasCapPerUser: parseGasLimit(limitsRaw.daily_gas_cap_per_user, "manifest.limits.daily_gas_cap_per_user"),
      governanceCap: parseNeoLimit(limitsRaw.governance_cap, "manifest.limits.governance_cap"),
    };
  } catch (e) {
    return error(500, (e as Error).message, "APP_LIMITS_INVALID", req);
  }

  const manifestHash = String(row.manifest_hash ?? "");
  return {
    appId,
    manifestHash,
    status: status || "active",
    permissions,
    limits,
  };
}
