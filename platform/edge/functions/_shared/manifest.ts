import { getEnv } from "./env.ts";
import { bytesToHex, normalizeHex } from "./hex.ts";

export type MiniAppManifestCore = {
  appId: string;
  entryUrl: string;
  developerPubKeyHex: string; // hex (no 0x)
  manifestHashHex: string; // sha256, hex (no 0x)
};

function isProductionEnv(): boolean {
  const candidates = [
    getEnv("ENV"),
    getEnv("NODE_ENV"),
    getEnv("SUPABASE_ENV"),
  ]
    .filter(Boolean)
    .map((v) => String(v).toLowerCase());
  return candidates.includes("prod") || candidates.includes("production");
}

function stableSort(value: unknown): unknown {
  if (value === null) return null;
  if (Array.isArray(value)) return value.map(stableSort);
  if (typeof value !== "object") return value;

  const obj = value as Record<string, unknown>;
  const out: Record<string, unknown> = {};
  for (const key of Object.keys(obj).sort()) {
    const v = obj[key];
    if (v === undefined) continue;
    out[key] = stableSort(v);
  }
  return out;
}

export function stableStringify(value: unknown): string {
  return JSON.stringify(stableSort(value));
}

function normalizeStringList(
  value: unknown,
  label: string,
  mode: "upper" | "lower" | "preserve" = "preserve",
): string[] {
  if (!Array.isArray(value)) throw new Error(`${label} must be an array`);
  const items = value
    .map((v) => String(v ?? "").trim())
    .filter(Boolean)
    .map((v) => (mode === "upper" ? v.toUpperCase() : mode === "lower" ? v.toLowerCase() : v));
  return Array.from(new Set(items)).sort();
}

export function canonicalizeMiniAppManifest(manifest: unknown): Record<string, unknown> {
  if (!manifest || typeof manifest !== "object" || Array.isArray(manifest)) {
    throw new Error("manifest must be an object");
  }
  const m = manifest as Record<string, unknown>;

  const appId = String(m.app_id ?? "").trim();
  const entryUrl = String(m.entry_url ?? "").trim();
  const developerPubKey = String(m.developer_pubkey ?? "").trim();

  if (!appId) throw new Error("manifest.app_id required");
  if (!entryUrl) throw new Error("manifest.entry_url required");
  if (!developerPubKey) throw new Error("manifest.developer_pubkey required");

  const out: Record<string, unknown> = { ...m };
  out.app_id = appId;
  out.entry_url = entryUrl;
  out.developer_pubkey = normalizeHex(developerPubKey, "manifest.developer_pubkey");

  if ("name" in m) out.name = String(m.name ?? "").trim();
  if ("version" in m) out.version = String(m.version ?? "").trim();

  if ("assets_allowed" in m) {
    out.assets_allowed = normalizeStringList(m.assets_allowed, "manifest.assets_allowed", "upper");
  }
  if ("governance_assets_allowed" in m) {
    out.governance_assets_allowed = normalizeStringList(
      m.governance_assets_allowed,
      "manifest.governance_assets_allowed",
      "upper",
    );
  }
  if ("sandbox_flags" in m) {
    out.sandbox_flags = normalizeStringList(m.sandbox_flags, "manifest.sandbox_flags", "lower");
  }
  if ("contracts_needed" in m) {
    out.contracts_needed = normalizeStringList(m.contracts_needed, "manifest.contracts_needed", "preserve");
  }

  return out;
}

export async function computeSHA256Hex(input: string): Promise<string> {
  const data = new TextEncoder().encode(input);
  const digest = await crypto.subtle.digest("SHA-256", data);
  return bytesToHex(new Uint8Array(digest));
}

export async function computeManifestHashHex(manifest: unknown): Promise<string> {
  const canonical = canonicalizeMiniAppManifest(manifest);
  const payload = stableStringify(canonical);
  return computeSHA256Hex(payload);
}

export function enforceMiniAppAssetPolicy(manifest: unknown): void {
  const canonical = canonicalizeMiniAppManifest(manifest);

  const assetsAllowed = normalizeStringList(
    canonical.assets_allowed,
    "manifest.assets_allowed",
    "upper",
  );
  if (assetsAllowed.length !== 1 || assetsAllowed[0] !== "GAS") {
    throw new Error("manifest.assets_allowed must be exactly [\"GAS\"]");
  }

  const governanceAssetsAllowed = normalizeStringList(
    canonical.governance_assets_allowed,
    "manifest.governance_assets_allowed",
    "upper",
  );
  if (governanceAssetsAllowed.length !== 1 || governanceAssetsAllowed[0] !== "NEO") {
    throw new Error("manifest.governance_assets_allowed must be exactly [\"NEO\"]");
  }

  const entryUrl = String(canonical.entry_url ?? "").trim();
  if (isProductionEnv() && !entryUrl.startsWith("https://")) {
    throw new Error("manifest.entry_url must use https:// in production");
  }
}

export async function parseMiniAppManifestCore(manifest: unknown): Promise<MiniAppManifestCore> {
  enforceMiniAppAssetPolicy(manifest);
  const canonical = canonicalizeMiniAppManifest(manifest);

  const appId = String(canonical.app_id ?? "").trim();
  const entryUrl = String(canonical.entry_url ?? "").trim();
  const developerPubKeyHex = normalizeHex(String(canonical.developer_pubkey ?? ""), "manifest.developer_pubkey");

  const manifestHashHex = await computeManifestHashHex(canonical);

  return {
    appId,
    entryUrl,
    developerPubKeyHex,
    manifestHashHex,
  };
}

