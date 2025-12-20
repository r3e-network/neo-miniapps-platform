import { handleCorsPreflight } from "./functions/_shared/cors.ts";
import { error, json } from "./functions/_shared/response.ts";

type EdgeHandler = (req: Request) => Response | Promise<Response>;

const DEFAULT_PORT = 8787;
const FUNCTIONS_PREFIX = "/functions/v1/";
const RPC_PREFIX = "/api/rpc/";
const NAME_RE = /^[a-z0-9][a-z0-9-]{0,62}$/i;

const handlers = new Map<string, EdgeHandler>();

async function discoverFunctions(): Promise<Set<string>> {
  const dir = new URL("./functions/", import.meta.url);
  const names = new Set<string>();
  for await (const entry of Deno.readDir(dir)) {
    if (!entry.isDirectory) continue;
    if (entry.name === "_shared") continue;
    if (!NAME_RE.test(entry.name)) continue;
    try {
      const indexUrl = new URL(`./functions/${entry.name}/index.ts`, import.meta.url);
      const stat = await Deno.stat(indexUrl);
      if (stat.isFile) names.add(entry.name);
    } catch {
      // ignore
    }
  }
  return names;
}

function parseFunctionName(pathname: string): { name: string; ok: boolean } {
  if (pathname.startsWith(FUNCTIONS_PREFIX)) {
    const rest = pathname.slice(FUNCTIONS_PREFIX.length);
    const name = rest.split("/")[0] ?? "";
    return { name, ok: NAME_RE.test(name) };
  }

  if (pathname.startsWith(RPC_PREFIX)) {
    const rest = pathname.slice(RPC_PREFIX.length);
    const name = rest.split("/")[0] ?? "";
    return { name, ok: NAME_RE.test(name) };
  }

  const trimmed = pathname.replace(/^\/+/, "");
  const name = trimmed.split("/")[0] ?? "";
  return { name, ok: NAME_RE.test(name) };
}

async function loadHandler(name: string): Promise<EdgeHandler | null> {
  if (handlers.has(name)) return handlers.get(name) ?? null;

  const mod = await import(`./functions/${name}/index.ts`);
  const candidate: unknown = (mod as any)?.handler ?? (mod as any)?.default;
  if (typeof candidate !== "function") return null;

  const h = candidate as EdgeHandler;
  handlers.set(name, h);
  return h;
}

const available = await discoverFunctions();
const port = Number(Deno.env.get("EDGE_DEV_PORT") ?? DEFAULT_PORT) || DEFAULT_PORT;

Deno.serve({ port }, async (req) => {
  const preflight = handleCorsPreflight(req);
  if (preflight) return preflight;

  const url = new URL(req.url);
  if (url.pathname === "/" || url.pathname === "/health") {
    return json({
      status: "ok",
      port,
      routes: {
        preferred_base_url: `http://localhost:${port}${FUNCTIONS_PREFIX.replace(/\/$/, "")}`,
        rpc_compat_base_url: `http://localhost:${port}${RPC_PREFIX.replace(/\/$/, "")}`,
        also_supported: `http://localhost:${port}`,
      },
      functions: Array.from(available).sort(),
    }, {}, req);
  }

  const { name, ok } = parseFunctionName(url.pathname);
  if (!ok || !available.has(name)) return error(404, "function not found", "NOT_FOUND", req);

  let handler: EdgeHandler | null;
  try {
    handler = await loadHandler(name);
  } catch (e) {
    return error(500, `failed to load function: ${(e as Error).message}`, "LOAD_FAILED", req);
  }

  if (!handler) return error(500, "function missing handler export", "BAD_FUNCTION", req);
  return await handler(req);
});
