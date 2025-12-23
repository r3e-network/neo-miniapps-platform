// =============================================================================
// Admin Console Type Definitions
// =============================================================================

export type ServiceStatus = "healthy" | "unhealthy" | "unknown";

export interface ServiceHealth {
  name: string;
  status: ServiceStatus;
  url: string;
  lastCheck: string;
  version?: string;
  uptime?: number;
  error?: string;
}

export interface MiniApp {
  app_id: string;
  developer_user_id: string;
  manifest_hash: string;
  entry_url: string;
  developer_pubkey: string;
  permissions: Record<string, unknown>;
  limits: {
    daily_gas_cap_per_user?: number;
    governance_cap?: number;
  };
  assets_allowed: string[];
  governance_assets_allowed: string[];
  manifest: Record<string, unknown>;
  status: "active" | "disabled";
  created_at: string;
  updated_at: string;
}

export interface User {
  id: string;
  address: string;
  email?: string;
  created_at: string;
  updated_at: string;
}

export interface MiniAppUsage {
  id: string;
  user_id: string;
  app_id: string;
  usage_date: string;
  gas_used: number;
  governance_used: number;
  created_at: string;
  updated_at: string;
}

export interface AnalyticsData {
  totalUsers: number;
  totalMiniApps: number;
  totalTransactions: number;
  gasUsageToday: number;
  usageByApp: Array<{
    app_id: string;
    total_gas: number;
    total_governance: number;
    user_count: number;
  }>;
  usageOverTime: Array<{
    date: string;
    gas_used: number;
    governance_used: number;
  }>;
}

export interface Contract {
  name: string;
  hash: string;
  deployed: boolean;
  network: string;
  deployedAt?: string;
}

export interface DeploymentRequest {
  contractName: string;
  network: string;
  parameters?: Record<string, unknown>;
}

export interface APIError {
  message: string;
  code?: string;
  details?: unknown;
}
