# Specification: Redesign gemara-publish-action to wrap grcli

## Document overview

This specification describes the redesign of the `gemara-publish-action`
composite GitHub Action to wrap the `grcli` CLI instead of shipping an
embedded Go CLI (`cmd/grc/`). It covers the current user base, the
capability gap between `grcli` and the old action, backward compatibility
requirements, and the design options.

**Related:**
- Issue: [#29 — Gemara Publish Action Redesign](https://github.com/gemaraproj/gemara-publish-action/issues/29)
- Draft PR: [#30](https://github.com/gemaraproj/gemara-publish-action/pull/30) (hub-only path; does not address backward compatibility)
- Sub-issues: #31 (CI enablement), #32 (clientkit compatibility)

## Background

The action currently ships `cmd/grc/`, a Go CLI that calls the go-gemara
SDK (`bundle.Assemble` + `bundle.Pack` + `oras.Copy`) to publish Gemara
bundles to any OCI registry. The `grcli` project provides a standalone
CLI with additional capabilities (in-process signing, provenance, hub
integration) and the community agreed to converge on it
(Gemara Community Meeting 09/03/26).

## Current users

GitHub code search identifies 10 repositories using this action across
two organizations:

### complytime/complytime-policies (v0.2.1)

Production workflow. Publishes policy bundles to **GHCR** (source) and
promotes to **Quay** (destination). Uses the full feature set:

- `registry: ghcr.io` + `username` + `password` (GITHUB_TOKEN)
- `validate: "true"`, `bundle_version: "1"`
- `sign_source: "true"`, `verify_source: "true"`
- `promote_to_destination: "true"` with `destination_registry: quay.io`
- `trust_mode: resign`, `sign_destination: "true"`, `verify_destination: "true"`
- Outputs consumed: `source_ref`, `destination_ref`, `verified_destination`
- Post-publish: ORAS verify manifest, retrieve layers, apply semver tags

### complytime-labs/* (8 repos, v0.1.1)

Internal/dev workflows across `complytime-policies-internal`,
`friendly-goggles`, `le-pamplemouse`, `supreme-pancake`, `glowing-spoon`,
`fluffy-couscous-uat2`, `fluffy-couscous-uat`, `fluffy-couscous`.

All use the same pattern:

- `registry: ghcr.io` + `username` + `password` (GITHUB_TOKEN)
- `validate: "true"`, `sign_source: "true"`, `verify_source: "true"`
- `promote_to_destination: "false"` (GHCR-only, no promotion)
- Post-publish: ORAS verify manifest, auto-bump semver tags

### Key observation

**Every known caller pushes to GHCR using `registry` + `username` +
`password`.** No caller uses grc.store or the hub. These are plain OCI
registry pushes with cosign signing. Breaking this flow is not acceptable.

## Capability comparison

### What the old action does (via `cmd/grc/`)

1. Caller supplies `registry`, `repository`, `tag`, `username`, `password`
2. `grc` CLI runs `bundle.Assemble` + `bundle.Pack` + `oras.Copy` to push
   directly to the caller's registry (GHCR, Quay, ECR, any OCI registry)
3. Cosign signs/verifies the source digest (external `cosign` binary)
4. Optional cross-registry promotion via ORAS copy + cosign resign

No hub involved. No OIDC token required for the publish itself (only for
cosign keyless signing).

### What grcli does

1. Discovers the registry via hub (`GET <url>/.well-known/grc-store-configuration`)
2. Authenticates via hub-minted OCI token (from OIDC or stored credentials)
3. Packs bundle with SLSA provenance, pushes to the **hub-managed registry**
4. Signs in-process via sigstore-go (no external cosign)
5. Notifies hub (`POST /v1/bundles/sync`) to index the bundle

grcli **requires a hub**. Without `--url`, it errors:
`"--url is required (use --dry-run to skip push)"`. There is no
`--registry` flag to push directly to a plain OCI registry.

### The gap

| Capability | Old action | grcli | Gap |
|---|---|---|---|
| Push to any OCI registry | Yes | No (hub-only) | **Breaking** |
| Registry auth via username/password | Yes | Yes (GRCLI_REGISTRY_*) | OK, but hub still needed for discovery |
| Tag from caller input | Yes (`tag` input) | No (derived from `metadata.version`) | **Breaking** — callers use `:latest` |
| Cosign source signing | Yes (external cosign) | Yes (in-process sigstore-go) | OK — improvement |
| SLSA provenance | No | Yes | OK — new capability |
| Hub indexing | No | Yes | OK — new capability |
| Cross-registry promotion | Yes (action handles) | No (action must still handle) | Same |
| CUE validation | Yes (manual cue vet) | Yes (`grcli validate`) | OK |

### Critical gaps

1. **Plain registry push.** All 10 known callers push to GHCR with basic
   credentials. grcli cannot do this without a hub.

2. **Caller-controlled tag.** callers use `tag: latest` (mutable) and apply
   semver tags post-publish via ORAS. grcli forces
   `tag = metadata.version` (immutable) with no override.

## Design options

### Option A: grcli upstream — add `--registry` flag

Add a `--registry` flag to grcli that bypasses hub discovery and pushes
directly to the given registry host. When `--registry` is set:
- Hub discovery is skipped
- Hub sync is skipped
- Registry auth uses `GRCLI_REGISTRY_USERNAME/PASSWORD` or `--token`
- Tag is still derived from `metadata.version` (or add `--tag` override)

**Pros:** Single publish path. All grcli capabilities (provenance, signing)
work on any registry. The action stays a thin wrapper.

**Cons:** Requires an upstream grcli change. May conflict with grcli's
hub-centric design philosophy. The `--tag` override question is
contentious (the hub enforces `tag == metadata.version`).

### Option B: Dual-path action

Keep both publish paths in the action:
- **Hub path:** Use grcli for hub-mediated publish (signing, provenance,
  indexing). Caller sets `grcli_url`.
- **Direct path:** Use the old go-gemara SDK approach (or plain ORAS push)
  for callers who supply `registry` + `username` + `password`. No hub,
  no grcli for the publish step. Cosign signs externally.

The action detects which path to use based on inputs:
- If `registry` is set → direct path
- If `grcli_url` is set (or default hub) → hub path

**Pros:** Backward compatible. Existing callers don't change anything.
New callers can opt into grcli/hub features.

**Cons:** Two publish paths to maintain. The "thin wrapper" simplicity is
lost. The direct path would either keep `cmd/grc/` (defeating the purpose
of #29) or replicate the pack-and-push logic in bash/ORAS.

### Option C: grcli `--dry-run` + ORAS push

Use grcli only for validation and local packing (`--dry-run`), then push
the OCI layout to the caller's registry via ORAS:

```
grcli publish -f file.yaml --license Apache-2.0 --dry-run --output layout/
oras copy --from-oci-layout layout:tag registry/repo:tag
cosign sign registry/repo@digest
```

**Pros:** No grcli upstream change needed. Works with any OCI registry.
Preserves grcli's pack + provenance logic. Callers get provenance in the
bundle even on plain registries.

**Cons:** Signing is external (cosign, not in-process sigstore-go). Hub
sync doesn't happen (the bundle isn't indexed). Two-step publish (grcli
dry-run + ORAS push) is more complex than the old single-step grc CLI.
Tag must be handled by the action (grcli dry-run output uses
`metadata.version` as the layout tag).

### Option D: Accept the breaking change

Document that v2 of the action requires grc.store. Existing plain-registry
callers should either:
- Use grcli directly (without the action) with `--dry-run` + ORAS push
- Migrate to grc.store for hub-managed publishing
- Stay on v1 of the action (pinned SHA)

**Pros:** Simplest action design. Clean break.

**Cons:** Breaks all 10 known callers. The action becomes less useful (just
a wrapper around `grcli publish` that callers could run directly).

## Recommendation

**Option C** (`grcli --dry-run` + ORAS push) is the most practical path:

1. It works today — no upstream grcli change needed.
2. It preserves backward compatibility — callers keep their
   `registry`/`username`/`password` inputs.
3. Bundles get grcli's provenance and packing logic even on plain
   registries.
4. The hub path can be added later (Option A) when grcli supports
   `--registry`, without breaking the direct path.

The action would have two modes:

- **Direct mode** (default, backward compatible): `grcli publish --dry-run`
  → ORAS push → cosign sign. Caller supplies `registry`, `repository`,
  `tag`, `username`, `password`.
- **Hub mode** (opt-in): `grcli publish` with `--url`. Caller sets
  `grcli_url` and `permissions: id-token: write`. No registry
  credentials needed.

However, this is a recommendation. The decision should be made with the
maintainers (`jpower432`) given their understanding of the project's
direction and whether grc.store adoption is the intended path for all
users.

## Test matrix

Before declaring any option complete, these scenarios must pass against
a real registry (not just dry-run):

| # | Scenario | Registry | Auth | Sign | Promote |
|---|----------|----------|------|------|---------|
| 1 | GHCR-only publish (complytime-labs pattern) | ghcr.io | username + GITHUB_TOKEN | cosign keyless | No |
| 2 | GHCR + Quay promotion (complytime pattern) | ghcr.io → quay.io | GITHUB_TOKEN + Quay robot | cosign keyless + resign | Yes |
| 3 | Local registry (integration test) | localhost:5000 | none | --no-sign | No |
| 4 | Hub-mediated publish | hub.grc.store | OIDC | in-process keyless | No |
| 5 | Invalid inputs | -- | -- | -- | -- |
| 6 | Tag override (`:latest` vs `metadata.version`) | any | any | any | No |

Test 1 and 2 are the most critical — they represent the actual callers.
