/**
 * Neo N3 MiniApp Platform SDK - Usage Guide
 *
 * This file captures production-oriented usage patterns for the MiniApp SDK.
 * It shows how to interact with the platform services end-to-end.
 */

import { createMiniAppSDK, createHostSDK } from "../src";
import type {
  MiniAppSDK,
  HostSDK,
  PayGASResponse,
  VoteNEOResponse,
  RNGResponse,
  PriceResponse,
  GasBankAccountResponse,
} from "../src/types";

// =============================================================================
// Configuration
// =============================================================================

const requireEnv = (name: string): string => {
  const value = String(process.env[name] ?? "").trim();
  if (!value) {
    throw new Error(`${name} is required`);
  }
  return value;
};

const EDGE_BASE_URL = requireEnv("EDGE_BASE_URL");
const AUTH_TOKEN = requireEnv("SUPABASE_AUTH_JWT");
const HOST_API_KEY = requireEnv("HOST_API_KEY");
const MINIAPP_APP_ID = requireEnv("MINIAPP_APP_ID");
const AUTOMATION_WEBHOOK_URL = requireEnv("AUTOMATION_WEBHOOK_URL");
const SECRET_NAME = requireEnv("SECRET_NAME");
const SECRET_VALUE = requireEnv("SECRET_VALUE");

const SDK_CONFIG = {
  edgeBaseUrl: EDGE_BASE_URL,
  // For authenticated requests, provide a token getter.
  getAuthToken: async () => AUTH_TOKEN,
};

// =============================================================================
// Workflow 1: Basic MiniApp SDK Initialization
// =============================================================================

async function initializeSDK(): Promise<MiniAppSDK> {
  const sdk = createMiniAppSDK(SDK_CONFIG);
  console.log("SDK initialized successfully");
  return sdk;
}

// =============================================================================
// Workflow 2: Host SDK Initialization
// =============================================================================

async function initializeHostSDK(): Promise<HostSDK> {
  const host = createHostSDK({
    edgeBaseUrl: EDGE_BASE_URL,
    getAPIKey: async () => HOST_API_KEY,
  });
  console.log("Host SDK initialized successfully");
  return host;
}

// =============================================================================
// Workflow 3: Get User Wallet Address
// =============================================================================

async function getWalletAddress(sdk: MiniAppSDK): Promise<string> {
  try {
    const address = await sdk.wallet.getAddress();
    console.log("User wallet address:", address);
    return address;
  } catch (error) {
    console.error("Failed to get wallet address:", error);
    throw error;
  }
}

// =============================================================================
// Workflow 4: Pay with GAS (Payment Flow)
// =============================================================================

async function payWithGAS(sdk: MiniAppSDK, appId: string, amount: string, memo?: string): Promise<PayGASResponse> {
  try {
    console.log(`Initiating GAS payment: ${amount} GAS to app ${appId}`);

    // Create payment intent via the gateway
    const response = await sdk.payments.payGAS(appId, amount, memo);

    console.log("Payment intent created:", {
      requestId: response.request_id,
      intent: response.intent,
      constraints: response.constraints,
    });

    // The response contains an invocation intent that can be submitted to the wallet
    // In a real app, you would use the wallet to sign and submit this transaction
    console.log("Invocation details:", response.invocation);

    return response;
  } catch (error) {
    console.error("Payment failed:", error);
    throw error;
  }
}

// =============================================================================
// Workflow 5: Vote with NEO (Governance Flow)
// =============================================================================

async function voteWithNEO(
  sdk: MiniAppSDK,
  appId: string,
  proposalId: string,
  neoAmount: string,
  support: boolean = true,
): Promise<VoteNEOResponse> {
  try {
    console.log(`Voting on proposal ${proposalId} with ${neoAmount} NEO`);

    const response = await sdk.governance.vote(appId, proposalId, neoAmount, support);

    console.log("Vote intent created:", {
      requestId: response.request_id,
      intent: response.intent,
      constraints: response.constraints,
    });

    return response;
  } catch (error) {
    console.error("Vote failed:", error);
    throw error;
  }
}

// =============================================================================
// Workflow 6: Request Random Number (RNG)
// =============================================================================

async function requestRandomNumber(sdk: MiniAppSDK, appId: string): Promise<RNGResponse> {
  try {
    console.log("Requesting random number...");

    const response = await sdk.rng.requestRandom(appId);

    console.log("Random number received:", {
      requestId: response.request_id,
      randomness: response.randomness,
      signature: response.signature,
      publicKey: response.public_key,
      attestationHash: response.attestation_hash,
    });

    return response;
  } catch (error) {
    console.error("RNG request failed:", error);
    throw error;
  }
}

// =============================================================================
// Workflow 7: Get Price Feed Data
// =============================================================================

