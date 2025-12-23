// =============================================================================
// Admin SDK - Administrative API client for platform management
// =============================================================================

export interface AdminSDKConfig {
  adminBaseUrl: string;
  supabaseUrl: string;
  serviceRoleKey?: string;
}

export interface ServiceHealthResponse {
  name: string;
  status: "healthy" | "unhealthy" | "unknown";
  url: string;
  lastCheck: string;
  version?: string;
  uptime?: number;
  error?: string;
}

export interface MiniAppListResponse {
  app_id: string;
  developer_user_id: string;
  manifest_hash: string;
  entry_url: string;
  status: "active" | "disabled";
  created_at: string;
  updated_at: string;
}

export interface UserListResponse {
  id: string;
  address: string;
  email?: string;
  created_at: string;
  updated_at: string;
}

export interface AnalyticsResponse {
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
}

/**
 * Admin SDK for platform management operations
 */
export class AdminSDK {
  private config: AdminSDKConfig;

  constructor(config: AdminSDKConfig) {
    this.config = config;
  }

  /**
   * Fetch all services health status
   */
  async getServicesHealth(): Promise<ServiceHealthResponse[]> {
    const response = await fetch(`${this.config.adminBaseUrl}/api/services/health`);
    if (!response.ok) {
      throw new Error(`Failed to fetch services health: ${response.statusText}`);
    }
    return response.json();
  }

  /**
   * Fetch analytics overview
   */
  async getAnalytics(): Promise<AnalyticsResponse> {
    const response = await fetch(`${this.config.adminBaseUrl}/api/analytics`);
    if (!response.ok) {
      throw new Error(`Failed to fetch analytics: ${response.statusText}`);
    }
    return response.json();
  }

  /**
   * Fetch all registered MiniApps
   */
  async getMiniApps(): Promise<MiniAppListResponse[]> {
    const response = await fetch(`${this.config.supabaseUrl}/rest/v1/miniapps?select=*&order=created_at.desc`, {
      headers: {
        apikey: this.config.serviceRoleKey || "",
        Authorization: `Bearer ${this.config.serviceRoleKey || ""}`,
      },
    });
    if (!response.ok) {
      throw new Error(`Failed to fetch MiniApps: ${response.statusText}`);
    }
    return response.json();
  }

  /**
   * Fetch all users
   */
  async getUsers(): Promise<UserListResponse[]> {
    const response = await fetch(`${this.config.supabaseUrl}/rest/v1/users?select=*&order=created_at.desc`, {
      headers: {
        apikey: this.config.serviceRoleKey || "",
        Authorization: `Bearer ${this.config.serviceRoleKey || ""}`,
      },
    });
    if (!response.ok) {
      throw new Error(`Failed to fetch users: ${response.statusText}`);
    }
    return response.json();
  }

  /**
   * Update MiniApp status
   */
  async updateMiniAppStatus(appId: string, status: "active" | "disabled"): Promise<void> {
    const response = await fetch(`${this.config.supabaseUrl}/rest/v1/miniapps?app_id=eq.${appId}`, {
      method: "PATCH",
      headers: {
        apikey: this.config.serviceRoleKey || "",
        Authorization: `Bearer ${this.config.serviceRoleKey || ""}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ status }),
    });
    if (!response.ok) {
      throw new Error(`Failed to update MiniApp status: ${response.statusText}`);
    }
  }
}

/**
 * Create an Admin SDK instance
 */
export function createAdminSDK(config: AdminSDKConfig): AdminSDK {
  return new AdminSDK(config);
}
