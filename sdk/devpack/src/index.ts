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
}

export interface OracleModule {
  createRequest(params: OracleRequestParams): ActionHandle;
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
}

export interface OracleRequestParams {
  dataSourceId: string;
  payload?: unknown;
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

export function createOracleRequest(params: OracleRequestParams): ActionHandle {
  return runtime().oracle.createRequest(params);
}

export function registerTrigger(params: TriggerRegisterParams): ActionHandle {
  return runtime().triggers.register(params);
}

export function scheduleAutomation(params: AutomationScheduleParams): ActionHandle {
  return runtime().automation.schedule(params);
}

export function success<T = unknown, TMeta = unknown>(data?: T, meta?: TMeta): DevpackResponse<T, TMeta> {
  return runtime().respond.success(data ?? null, meta);
}

export function failure<T = unknown, TMeta = unknown>(error?: T, meta?: TMeta): DevpackResponse<T, TMeta> {
  return runtime().respond.failure(error ?? null, meta);
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

