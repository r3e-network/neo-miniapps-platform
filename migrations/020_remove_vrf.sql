-- Remove legacy VRF persistence.
-- Randomness is now provided via NeoCompute scripts (optionally anchored on-chain via RandomnessLog).

DROP TABLE IF EXISTS public.vrf_requests CASCADE;

