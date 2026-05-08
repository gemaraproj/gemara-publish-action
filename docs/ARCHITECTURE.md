# Architecture: SDK-owned semantics, action-owned publish orchestration

This repository is **not** the source of truth for Gemara OCI semantics. That belongs in
**[go-gemara](https://github.com/gemaraproj/go-gemara)** and
**[go-gemara#60](https://github.com/gemaraproj/go-gemara/issues/60)** (manifest shape, media
types, Assemble/Pack/Unpack, `oras.Copy` from packed store to registry).

## Intended split (upstream: go-gemara)

Conceptually:

1. **`bundle.Assemble`** (SDK) resolves the dependency tree from a root Gemara YAML.
2. **`bundle.Pack`** (SDK) produces an OCI store / artifact.
3. **`oras.Copy`** (SDK, via `oras-go/v2`) moves that artifact to a **remote registry** with a tag.
4. **`bundle.Unpack`** (SDK) on the consumer pulls and reads it.

The GitHub Action provides what CI needs as a standardized contract in **`action.yml`**:

- Secrets and registry authentication for source and destination registries.
- The Gemara Registry CLI (`cmd/grc/main.go`) that wires `bundle.Assemble` + `bundle.Pack` +
  `oras.Copy` from the go-gemara SDK.
- Keyless cosign sign/verify for source and destination digests.
- Optional cross-registry promotion (caller supplies source + destination registry coordinates) with
  destination re-sign trust (defaults).
- Structured `GITHUB_OUTPUT` values for source/destination refs and verification state.

## Publish and promotion model

| Concern | In action | Notes |
|---------|-----------|-------|
| Publish source | go-gemara SDK via `grc` CLI | Caller provides root Gemara YAML; CLI runs Assemble + Pack + oras.Copy. |
| Source trust | `sign_source`, `verify_source` | Keyless cosign against source digest. |
| Promotion | `promote_to_destination`, `destination_*` | Copies source tag to destination tag on another registry host. |
| Destination trust | `trust_mode` + destination sign/verify toggles | Standard path uses `resign`; `copy-referrers` remains optional compatibility mode. |

## Why this keeps SDK boundaries intact

This action orchestrates publishing steps but does not redefine layer descriptors or media types.
The `grc` CLI delegates all bundle semantics (assembly, packing, manifest construction) to
go-gemara's public API.

## Design decisions

Architectural decisions are recorded in [`docs/adr/`](adr/):

- [ADR-0001: Composite action pattern](adr/0001-composite-action-pattern.md)
- [ADR-0002: SDK-owned bundle contract](adr/0002-sdk-owned-bundle-contract.md)
- [ADR-0003: Cross-registry trust model](adr/0003-cross-registry-trust-model.md)

## Compliance

- **Do not** add Gemara YAML schema ownership or layer `mediaType` tables here — use **go-gemara**.
- **Do** pin versions (`oras_version`, `cosign_version`, and action refs) and document migration.
