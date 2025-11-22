# Documentation Index

This repository treats the [Neo Service Layer Specification](requirements.md)
as the single source of truth for the platform. All previous supporting docs
have been consolidated into that specification.

## Primary Reference
- [Neo Service Layer Specification](requirements.md)
- [Service Layer Review Checklist](review-checklist.md)
- [CLI Quick Reference](../README.md#cli-quick-reference)
- JAM integration notes:
  - [Polkadot JAM-inspired integration](polkadot-jam-integration-design.md)
  - [JAM hardening and auth/quotas](jam-hardening.md), [jam-auth-and-quotas.md](jam-auth-and-quotas.md), [jam-hardening-implementation.md](jam-hardening-implementation.md)

All other documents have been retired to avoid drift. Update the specification
directly when behaviour changes so the documentation remains clean, clear, and
consistent.

## Working With The Specification
- Start every change by updating [`requirements.md`](requirements.md); it is the review contract.
- Capture problem statements, API surfaces, storage changes, and operational needs before writing code.
- Link related implementation files so future contributors can navigate from the spec into the codebase.
- Keep examples and sample payloads realistic—tests and SDK snippets should mirror the documented flows.

## Retired Artifacts
- The historical LaTeX/PDF export under `spec/` has been deleted. Markdown is the
  only maintained format going forward.
- Any new documentation should live in this directory unless explicitly called out
  elsewhere (e.g., generated SDK docs). This ensures a single source of truth and
  avoids drift between different formats.
