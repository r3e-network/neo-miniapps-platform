import { createClient } from '@supabase/supabase-js';
import type { Provider } from '@supabase/supabase-js';

const supabaseUrl = import.meta.env.VITE_SUPABASE_URL;
const supabaseAnonKey = import.meta.env.VITE_SUPABASE_ANON_KEY;

if (!supabaseUrl || !supabaseAnonKey) {
  console.warn('Supabase credentials not configured. Auth features will be limited.');
}

export const supabase = createClient(
  supabaseUrl || 'https://placeholder.supabase.co',
  supabaseAnonKey || 'placeholder-key',
  {
    auth: {
      autoRefreshToken: true,
      persistSession: true,
      detectSessionInUrl: true,
      flowType: 'pkce',
    },
  }
);

// Supported OAuth providers
export type OAuthProvider = 'github' | 'google' | 'discord' | 'twitter' | 'apple';

export const OAUTH_PROVIDERS: { id: OAuthProvider; name: string; icon: string; color: string }[] = [
  { id: 'github', name: 'GitHub', icon: 'github', color: '#24292e' },
  { id: 'google', name: 'Google', icon: 'google', color: '#4285f4' },
  { id: 'discord', name: 'Discord', icon: 'discord', color: '#5865f2' },
  { id: 'twitter', name: 'Twitter', icon: 'twitter', color: '#1da1f2' },
];

// Sign in with OAuth provider
export async function signInWithOAuth(provider: OAuthProvider) {
  const { data, error } = await supabase.auth.signInWithOAuth({
    provider: provider as Provider,
    options: {
      redirectTo: `${window.location.origin}/auth/callback`,
      scopes: provider === 'github' ? 'read:user user:email' : undefined,
    },
  });

  if (error) throw error;
  return data;
}

// Sign in with email/password
export async function signInWithEmail(email: string, password: string) {
  const { data, error } = await supabase.auth.signInWithPassword({
    email,
    password,
  });

  if (error) throw error;
  return data;
}

// Sign up with email/password
export async function signUpWithEmail(email: string, password: string, metadata?: { name?: string }) {
  const { data, error } = await supabase.auth.signUp({
    email,
    password,
    options: {
      data: metadata,
      emailRedirectTo: `${window.location.origin}/auth/callback`,
    },
  });

  if (error) throw error;
  return data;
}

// Sign out
export async function signOut() {
  const { error } = await supabase.auth.signOut();
  if (error) throw error;
}

// Get current session
export async function getSession() {
  const { data, error } = await supabase.auth.getSession();
  if (error) throw error;
  return data.session;
}

// Get current user
export async function getUser() {
  const { data, error } = await supabase.auth.getUser();
  if (error) throw error;
  return data.user;
}

// Reset password
export async function resetPassword(email: string) {
  const { error } = await supabase.auth.resetPasswordForEmail(email, {
    redirectTo: `${window.location.origin}/auth/reset-password`,
  });
  if (error) throw error;
}

// Update password
export async function updatePassword(newPassword: string) {
  const { error } = await supabase.auth.updateUser({
    password: newPassword,
  });
  if (error) throw error;
}

// Update user metadata
export async function updateUserMetadata(metadata: Record<string, unknown>) {
  const { data, error } = await supabase.auth.updateUser({
    data: metadata,
  });
  if (error) throw error;
  return data.user;
}

export default supabase;

// =============================================================================
// Service Request Types
// =============================================================================

export type ServiceType = 'oracle' | 'vrf' | 'secrets' | 'gasbank' | 'mixer' | 'datafeeds' | 'accounts' | 'automation' | 'ccip' | 'confidential' | 'cre' | 'datalink' | 'datastreams' | 'dta';
export type RequestStatus = 'pending' | 'processing' | 'completed' | 'failed' | 'cancelled';

