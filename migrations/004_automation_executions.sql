-- Automation execution log table for auditability/observability.
create table if not exists public.automation_executions (
    id uuid primary key,
    trigger_id uuid not null,
    executed_at timestamptz not null default now(),
    success boolean not null default false,
    error text,
    action_type text,
    action_payload jsonb
);

create index if not exists automation_executions_trigger_id_idx on public.automation_executions (trigger_id);
