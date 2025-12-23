// =============================================================================
// API Route: Services Health Check
// =============================================================================

import { NextResponse } from "next/server";
import type { ServiceHealth } from "@/types";

const SERVICES = [
  { name: "neofeeds", url: "http://neofeeds.service-layer.svc.cluster.local:8080" },
  { name: "neoaccounts", url: "http://neoaccounts.service-layer.svc.cluster.local:8085" },
  { name: "confcompute", url: "http://confcompute.service-layer.svc.cluster.local:8081" },
  { name: "conforacle", url: "http://conforacle.service-layer.svc.cluster.local:8082" },
  { name: "datafeed", url: "http://datafeed.service-layer.svc.cluster.local:8083" },
  { name: "vrf", url: "http://vrf.service-layer.svc.cluster.local:8084" },
  { name: "automation", url: "http://automation.service-layer.svc.cluster.local:8086" },
  { name: "gasbank", url: "http://gasbank.service-layer.svc.cluster.local:8087" },
  { name: "edge-gateway", url: "http://edge-gateway.platform.svc.cluster.local:8787" },
];

async function checkServiceHealth(name: string, url: string): Promise<ServiceHealth> {
  const lastCheck = new Date().toISOString();

  try {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 5000);

    const response = await fetch(`${url}/health`, {
      signal: controller.signal,
      headers: { "Content-Type": "application/json" },
    });

    clearTimeout(timeoutId);

    if (!response.ok) {
      return {
        name,
        status: "unhealthy",
        url,
        lastCheck,
        error: `HTTP ${response.status}`,
      };
    }

    const data = await response.json();

    return {
      name,
      status: "healthy",
      url,
      lastCheck,
      version: data.version,
      uptime: data.uptime,
    };
  } catch (error) {
    return {
      name,
      status: "unhealthy",
      url,
      lastCheck,
      error: error instanceof Error ? error.message : "Unknown error",
    };
  }
}

export async function GET() {
  try {
    const healthChecks = await Promise.all(SERVICES.map((service) => checkServiceHealth(service.name, service.url)));

    return NextResponse.json(healthChecks);
  } catch (error) {
    return NextResponse.json({ error: "Failed to check services health" }, { status: 500 });
  }
}