export interface ServiceRequest {
  id: string;
  user_id: string;
  service_type: ServiceType;
  operation: string;
  payload: Record<string, unknown>;
  status: RequestStatus;
  priority: number;
  result?: Record<string, unknown>;
  error_message?: string;
  tee_signature?: string;
  created_at: string;
  started_at?: string;
  completed_at?: string;
  metadata?: Record<string, unknown>;
}

export interface Notification {
  id: string;
  user_id: string;
  type: string;
  title: string;
  message?: string;
  data?: Record<string, unknown>;
  reference_type?: string;
  reference_id?: string;
  read: boolean;
  created_at: string;
}

// =============================================================================
// Service Request API (Frontend → Supabase → Service Layer)
// =============================================================================

/**
 * Submit a service request to be processed by Service Layer
 */
export async function submitServiceRequest(
  serviceType: ServiceType,
  operation: string,
  payload: Record<string, unknown>,
  priority: number = 0
): Promise<ServiceRequest> {
  const user = await getUser();
  if (!user) {
    throw new Error('User not authenticated');
  }

  const { data, error } = await supabase
    .from('service_requests')
    .insert({
      user_id: user.id,
      service_type: serviceType,
      operation,
      payload,
      priority,
      status: 'pending'
    })
    .select()
    .single();

  if (error) throw error;
  return data;
}

/**
 * Get a specific request by ID
 */
export async function getServiceRequest(requestId: string): Promise<ServiceRequest | null> {
  const { data, error } = await supabase
    .from('service_requests')
    .select('*')
    .eq('id', requestId)
    .single();

  if (error) {
    if (error.code === 'PGRST116') return null;
    throw error;
  }
  return data;
}

/**
 * List requests for the current user
 */
export async function listServiceRequests(
  serviceType?: ServiceType,
  status?: RequestStatus,
  limit: number = 50
): Promise<ServiceRequest[]> {
  let query = supabase
    .from('service_requests')
    .select('*')
    .order('created_at', { ascending: false })
    .limit(limit);

  if (serviceType) {
    query = query.eq('service_type', serviceType);
  }
  if (status) {
    query = query.eq('status', status);
  }

  const { data, error } = await query;
  if (error) throw error;
  return data || [];
}

/**
 * Cancel a pending request
 */
export async function cancelServiceRequest(requestId: string): Promise<void> {
  const { error } = await supabase
    .from('service_requests')
    .update({ status: 'cancelled' })
    .eq('id', requestId)
    .eq('status', 'pending');

  if (error) throw error;
}

/**
 * Wait for a request to complete (with Realtime subscription)
 */
export function waitForRequestCompletion(
  requestId: string,
  onUpdate: (request: ServiceRequest) => void,
  onComplete: (request: ServiceRequest) => void,
  onError: (error: Error) => void,
  timeoutMs: number = 60000
): () => void {
  let timeoutId: NodeJS.Timeout;

  const channel = supabase
    .channel(`request:${requestId}`)
    .on(
      'postgres_changes',
      {
        event: 'UPDATE',
        schema: 'public',
        table: 'service_requests',
        filter: `id=eq.${requestId}`
      },
      (payload) => {
        const request = payload.new as ServiceRequest;
        onUpdate(request);

        if (request.status === 'completed' || request.status === 'failed') {
          clearTimeout(timeoutId);
          channel.unsubscribe();
          onComplete(request);
        }
      }
    )
    .subscribe();

  // Timeout handler
  timeoutId = setTimeout(() => {
    channel.unsubscribe();
    onError(new Error('Request timeout'));
  }, timeoutMs);

  // Return cleanup function
  return () => {
    clearTimeout(timeoutId);
    channel.unsubscribe();
  };
}

// =============================================================================
// Realtime Subscriptions
// =============================================================================

/**
 * Subscribe to all user's service requests
 */
export function subscribeToUserRequests(
  onInsert: (request: ServiceRequest) => void,
  onUpdate: (request: ServiceRequest) => void
) {
  return supabase
    .channel('user_requests')
    .on(
      'postgres_changes',
      {
        event: 'INSERT',
        schema: 'public',
        table: 'service_requests'
      },
      (payload) => onInsert(payload.new as ServiceRequest)
    )
    .on(
      'postgres_changes',
      {
        event: 'UPDATE',
        schema: 'public',
        table: 'service_requests'
      },
      (payload) => onUpdate(payload.new as ServiceRequest)
    )
    .subscribe();
}

