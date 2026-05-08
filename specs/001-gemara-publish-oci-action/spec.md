# Feature Specification: Gemara publish OCI composite GitHub Action

## Document overview

This specification describes the **composite** GitHub Action shipped from this repository
(`action.yml`): publish a root Gemara YAML as an OCI bundle using the go-gemara SDK, plus keyless
sign/verify and optional cross-registry promotion with explicit trust modes.

**Key metadata**

- **Action definition:** `action.yml` (composite)
- **Related design:** [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md), [docs/adr/](../../docs/adr/)

**Scope boundary:** This repository intentionally stays **transport-only**: it does not assemble Gemara layer manifests or run SBOM/SLSA steps; Pack and OCI contract details live in **go-gemara** (see README).

## Background and motivation

CI callers need a **small, auditable** Action that:

1. Accepts a root Gemara artifact YAML (Policy, Catalog, or Guidance) and publishes it as an OCI
   bundle using go-gemara's `Assemble` + `Pack` + `oras.Copy`.
2. Authenticates to the registry using **secrets the workflow supplies**, without echoing tokens.
3. Optionally signs and verifies the published digest with keyless cosign.
4. Optionally promotes the bundle to a second registry with configurable trust modes.
5. Emits stable outputs for source/destination refs, digests, and verification state for downstream
   release jobs.

## Core user scenarios

### Priority 1: Full publish orchestration for callers

A maintainer calls the action once with publish settings and trust settings; the action builds the
bundle from a root Gemara YAML via go-gemara (Assemble + Pack + oras.Copy), publishes to the source
registry, signs/verifies the source digest, optionally promotes to a destination registry, and
returns source/destination outputs.

**Test coverage:** `.github/workflows/ci.yml` builds `cmd/grc/`, runs a dry-run assemble/pack
against `testdata/minimal-catalog.yaml`, and validates the CLI compiled successfully.

### Priority 2: Destination trust behavior

Callers can choose trust mode:

- `copy-only`
- `copy-referrers`
- `resign` (default)

and verify destination trust with the same workflow identity constraints.

## Edge cases addressed

- **Missing or invalid root YAML:** Fail before publish if `gemara.Load` validation fails (when `validate: "true"`).
- **Missing password:** Fail with a clear error.
- **Digest resolution:** Use `oras resolve` on source/destination references and fail fast if unavailable.
- **Username default:** When using password auth, default username behavior matches `action.yml` / README (e.g. `GITHUB_ACTOR` when username omitted).

## Functional requirements summary

The Action must:

1. **Set up Go** and build the `grc` CLI from `cmd/grc/`.
2. **Install ORAS** using the `oras_version` input for digest resolution and promotion copy.
3. **Publish:** Run the publisher with the caller's `file`, `registry`, `repository`, `tag`, and credentials. The publisher invokes `bundle.Assemble` + `bundle.Pack` + `oras.Copy` from go-gemara.
4. **Optional sign/verify:** Keyless cosign sign and verify on the source digest.
5. **Optional promotion:** Copy source to destination registry with selected trust mode and destination sign/verify.
6. **Output contract:** Append source/destination refs, digests, and verification booleans to
   `GITHUB_OUTPUT`.

## Scope boundaries

**In scope:** ORAS install pin, registry auth, go-gemara SDK publish via `grc` CLI, sign/verify
orchestration, optional promotion, structured outputs.

**Out of scope:** Gemara YAML schema ownership, layer `mediaType` tables, Pack/Unpack
implementation details (these live in go-gemara).

## Formal requirements (SHALL / scenarios)

### Requirement: Pinned ORAS CLI install

The Action SHALL download and install the ORAS CLI for the runner OS and architecture using the `oras_version` input as the **sole** version selector for the official ORAS release artifact, and SHALL place the `oras` binary on `PATH` for subsequent commands in the same step.

#### Scenario: Default version is used when input omitted

- **WHEN** the workflow invokes the Action without setting `oras_version`
- **THEN** the Action SHALL install the default ORAS version documented in `action.yml` and successfully run `oras version`

#### Scenario: Caller pins a specific ORAS version

- **WHEN** the workflow sets `oras_version` to a supported release (e.g. `1.2.0`)
- **THEN** the Action SHALL install ORAS from the corresponding `oras-project/oras` GitHub release and use that binary for digest resolution and promotion copy

### Requirement: Registry authentication

The Action SHALL authenticate to the registry using credentials supplied by the caller, and SHALL NOT print the `password` input to logs.

#### Scenario: Registry with password

- **WHEN** `password` is non-empty
- **THEN** the publisher CLI SHALL authenticate using the provided credentials and SHALL succeed when credentials are valid

#### Scenario: Registry without password is rejected

- **WHEN** `password` is empty
- **THEN** the Action SHALL fail with an error indicating `password` is required

### Requirement: Publish via go-gemara SDK

The Action SHALL build the `grc` CLI (`cmd/grc/`), invoke it with the caller's `file`,
`registry`, `repository`, `tag`, and credentials, and the CLI SHALL use `bundle.Assemble` +
`bundle.Pack` + `oras.Copy` from go-gemara to publish the root Gemara YAML as an OCI bundle.

#### Scenario: Successful publish

- **WHEN** the root Gemara YAML is valid, registry credentials are correct, and `validate` is `"true"`
- **THEN** the publisher SHALL assemble dependencies, pack into an OCI bundle, push to the target registry, and emit a digest output

#### Scenario: Validation failure

- **WHEN** `validate` is `"true"` and `gemara.Load` fails on the root YAML
- **THEN** the Action SHALL fail before attempting registry push

### Requirement: Source and destination output contract

After a successful publish, the Action SHALL write source digest/reference outputs. If promotion is
enabled, it SHALL write destination digest/reference outputs and trust/verification statuses.

#### Scenario: Outputs available to downstream steps

- **WHEN** publish completes successfully
- **THEN** `digest`, `source_digest`, and `source_ref` SHALL be non-empty outputs

#### Scenario: Promotion outputs emitted

- **WHEN** `promote_to_destination` is enabled and succeeds
- **THEN** `destination_ref` and `destination_digest` SHALL be emitted and non-empty

### Requirement: Destination trust mode

When `promote_to_destination` is enabled, the Action SHALL honor `trust_mode` (`copy-only`,
`copy-referrers`, `resign`) and SHALL fail for unsupported values.

#### Scenario: Resign destination

- **WHEN** `trust_mode` is `resign`
- **THEN** the destination digest SHALL be signed and verifiable per configured identity policy

## Success metrics

- **CI green:** Publisher builds and dry-run assemble/pack succeeds against `testdata/minimal-catalog.yaml`.
- **Consumers** can pin `oras_version` and rely on stable `with:` / `outputs.digest` semantics documented in [README.md](../../README.md).
