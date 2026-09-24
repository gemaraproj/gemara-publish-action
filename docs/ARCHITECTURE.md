# Architecture: grcli dry-run for packing, ORAS for transport

This repository is a **composite GitHub Action** that publishes Gemara
bundles to any OCI registry. It uses
[grcli](https://github.com/gemaraproj/grcli) for bundle packing
(assembly, provenance, license validation) and ORAS for registry
transport.

## How it works

1. **Install grcli** — pulls a pinned, pre-built binary from
   `ghcr.io/gemaraproj/grcli:<version>` via ORAS.
2. **Validate** (optional) — `grcli validate -f <file>` runs `cue vet`
   against the authoritative Gemara CUE schemas.
3. **Pack** — `grcli publish --dry-run --no-sign -f <file> --license <spdx>`
   assembles the bundle with SLSA provenance and writes an OCI layout to disk.
4. **Push** — `oras copy --from-oci-layout <layout>:<version> <registry>/<repo>:<tag>`
   pushes the bundle to the caller's registry with the caller's tag.
5. **Sign / Verify** — `cosign sign` and `cosign verify` handle source
   digest trust (same as the previous action version).
6. **Promote** (optional) — ORAS copies the artifact to a second registry.
   Cosign re-signs / verifies the destination digest when `trust_mode=resign`.

## Why dry-run + ORAS instead of direct grcli publish

grcli's `publish` command requires a hub (`--url`) for registry discovery.
All 10 known callers push to plain OCI registries (GHCR, Quay) using
`registry` + `username` + `password`. A direct `grcli publish` wrapper
would break them. See [ADR-0004](adr/0004-wrap-grcli.md).

The dry-run approach uses grcli for what it does well (packing, provenance,
license validation) and delegates transport to ORAS, which works with any
OCI registry.

## Intended split

| Concern | Owner | Notes |
|---------|-------|-------|
| Bundle assembly, packing, manifest shape | go-gemara SDK (via grcli) | `grcli publish --dry-run` |
| SLSA provenance | grcli | Embedded in bundle manifest config |
| License validation | grcli | SPDX canonicalization before packing |
| CUE schema validation | grcli | `grcli validate` wraps `cue vet` |
| Registry transport (push) | This action (ORAS) | `oras copy --from-oci-layout` |
| Source signing / verification | This action (cosign) | Keyless cosign sign/verify |
| Cross-registry promotion | This action (ORAS + cosign) | Copy + optional resign |
| CI orchestration | This action (`action.yml`) | Input validation, step sequencing, outputs |

## Design decisions

- [ADR-0001: Composite action pattern](adr/0001-composite-action-pattern.md)
- [ADR-0002: SDK-owned bundle contract](adr/0002-sdk-owned-bundle-contract.md) (superseded by ADR-0004)
- [ADR-0003: Cross-registry trust model](adr/0003-cross-registry-trust-model.md)
- [ADR-0004: Replace embedded grc CLI with grcli dry-run and ORAS push](adr/0004-wrap-grcli.md)

## Compliance

- **Do not** add Gemara YAML schema ownership or layer `mediaType` tables here — use **grcli** / **go-gemara**.
- **Do** pin `grcli_version` and `oras_version` and document migration.
