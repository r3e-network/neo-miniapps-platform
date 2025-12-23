/**
 * K8s-aware configuration module
 *
 * Detects if running in k3s cluster and provides appropriate service URLs.
 * Falls back to localhost URLs for local development outside k8s.
 */

import { getEnv } from "./env.ts";

/**
 * Detect if running inside a Kubernetes cluster
 */
export function isK8sCluster(): boolean {
  // Check for explicit K8S_CLUSTER env var
  const k8sFlag = getEnv("K8S_CLUSTER");
  if (k8sFlag === "true" || k8sFlag === "1") return true;

  // Check for Kubernetes service account (standard k8s detection)
  const k8sServiceHost = getEnv("KUBERNETES_SERVICE_HOST");
  if (k8sServiceHost) return true;

  return false;
}

/**
 * Configuration interface for service URLs
 */
export interface ServiceConfig {
  supabaseUrl: string;
  neoRpcUrl: string;
  neoFeedsUrl: string;
  neoFlowUrl: string;
  neoComputeUrl: string;
  neoVrfUrl: string;
  neoOracleUrl: string;
  txProxyUrl: string;
  globalSignerUrl: string;
}

/**
 * Get service URLs based on environment (k8s cluster vs local dev)
 */
export function getServiceConfig(): ServiceConfig {
  const inCluster = isK8sCluster();

  if (inCluster) {
    // Use internal k8s service URLs (cluster DNS)
    return {
      supabaseUrl: getEnv("SUPABASE_URL") || "http://supabase.platform.svc.cluster.local:8000",
      neoRpcUrl: getEnv("NEO_RPC_URL") || "http://neoaccounts.service-layer.svc.cluster.local:8085",
      neoFeedsUrl: getEnv("NEOFEEDS_SERVICE_URL") || "http://neofeeds.service-layer.svc.cluster.local:8083",
      neoFlowUrl: getEnv("NEOFLOW_SERVICE_URL") || "http://neoflow.service-layer.svc.cluster.local:8084",
      neoComputeUrl: getEnv("NEOCOMPUTE_SERVICE_URL") || "http://neocompute.service-layer.svc.cluster.local:8086",
      neoVrfUrl: getEnv("NEOVRF_SERVICE_URL") || "http://neovrf.service-layer.svc.cluster.local:8087",
      neoOracleUrl: getEnv("NEOORACLE_SERVICE_URL") || "http://neooracle.service-layer.svc.cluster.local:8088",
      txProxyUrl: getEnv("TXPROXY_SERVICE_URL") || "http://txproxy.service-layer.svc.cluster.local:8090",
      globalSignerUrl: getEnv("GLOBALSIGNER_SERVICE_URL") || "http://globalsigner.service-layer.svc.cluster.local:8092",
    };
  } else {
    // Use localhost URLs for local development
    return {
      supabaseUrl: getEnv("SUPABASE_URL") || "http://localhost:54321",
      neoRpcUrl: getEnv("NEO_RPC_URL") || "http://localhost:8085",
      neoFeedsUrl: getEnv("NEOFEEDS_SERVICE_URL") || "http://localhost:8083",
      neoFlowUrl: getEnv("NEOFLOW_SERVICE_URL") || "http://localhost:8084",
      neoComputeUrl: getEnv("NEOCOMPUTE_SERVICE_URL") || "http://localhost:8086",
      neoVrfUrl: getEnv("NEOVRF_SERVICE_URL") || "http://localhost:8087",
      neoOracleUrl: getEnv("NEOORACLE_SERVICE_URL") || "http://localhost:8088",
      txProxyUrl: getEnv("TXPROXY_SERVICE_URL") || "http://localhost:8090",
      globalSignerUrl: getEnv("GLOBALSIGNER_SERVICE_URL") || "http://localhost:8092",
    };
  }
}

/**
 * Get Supabase URL (k8s-aware)
 */
export function getSupabaseUrl(): string {
  return getServiceConfig().supabaseUrl;
}

/**
 * Get Neo RPC URL (k8s-aware)
 */
export function getNeoRpcUrl(): string {
  return getServiceConfig().neoRpcUrl;
}

/**
 * Log current configuration (useful for debugging)
 */
export function logConfig(): void {
  const inCluster = isK8sCluster();
  const config = getServiceConfig();

  console.log("[k8s-config] Running in:", inCluster ? "Kubernetes cluster" : "Local development");
  console.log("[k8s-config] Supabase URL:", config.supabaseUrl);
  console.log("[k8s-config] Neo RPC URL:", config.neoRpcUrl);
}
