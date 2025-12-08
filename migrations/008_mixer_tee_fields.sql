-- =============================================================================
-- Neo Service Layer - Mixer TEE Commitment Fields
-- Adds missing columns for off-chain first architecture with dispute mechanism
-- =============================================================================

-- Add missing columns to mixer_requests table for TEE commitment and dispute support
ALTER TABLE IF EXISTS public.mixer_requests
    ADD COLUMN IF NOT EXISTS user_address TEXT,
    ADD COLUMN IF NOT EXISTS token_type TEXT NOT NULL DEFAULT 'GAS',
    ADD COLUMN IF NOT EXISTS request_hash TEXT,
    ADD COLUMN IF NOT EXISTS tee_signature TEXT,
    ADD COLUMN IF NOT EXISTS deadline BIGINT,
    ADD COLUMN IF NOT EXISTS output_tx_ids JSONB DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS completion_proof_json TEXT;

-- Create index on request_hash for dispute lookups
CREATE INDEX IF NOT EXISTS mixer_requests_request_hash_idx
    ON public.mixer_requests (request_hash)
    WHERE request_hash IS NOT NULL;

-- Create index on deadline for expiry checks
CREATE INDEX IF NOT EXISTS mixer_requests_deadline_idx
    ON public.mixer_requests (deadline)
    WHERE deadline IS NOT NULL;

-- Add comment explaining the TEE commitment fields
COMMENT ON COLUMN public.mixer_requests.request_hash IS 'Hash256(canonical request bytes) for TEE commitment';
COMMENT ON COLUMN public.mixer_requests.tee_signature IS 'TEE signature over request_hash for dispute proof';
COMMENT ON COLUMN public.mixer_requests.deadline IS 'Unix timestamp for dispute deadline (7 days from creation)';
COMMENT ON COLUMN public.mixer_requests.output_tx_ids IS 'Array of output transaction hashes after delivery';
COMMENT ON COLUMN public.mixer_requests.completion_proof_json IS 'JSON serialized CompletionProof (stored, not submitted unless disputed)';
COMMENT ON COLUMN public.mixer_requests.token_type IS 'Token type: GAS, NEO, etc.';
COMMENT ON COLUMN public.mixer_requests.user_address IS 'User wallet address for the request';
