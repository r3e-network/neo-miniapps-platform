import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

function randomNonce(): string {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  let binary = "";
  for (const b of bytes) binary += String.fromCharCode(b);
  return btoa(binary);
}

function parseFederatedOrigins(): string[] {
  const raw = (process.env.NEXT_PUBLIC_MF_REMOTES || "").trim();
  if (!raw) return [];

  const origins = new Set<string>();
  const entries = raw.split(",").map((entry) => entry.trim()).filter(Boolean);

  for (const entry of entries) {
    const separator = entry.includes("@") ? "@" : entry.includes("=") ? "=" : null;
    if (!separator) continue;
    const [, urlRaw] = entry.split(separator);
    const url = String(urlRaw || "").trim();
    if (!url) continue;
    try {
      origins.add(new URL(url).origin);
    } catch {
      continue;
    }
  }

  return Array.from(origins);
}

function buildCSP(nonce: string): string {
  const isDev = process.env.NODE_ENV !== "production";
  const federatedOrigins = parseFederatedOrigins();

  const frameOrigins = (process.env.MINIAPP_FRAME_ORIGINS || "").trim();
  const frameSrc = frameOrigins
    ? `'self' ${frameOrigins}`
    : isDev
      ? "'self' https:"
      : "'self'";

  const supabaseUrl = (process.env.NEXT_PUBLIC_SUPABASE_URL || "").trim();
  const connectSources = ["'self'"];
  if (supabaseUrl) connectSources.push(supabaseUrl);
  else if (isDev) connectSources.push("https:");
  connectSources.push(...federatedOrigins);
  const connectSrc = connectSources.join(" ");

  const scriptSources = ["'self'", `'nonce-${nonce}'`, ...federatedOrigins];
  const scriptSrc = scriptSources.join(" ");

  const csp = [
    "default-src 'self'",
    // Next.js uses inline scripts; nonce-based CSP keeps this strict without 'unsafe-inline'.
    `script-src ${scriptSrc}`,
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data: https:",
    `connect-src ${connectSrc}`,
    `frame-src ${frameSrc}`,
    // Prevent the host itself being embedded.
    "frame-ancestors 'none'",
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
  ];

  return csp.join("; ");
}

export function middleware(req: NextRequest) {
  // Skip CSP for Next.js internals and static assets.
  const pathname = req.nextUrl.pathname;
  if (pathname.startsWith("/_next/") || pathname.startsWith("/favicon") || pathname.startsWith("/robots")) {
    return NextResponse.next();
  }

  const nonce = randomNonce();
  const requestHeaders = new Headers(req.headers);
  requestHeaders.set("x-csp-nonce", nonce);

  const res = NextResponse.next({
    request: { headers: requestHeaders },
  });

  res.headers.set("Content-Security-Policy", buildCSP(nonce));

  return res;
}

export const config = {
  matcher: ["/((?!_next/static|_next/image).*)"],
};
