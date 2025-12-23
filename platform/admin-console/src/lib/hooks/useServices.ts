// =============================================================================
// React Query Hooks - Services Health
// =============================================================================

import { useQuery } from "@tanstack/react-query";
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

/**
 * Fetch all services health status
 */
async function fetchServicesHealth(): Promise<ServiceHealth[]> {
  // Call Next.js API route that checks internal services
  const response = await fetch("/api/services/health");
  if (!response.ok) {
    throw new Error("Failed to fetch services health");
  }
  return response.json();
}

/**
 * Hook to fetch services health with polling
 */
export function useServicesHealth(refetchInterval = 30000) {
  return useQuery({
    queryKey: ["services", "health"],
    queryFn: fetchServicesHealth,
    refetchInterval,
    staleTime: 10000,
  });
}

/**
 * Hook to fetch single service health
 */
export function useServiceHealth(serviceName: string) {
  return useQuery({
    queryKey: ["services", "health", serviceName],
    queryFn: async () => {
      const response = await fetch(`/api/services/health?service=${serviceName}`);
      if (!response.ok) {
        throw new Error(`Failed to fetch ${serviceName} health`);
      }
      return response.json();
    },
    refetchInterval: 30000,
    staleTime: 10000,
  });
}
