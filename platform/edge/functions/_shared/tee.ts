import { getEnv, isProductionEnv } from "./env.ts";
import { error } from "./response.ts";

let mtlsClient: Deno.HttpClient | null | undefined;
let mtlsWarningLogged = false;

function getMTLSClient(): Deno.HttpClient | undefined {
  if (mtlsClient !== undefined) return mtlsClient ?? undefined;

  const cert = getEnv("TEE_MTLS_CERT_PEM") ?? getEnv("EDGE_MTLS_CERT_PEM");
  const key = getEnv("TEE_MTLS_KEY_PEM") ?? getEnv("EDGE_MTLS_KEY_PEM");
  const ca = getEnv("TEE_MTLS_ROOT_CA_PEM") ?? getEnv("MARBLERUN_ROOT_CA_PEM");

  if (!cert || !key || !ca) {
    mtlsClient = null;
    // Log warning once in production mode
    if (!mtlsWarningLogged && isProductionEnv()) {
      console.warn(
        "[TEE] WARNING: mTLS not configured. TEE services requiring service authentication will fail. " +
          "Set TEE_MTLS_CERT_PEM, TEE_MTLS_KEY_PEM, and TEE_MTLS_ROOT_CA_PEM for production.",
      );
      mtlsWarningLogged = true;
    }
    return undefined;
  }

  mtlsClient = Deno.createHttpClient({
    caCerts: [ca],
    cert,
    key,
  });
  return mtlsClient;
}

export async function requestJSON(
  url: string,
  init: {
    method: string;
    headers?: Record<string, string>;
    body?: unknown;
  },
  req?: Request,
): Promise<unknown | Response> {
  if (isProductionEnv() && !url.toLowerCase().startsWith("https://")) {
    return error(400, "TEE service URL must use https:// in production", "INSECURE_TEE_URL", req);
  }

  const headers = new Headers(init.headers);
  let body: string | undefined = undefined;

  if (init.body !== undefined) {
    headers.set("Content-Type", "application/json");
    body = JSON.stringify(init.body);
  }

  const requestInit: RequestInit & { client?: Deno.HttpClient } = {
    method: init.method,
    headers,
    body,
  };

  const client = getMTLSClient();
  if (client) {
    requestInit.client = client;
  } else if (isProductionEnv()) {
    return error(503, "mTLS is required for TEE service calls in production", "MTLS_REQUIRED", req);
  }

  const resp = await fetch(url, requestInit);

  const text = await resp.text();
  if (!resp.ok) {
    return error(resp.status, text || `upstream error (${resp.status})`, "UPSTREAM_ERROR", req);
  }

  if (!text) return {};
  try {
    return JSON.parse(text);
  } catch {
    return error(502, "invalid upstream JSON", "UPSTREAM_INVALID_JSON", req);
  }
}

export async function postJSON(
  url: string,
  body: unknown,
  headers: Record<string, string> = {},
  req?: Request,
): Promise<unknown | Response> {
  return requestJSON(url, { method: "POST", headers, body }, req);
}

export async function getJSON(url: string, headers: Record<string, string> = {}, req?: Request): Promise<unknown | Response> {
  return requestJSON(url, { method: "GET", headers }, req);
}
