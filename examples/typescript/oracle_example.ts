/**
 * Oracle Service Example
 *
 * This example demonstrates how to use the Oracle service to fetch external data
 * with TEE (Trusted Execution Environment) protection.
 *
 * Run: npx ts-node oracle_example.ts
 */

import { createClient, SupabaseClient } from '@supabase/supabase-js';

// =============================================================================
// Configuration
// =============================================================================

const SUPABASE_URL = process.env.VITE_SUPABASE_URL || 'http://localhost:54321';
const SUPABASE_KEY = process.env.VITE_SUPABASE_ANON_KEY || '';

// =============================================================================
// Types
// =============================================================================

interface ServiceRequest {
  id?: string;
  user_id?: string;
  service_type: string;
  operation: string;
  payload: Record<string, unknown>;
  status?: 'pending' | 'processing' | 'completed' | 'failed';
  result?: Record<string, unknown>;
  error_message?: string;
  tee_signature?: string;
  created_at?: string;
  completed_at?: string;
}

// =============================================================================
// Service Client
// =============================================================================

class ServiceLayerClient {
  private supabase: SupabaseClient;

  constructor(url: string, key: string) {
    this.supabase = createClient(url, key);
  }

  /**
   * Submit a service request
   */
  async submitRequest(request: Omit<ServiceRequest, 'id' | 'status'>): Promise<ServiceRequest> {
    const { data, error } = await this.supabase
      .from('service_requests')
      .insert(request)
      .select()
      .single();

    if (error) throw new Error(`Submit failed: ${error.message}`);
    return data;
  }

  /**
   * Get a request by ID
   */
  async getRequest(id: string): Promise<ServiceRequest | null> {
    const { data, error } = await this.supabase
      .from('service_requests')
      .select('*')
      .eq('id', id)
      .single();

    if (error) return null;
    return data;
  }

  /**
   * Wait for request completion
   */
  async waitForCompletion(id: string, timeoutMs: number = 30000): Promise<ServiceRequest> {
    const startTime = Date.now();

    while (Date.now() - startTime < timeoutMs) {
      const request = await this.getRequest(id);

      if (request?.status === 'completed' || request?.status === 'failed') {
        return request;
      }

      console.log(`Status: ${request?.status || 'unknown'}, waiting...`);
      await new Promise(resolve => setTimeout(resolve, 1000));
    }

    throw new Error('Timeout waiting for completion');
  }

  /**
   * Subscribe to request updates (real-time)
   */
  subscribeToRequest(id: string, callback: (request: ServiceRequest) => void) {
    return this.supabase
      .channel(`request-${id}`)
      .on(
        'postgres_changes',
        {
          event: 'UPDATE',
          schema: 'public',
          table: 'service_requests',
          filter: `id=eq.${id}`,
        },
        (payload) => callback(payload.new as ServiceRequest)
      )
      .subscribe();
  }
}

// =============================================================================
// Oracle Service Helpers
// =============================================================================

class OracleService {
  private client: ServiceLayerClient;

  constructor(client: ServiceLayerClient) {
    this.client = client;
  }

  /**
   * Fetch data from an external URL
   */
  async fetch(url: string, options?: {
    method?: 'GET' | 'POST';
    headers?: Record<string, string>;
    body?: string;
    jsonPath?: string;
  }): Promise<ServiceRequest> {
    const request = await this.client.submitRequest({
      service_type: 'oracle',
      operation: 'fetch',
      payload: {
        url,
        method: options?.method || 'GET',
        headers: options?.headers,
        body: options?.body,
        json_path: options?.jsonPath,
      },
    });

    return this.client.waitForCompletion(request.id!);
  }

  /**
   * Fetch with callback to smart contract
   */
  async fetchWithCallback(url: string, callbackHash: string): Promise<ServiceRequest> {
    const request = await this.client.submitRequest({
      service_type: 'oracle',
      operation: 'fetch',
      payload: {
        url,
        method: 'GET',
        callback_hash: callbackHash,
      },
    });

    return this.client.waitForCompletion(request.id!);
  }
}

// =============================================================================
// Examples
// =============================================================================

async function main() {
  console.log('=== Oracle Service Example ===\n');

  if (!SUPABASE_KEY) {
    console.error('Error: VITE_SUPABASE_ANON_KEY environment variable required');
    process.exit(1);
  }

  const client = new ServiceLayerClient(SUPABASE_URL, SUPABASE_KEY);
  const oracle = new OracleService(client);

  // Example 1: Simple price fetch
  console.log('Example 1: Fetching NEO price...');
  try {
    const result = await oracle.fetch(
      'https://api.coingecko.com/api/v3/simple/price?ids=neo&vs_currencies=usd'
    );

    if (result.status === 'completed') {
      console.log('✓ Success!');
      console.log('  Result:', JSON.stringify(result.result, null, 2));
      if (result.tee_signature) {
        console.log('  TEE Signature:', result.tee_signature.substring(0, 32) + '...');
      }
    } else {
      console.log('✗ Failed:', result.error_message);
    }
  } catch (error) {
    console.error('Error:', error);
  }

  console.log();

  // Example 2: Fetch with JSON path extraction
  console.log('Example 2: Fetching with JSON path...');
  try {
    const result = await oracle.fetch(
      'https://api.coingecko.com/api/v3/simple/price?ids=neo&vs_currencies=usd',
      { jsonPath: '$.neo.usd' }
    );

    if (result.status === 'completed') {
      console.log('✓ NEO Price (USD):', result.result?.value);
    }
  } catch (error) {
    console.error('Error:', error);
  }

  console.log();

  // Example 3: Real-time subscription
  console.log('Example 3: Real-time subscription demo...');
  try {
    const request = await client.submitRequest({
      service_type: 'oracle',
      operation: 'fetch',
      payload: {
        url: 'https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd',
      },
    });

    console.log('  Request submitted:', request.id);
    console.log('  Subscribing to updates...');

    const subscription = client.subscribeToRequest(request.id!, (updated) => {
      console.log('  Update received:', updated.status);
      if (updated.status === 'completed') {
        console.log('  ✓ Completed via real-time!');
        subscription.unsubscribe();
      }
    });

    // Also wait traditionally
    await client.waitForCompletion(request.id!);
    subscription.unsubscribe();
  } catch (error) {
    console.error('Error:', error);
  }

  console.log('\n=== Example Complete ===');
}

// Run if executed directly
main().catch(console.error);

export { ServiceLayerClient, OracleService };
