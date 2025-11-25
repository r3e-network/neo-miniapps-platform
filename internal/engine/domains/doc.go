// Package domains provides domain-specific engine interfaces that extend the base ServiceModule contract.
//
// # Architecture Overview
//
// The domain engines represent optional, specialized extension points for service modules that operate
// in specific blockchain verticals such as DeFi, GameFi, and NFTs. These interfaces build upon the
// core engine.ServiceModule interface to provide standardized contracts for domain-specific operations.
//
// # Design Philosophy
//
// Domain engines follow these key principles:
//
//   - Optional Extensions: Services are not required to implement domain interfaces. They extend
//     ServiceModule only when they provide domain-specific capabilities.
//
//   - Composable Interfaces: Each domain is decomposed into fine-grained sub-interfaces (e.g.,
//     TokenEngine, SwapEngine, LiquidityEngine for DeFi) allowing services to implement only
//     the capabilities they support.
//
//   - Capability Discovery: Services can advertise their capabilities through marker interfaces
//     (e.g., SwapCapable, MintCapable) enabling runtime feature detection.
//
//   - Standard Types: Domains define common data structures (Token, NFT, GameAsset) to ensure
//     interoperability between different service implementations.
//
// # Current Status
//
// The domain interfaces are fully defined with comprehensive type systems and method signatures.
// These serve as architectural blueprints for service implementations. Concrete implementations
// will be developed in internal/services/ as domain-specific services are built out.
//
// # Available Domains
//
//   - defi:    Decentralized finance operations including token swaps, liquidity pools, lending,
//              staking, and yield farming.
//
//   - gamefi:  Blockchain gaming primitives including player management, game assets, matches,
//              leaderboards, achievements, quests, and tournaments.
//
//   - nft:     Non-fungible token operations covering collections, minting, marketplace trading,
//              metadata management, royalties, and search/discovery.
//
//   - rwa:     Real-world asset tokenization covering asset management, tokenization, compliance,
//              custody, investor management, document handling, distributions, and redemptions.
//
// # Implementation Pattern
//
// Services implementing domain engines should:
//
//  1. Embed the domain interface in their service struct
//  2. Implement the base ServiceModule methods (Name, Domain, Start, Stop)
//  3. Implement the composite domain interface method (e.g., DeFiInfo(), GameFiInfo(), NFTInfo())
//  4. Optionally implement sub-interfaces for specific capabilities
//  5. Use capability markers to advertise supported features
//
// Example structure:
//
//	type MyDeFiService struct {
//	    // ... fields
//	}
//
//	func (s *MyDeFiService) Name() string { return "my-defi" }
//	func (s *MyDeFiService) Domain() string { return "defi" }
//	func (s *MyDeFiService) DeFiInfo() defi.DeFiInfo { ... }
//	func (s *MyDeFiService) HasSwapEngine() bool { return true }
//	func (s *MyDeFiService) SwapEngine() defi.SwapEngine { return s }
//
// # Future Expansion
//
// Additional domains may be added following the same pattern. Candidates include:
//   - identity: Decentralized identity and attestation
//   - dao:      Decentralized autonomous organization primitives
//   - social:   Social graph and reputation systems
//   - bridge:   Cross-chain asset bridging and messaging
//
// Each new domain should provide comprehensive interfaces, standard types, and capability markers
// while maintaining backward compatibility with existing service implementations.
package domains
