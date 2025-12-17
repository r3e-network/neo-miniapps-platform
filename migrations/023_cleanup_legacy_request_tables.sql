-- =============================================================================
-- Neo Service Layer - Cleanup Legacy Request Tables
-- =============================================================================
--
-- The current platform uses:
-- - `service_requests` as a generic request/audit log keyed by service ID (string)
-- - `neoflow_triggers` / `neoflow_executions` for automation persistence
--
-- Older schemas introduced per-service request tables (`oracle_requests`,
-- `compute_jobs`) and a wide `service_type` enum. These are out of scope for the
-- current "Supabase Edge + TEE services" architecture and are removed to avoid
-- duplicated or unused functionality.

-- Convert service_requests.service_type from legacy enum → TEXT (service_id string).
DO $$
BEGIN
  IF to_regclass('public.service_requests') IS NOT NULL THEN
    IF EXISTS (
      SELECT 1
      FROM pg_attribute a
      JOIN pg_class c ON c.oid = a.attrelid
      JOIN pg_namespace n ON n.oid = c.relnamespace
      JOIN pg_type t ON t.oid = a.atttypid
      WHERE n.nspname = 'public'
        AND c.relname = 'service_requests'
        AND a.attname = 'service_type'
        AND a.attisdropped = false
        AND t.typname = 'service_type'
    ) THEN
      ALTER TABLE public.service_requests
        ALTER COLUMN service_type TYPE TEXT USING service_type::text;
    END IF;
  END IF;
END $$;

-- Drop the legacy enum type if it still exists (safe after the conversion above).
DROP TYPE IF EXISTS public.service_type;

-- Drop legacy per-service request tables (current services persist via `service_requests`).
DROP TABLE IF EXISTS public.oracle_requests;
DROP TABLE IF EXISTS public.compute_jobs;