/**
 * Subscribe to notifications
 */
export function subscribeToNotifications(
  onNotification: (notification: Notification) => void
) {
  return supabase
    .channel('notifications')
    .on(
      'postgres_changes',
      {
        event: 'INSERT',
        schema: 'public',
        table: 'realtime_notifications'
      },
      (payload) => onNotification(payload.new as Notification)
    )
    .subscribe();
}

/**
 * Get unread notifications
 */
export async function getUnreadNotifications(): Promise<Notification[]> {
  const { data, error } = await supabase
    .from('realtime_notifications')
    .select('*')
    .eq('read', false)
    .order('created_at', { ascending: false });

  if (error) throw error;
  return data || [];
}

/**
 * Mark notification as read
 */
export async function markNotificationRead(notificationId: string): Promise<void> {
  const { error } = await supabase
    .from('realtime_notifications')
    .update({ read: true })
    .eq('id', notificationId);

  if (error) throw error;
}

// =============================================================================
// Service-Specific Helpers
// =============================================================================

/** Oracle: Fetch external data */
export async function oracleFetch(
  url: string,
  method: string = 'GET',
  headers?: Record<string, string>,
  jsonPath?: string
): Promise<ServiceRequest> {
  return submitServiceRequest('oracle', 'fetch', {
    url,
    method,
    headers,
    json_path: jsonPath
  });
}

/** VRF: Generate random value */
export async function vrfRandom(seed: string, numValues: number = 1): Promise<ServiceRequest> {
  return submitServiceRequest('vrf', 'random', { seed, num_values: numValues });
}

/** Secrets: Store a secret */
export async function secretsStore(name: string, value: string): Promise<ServiceRequest> {
  return submitServiceRequest('secrets', 'store', { name, value });
}

/** Secrets: Get a secret */
export async function secretsGet(name: string): Promise<ServiceRequest> {
  return submitServiceRequest('secrets', 'get', { name });
}

/** GasBank: Get balance */
export async function gasbankBalance(): Promise<ServiceRequest> {
  return submitServiceRequest('gasbank', 'balance', {});
}

/** GasBank: Deposit */
export async function gasbankDeposit(amount: string, txHash: string): Promise<ServiceRequest> {
  return submitServiceRequest('gasbank', 'deposit', { amount, tx_hash: txHash });
}

/** DataFeeds: Get feed value */
export async function datafeedsGetValue(feedId: string): Promise<ServiceRequest> {
  return submitServiceRequest('datafeeds', 'get_value', { feed_id: feedId });
}

/** Automation: Create task */
export async function automationCreateTask(
  name: string,
  cron: string,
  functionCode: string
): Promise<ServiceRequest> {
  return submitServiceRequest('automation', 'create_task', {
    name,
    cron,
    function_code: functionCode
  });
}

// =============================================================================
// Additional Service Helpers (8 more services)
// =============================================================================

/** Mixer: Create mixing request */
export async function mixerCreateRequest(
  amount: string,
  sourceChain: string,
  targetChain: string,
  targetAddress: string
): Promise<ServiceRequest> {
  return submitServiceRequest('mixer', 'create_request', {
    amount,
    source_chain: sourceChain,
    target_chain: targetChain,
    target_address: targetAddress
  });
}

/** Mixer: Get mixing status */
export async function mixerGetStatus(requestId: string): Promise<ServiceRequest> {
  return submitServiceRequest('mixer', 'get_status', { request_id: requestId });
}

/** Accounts: Create account */
export async function accountsCreate(name: string, metadata?: Record<string, unknown>): Promise<ServiceRequest> {
  return submitServiceRequest('accounts', 'create', { name, metadata });
}

