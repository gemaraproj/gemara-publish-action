---
layout: page
title: Replace embedded grc CLI with grcli dry-run and ORAS push
---

- **ADR:** 0004
- **Status:** Accepted
- **Supersedes:** ADR-0002 (the SDK-owned-semantics principle remains; the mechanism changes)

## Context

The action shipped an embedded Go CLI (`cmd/grc/`) calling go-gemara's
`bundle.Assemble` + `bundle.Pack` + `oras.Copy`. The `grcli` project now
provides the same packing logic plus SLSA provenance, license validation,
and in-process signing.

All 10 known callers push to plain OCI registries (GHCR, Quay) using
`registry` + `username` + `password`. grcli cannot push to a plain
registry (it requires hub discovery via `--url`). A direct `grcli publish`
wrapper would break every caller.

## Action

Use grcli for packing only (`--dry-run`), then push to the caller's
registry via ORAS:

1. `grcli publish --dry-run --no-sign -f <file> --license <license>` packs
   the bundle into a local OCI layout with SLSA provenance and license
   annotation.
2. `oras copy --from-oci-layout <layout>:<version> <registry>/<repo>:<tag>`
   pushes to the caller's registry with the caller's tag.
3. `cosign sign` handles source signing (same as the old action).

This preserves all existing inputs (`registry`, `tag`, `username`,
`password`, `sign_source`, `verify_source`) while replacing the embedded
Go CLI with grcli.

## Consequences

**Positive:**
- Backward compatible — existing callers add only `license` (new required input).
- Bundles gain SLSA provenance and license annotation (additive).
- No Go toolchain at runtime (grcli is a pre-built binary).
- No upstream grcli change needed for the core flow.
- SDK-owned-semantics boundary preserved — packing delegates to grcli/go-gemara.

**Negative:**
- Source signing remains external cosign, not grcli's in-process sigstore-go.
  Callers who want Sigstore bundle referrers (vs cosign `.sig` tags) need
  grcli to support direct registry push (tracked as a future enhancement).
- `metadata.version` is required by grcli for packing. Callers whose
  artifacts lack it need the `--version` flag (gemaraproj/grcli#3) or must
  add the field to their YAML.
- Two-step publish (grcli dry-run + ORAS copy) is more complex internally
  than the old single-step `grc` CLI.

## Alternatives Considered

**Direct grcli publish (hub-only):** Removes `registry`/`tag`/`password`
inputs and requires grc.store. Breaks all 10 known callers. Rejected.

**Add `--registry` flag to grcli:** Cleanest long-term solution but requires
an upstream change. Can be adopted later without breaking the ORAS-push path.

**Keep `cmd/grc/` alongside grcli:** Defeats the purpose of issue #29
(deduplicate bundling logic). Rejected.
