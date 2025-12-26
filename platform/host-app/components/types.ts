// MiniApp Platform Types

export type MiniAppCategory = "gaming" | "defi" | "governance" | "utility" | "social";

export type MiniAppInfo = {
  app_id: string;
  name: string;
  description: string;
  icon: string;
  category: MiniAppCategory;
  entry_url: string;
  permissions: {
    payments?: boolean;
    governance?: boolean;
    randomness?: boolean;
    datafeed?: boolean;
  };
  limits?: {
    max_gas_per_tx?: string;
    daily_gas_cap_per_user?: string;
  };
};

export type MiniAppStats = {
  app_id: string;
  total_transactions: number;
  total_users: number;
  total_gas_used: string;
  daily_active_users: number;
  weekly_active_users: number;
  last_activity_at: string | null;
};

export type MiniAppNotification = {
  id: string;
  app_id: string;
  title: string;
  content: string;
  notification_type: string;
  source: string;
  tx_hash?: string;
  created_at: string;
};

export type WalletState = {
  connected: boolean;
  address: string;
  provider: "neoline" | "o3" | "onegate" | null;
  balance?: { neo: string; gas: string };
};