/** Accounts: Get account */
export async function accountsGet(accountId: string): Promise<ServiceRequest> {
  return submitServiceRequest('accounts', 'get', { account_id: accountId });
}

/** Accounts: List accounts */
export async function accountsList(): Promise<ServiceRequest> {
  return submitServiceRequest('accounts', 'list', {});
}

/** CCIP: Send cross-chain message */
export async function ccipSendMessage(
  targetChain: string,
  targetContract: string,
  payload: string,
  gasLimit?: number
): Promise<ServiceRequest> {
  return submitServiceRequest('ccip', 'send_message', {
    target_chain: targetChain,
    target_contract: targetContract,
    payload,
    gas_limit: gasLimit
  });
}

/** CCIP: Get message status */
export async function ccipGetMessageStatus(messageId: string): Promise<ServiceRequest> {
  return submitServiceRequest('ccip', 'get_message_status', { message_id: messageId });
}

/** Confidential: Execute confidential computation */
export async function confidentialExecute(
  functionCode: string,
  inputs: Record<string, unknown>,
  encryptOutput?: boolean
): Promise<ServiceRequest> {
  return submitServiceRequest('confidential', 'execute', {
    function_code: functionCode,
    inputs,
    encrypt_output: encryptOutput ?? true
  });
}

/** Confidential: Get execution result */
export async function confidentialGetResult(executionId: string): Promise<ServiceRequest> {
  return submitServiceRequest('confidential', 'get_result', { execution_id: executionId });
}

/** CRE (Chainlink Runtime Environment): Deploy function */
export async function creDeployFunction(
  name: string,
  sourceCode: string,
  runtime: string
): Promise<ServiceRequest> {
  return submitServiceRequest('cre', 'deploy_function', {
    name,
    source_code: sourceCode,
    runtime
  });
}

/** CRE: Invoke function */
export async function creInvokeFunction(
  functionId: string,
  args: Record<string, unknown>
): Promise<ServiceRequest> {
  return submitServiceRequest('cre', 'invoke_function', {
    function_id: functionId,
    args
  });
}

/** DataLink: Create data link */
export async function datalinkCreate(
  name: string,
  sourceUrl: string,
  transformScript?: string
): Promise<ServiceRequest> {
  return submitServiceRequest('datalink', 'create', {
    name,
    source_url: sourceUrl,
    transform_script: transformScript
  });
}

/** DataLink: Fetch data */
export async function datalinkFetch(linkId: string): Promise<ServiceRequest> {
  return submitServiceRequest('datalink', 'fetch', { link_id: linkId });
}

/** DataStreams: Create stream */
export async function datastreamsCreate(
  name: string,
  sourceType: string,
  config: Record<string, unknown>
): Promise<ServiceRequest> {
  return submitServiceRequest('datastreams', 'create', {
    name,
    source_type: sourceType,
    config
  });
}

/** DataStreams: Subscribe to stream */
export async function datastreamsSubscribe(streamId: string): Promise<ServiceRequest> {
  return submitServiceRequest('datastreams', 'subscribe', { stream_id: streamId });
}

/** DataStreams: Get latest value */
export async function datastreamsGetLatest(streamId: string): Promise<ServiceRequest> {
  return submitServiceRequest('datastreams', 'get_latest', { stream_id: streamId });
}

/** DTA (Data Trust Alliance): Register data source */
export async function dtaRegisterSource(
  name: string,
  endpoint: string,
  schema: Record<string, unknown>
): Promise<ServiceRequest> {
  return submitServiceRequest('dta', 'register_source', {
    name,
    endpoint,
    schema
  });
}

/** DTA: Query data */
export async function dtaQuery(
  sourceId: string,
  query: string
): Promise<ServiceRequest> {
  return submitServiceRequest('dta', 'query', {
    source_id: sourceId,
    query
  });
}

/** DTA: Verify data integrity */
export async function dtaVerify(dataHash: string): Promise<ServiceRequest> {
  return submitServiceRequest('dta', 'verify', { data_hash: dataHash });
}
