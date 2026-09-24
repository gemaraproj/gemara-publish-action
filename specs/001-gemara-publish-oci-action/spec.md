# Feature Specification: Gemara publish OCI composite GitHub Action

## Document overview

This specification describes the **composite** GitHub Action shipped from this repository
(`action.yml`): pack a root Gemara YAML into an OCI bundle via grcli, push to any OCI
registry via ORAS, sign/verify with cosign, and optionally promote to a second registry.

**Key metadata**

- **Action definition:** `action.yml` (composite)
- **Related design:** [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md), [docs/adr/](../../docs/adr/)

**Scope boundary:** This repository owns transport and trust orchestration. Bundle packing,
provenance, and schema validation are delegated to
[grcli](https://github.com/gemaraproj/grcli) and
[go-gemara](https://github.com/gemaraproj/go-gemara).

## Background and motivation

CI callers need a **small, auditable** Action that:

1. Accepts a root Gemara artifact YAML (Policy, Catalog, or Guidance) and publishes it as an OCI
   bundle to any registry the caller specifies.
2. Uses grcli for packing (assembly, SLSA provenance, license validation) without requiring a hub.
3. Authenticates to the registry using **secrets the workflow supplies**, without echoing tokens.
4. Optionally validates the YAML against the Gemara CUE schemas via `grcli validate`.
5. Optionally signs and verifies the published digest with keyless cosign.
6. Optionally promotes the bundle to a second registry with configurable trust modes.
7. Emits stable outputs for source/destination refs, digests, and verification state.

## Core user scenarios

### Priority 1: Full publish orchestration for callers

A maintainer calls the action once with publish settings and trust settings; the action
packs the bundle via `grcli publish --dry-run`, pushes to the caller's registry via ORAS,
signs/verifies the source digest with cosign, optionally promotes to a destination registry,
and returns source/destination outputs.

**Test coverage:** `.github/workflows/ci.yml` installs grcli, runs `grcli validate` and
`grcli publish --dry-run` against `testdata/minimal-catalog.yaml`, and verifies the OCI
layout structure.

### Priority 2: Destination trust behavior

Callers can choose trust mode:

- `copy-only`
- `copy-referrers`
- `resign` (default)

and verify destination trust with the same workflow identity constraints.

## Edge cases addressed

- **Missing or invalid root YAML:** `grcli validate` fails before publish.
- **Missing license:** Fail with a clear error (grcli requires `--license`).
- **Missing password:** Fail with a clear error.
- **Missing metadata.version:** grcli requires this field for packing. A `--version` CLI
  fallback is tracked at [gemaraproj/grcli#3](https://github.com/gemaraproj/grcli/issues/3).
- **Digest resolution:** Use `oras resolve` on source/destination references and fail fast.
- **Username default:** `GITHUB_ACTOR` when username omitted.

## Functional requirements summary

The Action must:

1. **Install grcli** from its GHCR release artifact using ORAS.
2. **Install ORAS** for grcli install, registry push, and promotion copy.
3. **Validate** (optional): Run `grcli validate` against the Gemara CUE spec.
4. **Pack:** Run `grcli publish --dry-run --no-sign` to assemble the bundle with SLSA
   provenance and license annotation into a local OCI layout.
5. **Push:** Run `oras copy --from-oci-layout` to push the bundle to the caller's registry
   with the caller's tag.
6. **Optional sign/verify:** Keyless cosign sign and verify on the source digest.
7. **Optional promotion:** Copy source to destination registry with selected trust mode and
   destination sign/verify.
8. **Output contract:** Append source/destination refs, digests, and verification booleans to
   `GITHUB_OUTPUT`.

## Scope boundaries

**In scope:** grcli install, ORAS install, grcli validate/pack, ORAS push, registry auth,
sign/verify orchestration, optional promotion, structured outputs.

**Out of scope:** Gemara YAML schema ownership, layer `mediaType` tables, Pack/Unpack
implementation details, signing implementation during packing (these live in grcli / go-gemara).

## Formal requirements (SHALL / scenarios)

### Requirement: Pinned grcli binary install

The Action SHALL install the grcli binary for the runner platform using ORAS and the
`grcli_version` input as the version selector.

#### Scenario: Default version is used when input omitted

- **WHEN** the workflow invokes the Action without setting `grcli_version`
- **THEN** the Action SHALL install the default grcli version documented in `action.yml`

### Requirement: Pinned ORAS CLI install

The Action SHALL install the ORAS CLI using the `oras_version` input.

#### Scenario: Default version is used when input omitted

- **WHEN** the workflow invokes the Action without setting `oras_version`
- **THEN** the Action SHALL install the default ORAS version documented in `action.yml`

### Requirement: License is required

The Action SHALL require a `license` input (SPDX expression) and SHALL fail before
packing if it is missing.

#### Scenario: Missing license

- **WHEN** `license` is empty
- **THEN** the Action SHALL fail with an error indicating `license` is required

### Requirement: Registry authentication

The Action SHALL authenticate to the registry using credentials supplied by the caller,
and SHALL NOT print the `password` input to logs.

#### Scenario: Registry with password

- **WHEN** `password` is non-empty
- **THEN** ORAS SHALL authenticate and push the bundle to the target registry

#### Scenario: Registry without password is rejected

- **WHEN** `password` is empty
- **THEN** the Action SHALL fail with an error indicating `password` is required

### Requirement: Pack via grcli and push via ORAS

The Action SHALL run `grcli publish --dry-run` to pack the bundle locally, then
`oras copy --from-oci-layout` to push it to the caller's registry with the caller's tag.

#### Scenario: Successful publish

- **WHEN** the root Gemara YAML is valid, registry credentials are correct
- **THEN** the Action SHALL pack the bundle, push it, and emit a digest output

#### Scenario: Validation failure

- **WHEN** `validate` is `"true"` and `grcli validate` fails on the root YAML
- **THEN** the Action SHALL fail before attempting pack or push

### Requirement: Source and destination output contract

After a successful publish, the Action SHALL write source digest/reference outputs. If
promotion is enabled, it SHALL write destination digest/reference outputs.

#### Scenario: Outputs available to downstream steps

- **WHEN** publish completes successfully
- **THEN** `digest`, `source_digest`, and `source_ref` SHALL be non-empty outputs

#### Scenario: Promotion outputs emitted

- **WHEN** `promote_to_destination` is enabled and succeeds
- **THEN** `destination_ref` and `destination_digest` SHALL be emitted and non-empty

### Requirement: Destination trust mode

When `promote_to_destination` is enabled, the Action SHALL honor `trust_mode` (`copy-only`,
`copy-referrers`, `resign`) and SHALL fail for unsupported values.

## Success metrics

- **CI green:** grcli validate and dry-run publish succeed against
  `testdata/minimal-catalog.yaml`. OCI layout structure verified.
- **Backward compatible:** All existing inputs (`registry`, `tag`, `username`, `password`,
  `sign_source`, `verify_source`) work unchanged. Only `license` is new.
- **Consumers** can pin `grcli_version` and `oras_version` and rely on stable
  `with:` / `outputs.digest` semantics documented in [README.md](../../README.md).