async function getPriceData(sdk: MiniAppSDK, symbol: string): Promise<PriceResponse> {
  try {
    console.log(`Fetching price for ${symbol}...`);

    const response = await sdk.datafeed.getPrice(symbol);

    console.log("Price data received:", {
      feedId: response.feed_id,
      pair: response.pair,
      price: response.price,
      decimals: response.decimals,
      timestamp: response.timestamp,
      sources: response.sources,
    });

    return response;
  } catch (error) {
    console.error("Price fetch failed:", error);
    throw error;
  }
}

// =============================================================================
// Workflow 8: Host SDK - GasBank Operations
// =============================================================================

async function gasBankOperations(hostSdk: HostSDK): Promise<void> {
  try {
    // Get account balance
    console.log("Fetching GasBank account...");
    const accountResponse: GasBankAccountResponse = await hostSdk.gasbank.getAccount();

    console.log("GasBank Account:", {
      id: accountResponse.account.id,
      balance: accountResponse.account.balance,
      reserved: accountResponse.account.reserved,
      available: accountResponse.account.available,
    });

    // List deposits
    console.log("Fetching deposit history...");
    const depositsResponse = await hostSdk.gasbank.listDeposits();
    console.log(`Found ${depositsResponse.deposits.length} deposits`);

    // List transactions
    console.log("Fetching transaction history...");
    const transactionsResponse = await hostSdk.gasbank.listTransactions();
    console.log(`Found ${transactionsResponse.transactions.length} transactions`);
  } catch (error) {
    console.error("GasBank operation failed:", error);
    throw error;
  }
}

// =============================================================================
// Workflow 9: Host SDK - Secrets Management
// =============================================================================

async function secretsManagement(hostSdk: HostSDK): Promise<void> {
  try {
    // List secrets
    console.log("Listing secrets...");
    const listResponse = await hostSdk.secrets.list();
    console.log(`Found ${listResponse.secrets.length} secrets`);

    // Upsert a secret
    console.log("Creating/updating secret...");
    const upsertResponse = await hostSdk.secrets.upsert(SECRET_NAME, SECRET_VALUE);
    console.log("Secret upserted:", {
      name: upsertResponse.secret.name,
      version: upsertResponse.secret.version,
      created: upsertResponse.created,
    });

    // Get a secret
    console.log("Retrieving secret...");
    const getResponse = await hostSdk.secrets.get(SECRET_NAME);
    console.log("Secret retrieved:", {
      name: getResponse.name,
      version: getResponse.version,
    });

    // Set permissions
    console.log("Setting secret permissions...");
    await hostSdk.secrets.setPermissions(SECRET_NAME, ["neofeeds", "neooracle"]);
    console.log("Permissions updated");
  } catch (error) {
    console.error("Secrets operation failed:", error);
    throw error;
  }
}

// =============================================================================
// Workflow 10: Host SDK - Automation Triggers
// =============================================================================

async function automationTriggers(hostSdk: HostSDK): Promise<void> {
  try {
    // Create a time-based trigger
    console.log("Creating automation trigger...");
    const trigger = await hostSdk.automation.createTrigger({
      name: "Daily Price Check",
      trigger_type: "schedule",
      schedule: "0 0 * * *", // Daily at midnight
      action: {
        type: "webhook",
        url: AUTOMATION_WEBHOOK_URL,
      },
    });
    console.log("Trigger created:", trigger.id);

    // List triggers
    const triggers = await hostSdk.automation.listTriggers();
    console.log(`Found ${triggers.length} triggers`);

    // Enable/disable trigger
    await hostSdk.automation.enableTrigger(trigger.id);
    console.log("Trigger enabled");

    // Get execution history
    const executions = await hostSdk.automation.listExecutions(trigger.id, 10);
    console.log(`Found ${executions.length} executions`);
  } catch (error) {
    console.error("Automation operation failed:", error);
    throw error;
  }
}

// =============================================================================
// Workflow 11: Complete MiniApp Flow
// =============================================================================

async function completeMiniAppFlow(): Promise<void> {
  console.log("=== Neo N3 MiniApp Platform SDK Run ===\n");

  // Initialize SDK
  const sdk = await initializeSDK();

  // Get wallet address
  const address = await getWalletAddress(sdk);
  console.log(`Connected wallet: ${address}\n`);

  // Get price data
  const price = await getPriceData(sdk, "NEO-USD");
  console.log(`Current NEO price: $${price.price}\n`);

  // Request random number for a game
  const rng = await requestRandomNumber(sdk, MINIAPP_APP_ID);
  console.log(`Random number for game: ${rng.randomness}\n`);

  // Create a payment intent
  const payment = await payWithGAS(sdk, MINIAPP_APP_ID, "1.0", "Game purchase");
  console.log(`Payment intent: ${payment.request_id}\n`);

  console.log("=== Run Complete ===");
}

// =============================================================================
// Run Workflows
// =============================================================================

// Uncomment to run:
// completeMiniAppFlow().catch(console.error);

export {
  initializeSDK,
  initializeHostSDK,
  getWalletAddress,
  payWithGAS,
  voteWithNEO,
  requestRandomNumber,
  getPriceData,
  gasBankOperations,
  secretsManagement,
  automationTriggers,
  completeMiniAppFlow,
};
