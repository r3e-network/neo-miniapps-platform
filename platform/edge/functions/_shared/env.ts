export function getEnv(name: string): string | undefined {
  const raw = Deno.env.get(name);
  const trimmed = raw?.trim();
  return trimmed ? trimmed : undefined;
}

export function mustGetEnv(name: string): string {
  const value = getEnv(name);
  if (!value) throw new Error(`missing required env var: ${name}`);
  return value;
}

export function isProductionEnv(): boolean {
  const candidates = [
    getEnv("EDGE_ENV"),
    getEnv("DENO_ENV"),
    getEnv("ENV"),
    getEnv("NODE_ENV"),
    getEnv("SUPABASE_ENV"),
  ]
    .filter(Boolean)
    .map((v) => String(v).toLowerCase());
  return candidates.includes("prod") || candidates.includes("production");
}
