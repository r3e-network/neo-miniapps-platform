export interface DevpackContext {
  functionId: string;
  accountId: string;
  [key: string]: unknown;
}

export interface ActionHandle<TMeta = Record<string, unknown>> {
  id: string;
  type: string;
  params: Record<string, unknown>;
  asResult(meta?: TMeta): ActionReference<TMeta>;
}

export interface ActionReference<TMeta = Record<string, unknown>> {
  __devpack_ref__: true;
  id: string;
  type: string;
  meta?: TMeta;
}

export interface SuccessResponse<T = unknown, TMeta = unknown> {
  success: true;
  data: T;
  meta: TMeta | null;
}

export interface FailureResponse<T = unknown, TMeta = unknown> {
  success: false;
  error: T;
  meta: TMeta | null;
}

export type DevpackResponse<T = unknown, TMeta = unknown> =
  | SuccessResponse<T, TMeta>
  | FailureResponse<T, TMeta>;

export interface GasBankModule {
  ensureAccount(params: GasBankEnsureParams): ActionHandle;
  withdraw(params: GasBankWithdrawParams): ActionHandle;
  balance(params?: GasBankBalanceParams): ActionHandle;
  listTransactions(params: GasBankListTransactionsParams): ActionHandle;
}

export interface OracleModule {
  createRequest(params: OracleRequestParams): ActionHandle;
}

export interface RandomModule {
  generate(params: RandomGenerateParams): ActionHandle;
}

export interface PriceFeedsModule {
  recordSnapshot(params: PriceFeedSnapshotParams): ActionHandle;
}

export interface DataFeedsModule {
  submitUpdate(params: DataFeedUpdateParams): ActionHandle;
}

export interface DataStreamsModule {
  publishFrame(params: DataStreamFrameParams): ActionHandle;
}

export interface DataLinkModule {
  createDelivery(params: DataLinkDeliveryParams): ActionHandle;
}

export interface TriggersModule {
  register(params: TriggerRegisterParams): ActionHandle;
}

export interface AutomationModule {
  schedule(params: AutomationScheduleParams): ActionHandle;
}

export interface DevpackRuntime {
  version: string;
  context: DevpackContext;
  setContext(ctx: DevpackContext): void;
  gasBank: GasBankModule;
  oracle: OracleModule;
  random: RandomModule;
  priceFeeds: PriceFeedsModule;
  dataFeeds: DataFeedsModule;
  dataStreams: DataStreamsModule;
  dataLink: DataLinkModule;
  triggers: TriggersModule;
  automation: AutomationModule;
  respond: {
    success<T = unknown, TMeta = unknown>(data?: T, meta?: TMeta): SuccessResponse<T, TMeta>;
    failure<T = unknown, TMeta = unknown>(error?: T, meta?: TMeta): FailureResponse<T, TMeta>;
  };
}

export interface GasBankEnsureParams {
  wallet?: string;
}

export interface GasBankWithdrawParams {
  gasAccountId?: string;
  wallet?: string;
  amount: number;
  to?: string;
  /**
   * RFC3339 timestamp for deferred execution. Cron expressions are not supported yet.
   */
  scheduleAt?: string;
}

export interface GasBankBalanceParams {
  gasAccountId?: string;
  wallet?: string;
}

export interface GasBankListTransactionsParams {
  gasAccountId: string;
  status?: string;
  type?: string;
  limit?: number;
}

export interface OracleRequestParams {
  dataSourceId: string;
  alternateSourceIds?: string[];
  payload?: unknown;
}

export interface RandomGenerateParams {
  length?: number;
  requestId?: string;
}

export interface PriceFeedSnapshotParams {
  feedId: string;
  price: number | string;
  source?: string;
  collectedAt?: string;
}

export interface DataFeedUpdateParams {
  feedId: string;
  roundId: number;
  price: string | number;
  timestamp?: string;
  signer?: string;
  signature?: string;
  metadata?: Record<string, string>;
}

export interface DataStreamFrameParams {
  streamId: string;
  sequence: number;
  payload?: Record<string, unknown>;
  latencyMs?: number;
  status?: string;
  metadata?: Record<string, string>;
}

export interface DataLinkDeliveryParams {
  channelId: string;
  payload: Record<string, unknown>;
  metadata?: Record<string, string>;
}

export interface TriggerRegisterParams {
  type: string;
  rule?: string;
  config?: Record<string, string | number | boolean>;
  enabled?: boolean;
}

export interface AutomationScheduleParams {
  name: string;
  schedule: string;
  description?: string;
  enabled?: boolean;
}

declare const Devpack: DevpackRuntime | undefined;

function runtime(): DevpackRuntime {
  if (typeof Devpack === "undefined") {
    throw new Error("Devpack runtime unavailable. Ensure the function is executed inside the Service Layer environment.");
  }
  return Devpack;
}

export function devpack(): DevpackRuntime {
  return runtime();
}

export function ensureGasAccount(params: GasBankEnsureParams = {}): ActionHandle {
  return runtime().gasBank.ensureAccount(params);
}

export function withdrawGas(params: GasBankWithdrawParams): ActionHandle {
  return runtime().gasBank.withdraw(params);
}

export function balanceGasAccount(params: GasBankBalanceParams = {}): ActionHandle {
  return runtime().gasBank.balance(params);
}

export function listGasTransactions(params: GasBankListTransactionsParams): ActionHandle {
  return runtime().gasBank.listTransactions(params);
}

export function createOracleRequest(params: OracleRequestParams): ActionHandle {
  return runtime().oracle.createRequest(params);
}

export function generateRandom(params: RandomGenerateParams = {}): ActionHandle {
  const next = { ...params };
  if (!next.length || next.length <= 0) {
    next.length = 32;
  }
  return runtime().random.generate(next);
}

export function recordPriceSnapshot(params: PriceFeedSnapshotParams): ActionHandle {
  return runtime().priceFeeds.recordSnapshot(params);
}

export function submitDataFeedUpdate(params: DataFeedUpdateParams): ActionHandle {
  return runtime().dataFeeds.submitUpdate(params);
}

export function publishDataStreamFrame(params: DataStreamFrameParams): ActionHandle {
  return runtime().dataStreams.publishFrame(params);
}

export function createDataLinkDelivery(params: DataLinkDeliveryParams): ActionHandle {
  return runtime().dataLink.createDelivery(params);
}

export function registerTrigger(params: TriggerRegisterParams): ActionHandle {
  return runtime().triggers.register(params);
}

export function scheduleAutomation(params: AutomationScheduleParams): ActionHandle {
  return runtime().automation.schedule(params);
}

export function success<T = unknown, TMeta = unknown>(data?: T, meta?: TMeta): DevpackResponse<T | null, TMeta> {
  const payload = (data === undefined ? null : data) as T | null;
  return runtime().respond.success(payload, meta);
}

export function failure<T = unknown, TMeta = unknown>(error?: T, meta?: TMeta): DevpackResponse<T | null, TMeta> {
  const payload = (error === undefined ? null : error) as T | null;
  return runtime().respond.failure(payload, meta);
}

export const respond = {
  success,
  failure,
};

export const context = new Proxy<DevpackContext>({} as DevpackContext, {
  get(_target, prop: string) {
    return runtime().context[prop];
  },
});

export function currentContext(): DevpackContext {
  return runtime().context;
}

export type { ActionHandle as DevpackActionHandle };
