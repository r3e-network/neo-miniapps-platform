// ============================================================================
// Core Types
// ============================================================================

export interface User {
  id: string;
  email: string;
  name: string;
  walletAddress?: string;
  createdAt: string;
  updatedAt: string;
}

export interface Account {
  id: string;
  userId: string;
  email: string;
  name: string;
  status: 'active' | 'suspended' | 'closed';
  tier: 'free' | 'pro' | 'enterprise';
  metadata: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
}

export interface APIKey {
  id: string;
  accountId: string;
  name: string;
  keyPrefix: string;
  permissions: string[];
  expiresAt?: string;
  lastUsedAt?: string;
  usageCount: number;
  createdAt: string;
}

export interface Session {
  token: string;
  user: User;
  account: Account;
  expiresAt: string;
}

// ============================================================================
// Service Types
// ============================================================================

export interface ServiceConfig {
  id: string;
  serviceType: ServiceType;
  accountId: string;
  name: string;
  enabled: boolean;
  config: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
}

export type ServiceType =
  | 'oracle'
  | 'vrf'
  | 'datafeeds'
  | 'automation'
  | 'secrets'
  | 'gasbank'
  | 'mixer'
  | 'ccip'
  | 'confidential'
  | 'datalink'
  | 'datastreams'
  | 'dta'
  | 'cre';

export interface ServiceStats {
  serviceType: ServiceType;
  totalRequests: number;
  successfulRequests: number;
  failedRequests: number;
  avgResponseTime: number;
  lastRequestAt?: string;
}

// ============================================================================
// Oracle Types
// ============================================================================

export interface DataFeed {
  id: string;
  accountId: string;
  name: string;
  url: string;
  method: 'GET' | 'POST';
  headers?: Record<string, string>;
  schedule?: string;
  status: 'active' | 'paused' | 'error';
  lastFetchAt?: string;
  lastError?: string;
  createdAt: string;
}

export interface OracleRequest {
  id: string;
  feedId: string;
  status: 'pending' | 'completed' | 'failed';
  response?: unknown;
  error?: string;
  createdAt: string;
  completedAt?: string;
}

// ============================================================================
// VRF Types
// ============================================================================

export interface VRFRequest {
  id: string;
  accountId: string;
  seed: string;
  blockHash?: string;
  status: 'pending' | 'fulfilled' | 'failed';
  randomness?: string;
  proof?: string;
  callbackContract?: string;
  createdAt: string;
  fulfilledAt?: string;
}

// ============================================================================
// DataFeeds Types
// ============================================================================

export interface PriceFeed {
  id: string;
  accountId: string;
  symbol: string;
  sources: string[];
  updateInterval: number;
  deviationThreshold: number;
  enabled: boolean;
  lastPrice?: number;
  lastUpdatedAt?: string;
}

export interface PriceData {
  symbol: string;
  price: number;
  volume24h: number;
  change24h: number;
  high24h: number;
  low24h: number;
  source: string;
  timestamp: string;
  signature?: string;
}

// ============================================================================
// Automation Types
// ============================================================================

export interface AutomationTask {
  id: string;
  accountId: string;
  name: string;
  triggerType: 'cron' | 'interval' | 'event' | 'condition';
  triggerConfig: Record<string, unknown>;
  script: string;
  status: 'active' | 'paused' | 'completed' | 'failed';
  lastRunAt?: string;
  nextRunAt?: string;
  runCount: number;
  createdAt: string;
}

export interface TaskExecution {
  id: string;
  taskId: string;
  status: 'running' | 'completed' | 'failed';
  output?: string;
  error?: string;
  duration: number;
  createdAt: string;
}

// ============================================================================
// Secrets Types
// ============================================================================

export interface Secret {
  id: string;
  accountId: string;
  name: string;
  version: number;
  tags: string[];
  createdAt: string;
  updatedAt: string;
  expiresAt?: string;
}

export interface SecretRef {
  name: string;
  version?: number;
}

// ============================================================================
// GasBank Types
// ============================================================================

export interface GasAccount {
  id: string;
  accountId: string;
  balance: string;
  totalDeposited: string;
  totalSpent: string;
  createdAt: string;
  updatedAt: string;
}

export interface GasTransaction {
  id: string;
  gasAccountId: string;
  type: 'deposit' | 'sponsorship' | 'refund' | 'withdraw';
  amount: string;
  txHash?: string;
  targetTx?: string;
  status: 'pending' | 'confirmed' | 'failed';
  createdAt: string;
}

export interface DepositRequest {
  amount: string;
  txHash: string;
}

// ============================================================================
// Permission Types
// ============================================================================

export type Permission =
  | 'account:read'
  | 'account:write'
  | 'services:read'
  | 'services:write'
  | 'secrets:read'
  | 'secrets:write'
  | 'gasbank:read'
  | 'gasbank:write'
  | 'apikeys:read'
  | 'apikeys:write'
  | 'oracle:read'
  | 'oracle:write'
  | 'vrf:read'
  | 'vrf:write'
  | 'datafeeds:read'
  | 'datafeeds:write'
  | 'automation:read'
  | 'automation:write';

export interface PermissionGroup {
  name: string;
  description: string;
  permissions: Permission[];
}

// ============================================================================
// Dashboard Types
// ============================================================================

export interface DashboardStats {
  totalRequests: number;
  successRate: number;
  activeServices: number;
  gasBalance: string;
  secretsCount: number;
  apiKeysCount: number;
  recentActivity: ActivityItem[];
}

export interface ActivityItem {
  id: string;
  type: 'request' | 'config' | 'deposit' | 'secret' | 'apikey';
  description: string;
  serviceType?: ServiceType;
  status: 'success' | 'error' | 'pending';
  timestamp: string;
}

export interface UsageMetrics {
  date: string;
  requests: number;
  gasUsed: string;
  errors: number;
}

// ============================================================================
// API Response Types
// ============================================================================

export interface ApiResponse<T> {
  success: boolean;
  data?: T;
  error?: ApiError;
}

export interface ApiError {
  code: string;
  message: string;
  details?: Record<string, unknown>;
}

export interface PaginatedResponse<T> {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
  hasMore: boolean;
}
