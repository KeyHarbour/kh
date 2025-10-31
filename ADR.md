# ADR-001: Language for Key-Harbour CLI — Go vs Rust

Status: Proposed (2025-10-31)
Context: The Key-Harbour CLI will manage Terraform states (list, verify, migrate, lock), integrate with tools, and be distributed across Linux/macOS/Windows to DevOps teams and CI agents.

## Options

Go: Mature Terraform/Vault/Consul ecosystem; static binaries; fast iteration.

Rust: Strong safety/performance; steeper learning curve; thinner HashiCorp ecosystem.

## Decision

Adopt Go for the initial and primary CLI.

## Rationale

Ecosystem fit: HashiCorp tooling and SDKs are Go-first (Terraform providers, HCL, Vault/Consul clients).

Distribution: Single static binaries simplify installs on CI agents and locked-down servers.

Developer velocity: Faster ramp-up, abundant CLI libraries (cobra, viper, retryablehttp), and examples.

Concurrency: Goroutines suffice for parallel state scans/migrations without complex async models.

Release pipeline: goreleaser + Homebrew/Scoop/APT/YUM + codesigning/SBOM streamline supply-chain hygiene.

## Consequences

### Pros

Shorter time-to-MVP and easier onboarding.

Lower ops friction (no runtimes, predictable cross-compiles).

Rich prior art for Terraform-adjacent CLIs.

### Cons

Less strict memory/ownership guarantees than Rust.

Potential larger binaries than Rust’s optimized releases.

If we later need high-perf crypto or extreme throughput, Go may not be ideal.

## Risks & Mitigations

Risk: Runtime errors not caught by Rust-style borrow checker.
Mitigation: Strong unit/integration tests, fuzzing, -race, linters, and safe patterns.

Risk: Tight coupling to Go ecosystem.
Mitigation: Keep a clean service API (OpenAPI/Connect/gRPC). Avoid private Terraform internals.

## Future Considerations

Implement hot paths (e.g., checksum/packing/crypto) as optional Rust libraries (FFI) if profiling warrants.

Revisit language choice in 12 months or upon performance/security triggers.

## Scope

This ADR covers the CLI. SDKs for automation may be published in Python/TypeScript from the same API schema.

Decision Owner: Étienne / Key-Harbour Platform
Review Date: 2026-10-31