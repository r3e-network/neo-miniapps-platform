-- =============================================================================
-- Neo Service Layer - Cleanup Legacy Mixer Tables
-- Removes outdated Tornado Cash style tables that are no longer used
-- Current architecture: Off-Chain First with On-Chain Dispute Only
-- =============================================================================

-- Drop legacy Tornado Cash style tables (no longer used in current architecture)
-- These tables were part of the old commitment/nullifier based mixing approach
-- Current architecture uses off-chain mixing with TEE proofs and on-chain dispute only

-- Drop mixer_nullifiers (old nullifier tracking for double-spend prevention)
DROP TABLE IF EXISTS public.mixer_nullifiers CASCADE;

-- Drop mixer_commitments (old commitment tracking for deposits)
DROP TABLE IF EXISTS public.mixer_commitments CASCADE;

-- Drop mixer_pools (old pool configuration with merkle roots)
DROP TABLE IF EXISTS public.mixer_pools CASCADE;

-- Add comment explaining the migration
COMMENT ON TABLE public.mixer_requests IS 'Mixer requests using off-chain first architecture with TEE proofs. Legacy Tornado Cash tables (mixer_pools, mixer_commitments, mixer_nullifiers) have been removed.';
