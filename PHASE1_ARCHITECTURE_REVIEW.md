# Phase 1 – Architecture Consistency Review

Services reviewed: vrf, mixer, automation, secrets, datafeeds, accountpool, confidential, oracle.

## vrf
- Structure compliance: marble/service.go ✓; marble/handlers.go ✓; marble/api.go ✓; marble/types.go ✓; marble/lifecycle.go ✓; supabase/repository.go ✓; supabase/models.go ✓; chain/contract.go ✓; chain/events.go ✓.
- Interface compliance: marble.Service ✓ (ID/Name/Version/Router via embedded base, Start/Stop in `services/vrf/marble/lifecycle.go`); RepositoryInterface ✓ (`services/vrf/supabase/repository.go` implements var _); ServiceChainModule ✓ (`services/vrf/chain/contract.go` defines Module with ServiceType/Initialize and registers in init()).
- Missing components: none.
- Issues found: none.

## mixer
- Structure compliance: marble/service.go ✓; marble/handlers.go ✓; marble/api.go ✓; marble/types.go ✓; marble/lifecycle.go ✓; supabase/repository.go ✓; supabase/models.go ✓; chain/contract.go ✓; chain/events.go ✓.
- Interface compliance: marble.Service ✓; RepositoryInterface ✓ (`services/mixer/supabase/repository.go` var _); ServiceChainModule ✗ (no Module implementation or RegisterServiceChain call under `services/mixer/chain`).
- Missing components: ServiceChainModule implementation/registration for mixer chain package.
- Issues found: Chain package is not registered with the chain registry and lacks ServiceType/Initialize hooks, so automation via `internal/chain` cannot initialize mixer contracts.

## automation
- Structure compliance: marble/service.go ✓; marble/handlers.go ✓; marble/api.go ✓; marble/types.go ✓; marble/lifecycle.go ✓; supabase/repository.go ✓; supabase/models.go ✓; chain/contract.go ✓; chain/events.go ✓.
- Interface compliance: marble.Service ✓; RepositoryInterface ✓ (`services/automation/supabase/repository.go` var _); ServiceChainModule ✗ (no Module or registration in `services/automation/chain`).
- Missing components: ServiceChainModule implementation/registration for automation chain package.
- Issues found: Chain integration cannot be initialized through `internal/chain` registry because no ServiceChainModule is provided.

## secrets
- Structure compliance: marble/service.go ✓; marble/handlers.go ✓; marble/api.go ✓; marble/types.go ✓; marble/lifecycle.go ✓; supabase/repository.go ✓; supabase/models.go ✓; chain/contract.go ✗; chain/events.go ✗.
- Interface compliance: marble.Service ✓; RepositoryInterface ✓; ServiceChainModule ✗ (no chain package).
- Missing components: chain folder with contract.go/events.go and corresponding ServiceChainModule.
- Issues found: Chain integration scaffold absent, so service cannot register chain interactions through `internal/chain`.

## datafeeds
- Structure compliance: marble/service.go ✓; marble/handlers.go ✓; marble/api.go ✓; marble/types.go ✓; marble/lifecycle.go ✓; supabase/repository.go ✗; supabase/models.go ✗; chain/contract.go ✓; chain/events.go ✓.
- Interface compliance: marble.Service ✓; RepositoryInterface ✗ (no supabase package); ServiceChainModule ✗ (chain package lacks Module/registration).
- Missing components: supabase repository/models; ServiceChainModule implementation/registration for datafeeds chain package.
- Issues found: No persistence layer defined for datafeeds; chain package not registered with `internal/chain`, preventing initialization via registry.

## accountpool
- Structure compliance: marble/service.go ✓; marble/handlers.go ✓; marble/api.go ✓; marble/types.go ✓; marble/lifecycle.go ✓; supabase/repository.go ✓; supabase/models.go ✓; chain/contract.go ✗; chain/events.go ✗.
- Interface compliance: marble.Service ✓; RepositoryInterface ✓ (`services/accountpool/supabase/repository.go` var _); ServiceChainModule ✗ (no chain package).
- Missing components: chain folder with contract.go/events.go and ServiceChainModule for accountpool-specific chain interactions.
- Issues found: Chain integration not present, so accountpool cannot be initialized via chain registry.

## confidential
- Structure compliance: marble/service.go ✓; marble/handlers.go ✓; marble/api.go ✓; marble/types.go ✓; marble/lifecycle.go ✓; supabase/repository.go ✗; supabase/models.go ✗; chain/contract.go ✗; chain/events.go ✗.
- Interface compliance: marble.Service ✓; RepositoryInterface ✗ (no supabase package); ServiceChainModule ✗ (no chain package).
- Missing components: supabase repository/models; chain package with contract/events and ServiceChainModule.
- Issues found: No persistence or chain integration scaffolding present for the confidential service.

## oracle
- Structure compliance: marble/service.go ✓; marble/handlers.go ✓; marble/api.go ✓; marble/types.go ✓; marble/lifecycle.go ✓; supabase/repository.go ✗; supabase/models.go ✗; chain/contract.go ✗; chain/events.go ✗ (only C# contract files under `services/oracle/contract`).
- Interface compliance: marble.Service ✓; RepositoryInterface ✗ (no supabase package); ServiceChainModule ✗ (no chain package/registration).
- Missing components: supabase repository/models; Go-based chain package with contract.go/events.go and ServiceChainModule registration.
- Issues found: No supabase persistence layer; chain integration absent (current contract assets are C# and not wired into Go chain registry).
