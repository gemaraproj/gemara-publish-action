# gemara-publish-action

GitHub Action for Gemara OCI publish and trust orchestration. Uses the
[go-gemara](https://github.com/gemaraproj/go-gemara) bundle SDK (`Assemble` + `Pack`)
and ORAS to push a root Gemara YAML (Policy, Catalog, or Guidance) as an OCI bundle.

| Phase | What it does |
|--------|----------------|
| **1 — Publish** | Validate (`gemara.Load` + `cue vet` against authoritative CUE schemas), assemble dependencies, pack into OCI bundle, push to **source** registry via go-gemara + ORAS. |
| **2 — Trust (source)** | Optional keyless **cosign** sign/verify on the **source** digest. |
| **3 — Promote (optional)** | Optional **ORAS** copy to a **destination** registry with `trust_mode`. |
| **4 — Trust (destination)** | Optional sign/verify on the **destination** digest when promotion and flags request it. |

**Outputs:** `digest` / `source_digest` and `source_ref` are always **normalized** to the form `sha256:<64 lowercase hex>` / `registry/repo@sha256:…` for stable downstream use.

**Who owns the contract:** [go-gemara](https://github.com/gemaraproj/go-gemara) owns bundle media types, manifest shape, and Pack/Assemble. This action does not.

### Boundaries (what is *not* in this action)

- **SLSA provenance**, **SBOM**, and **vulnerability** attestations are **out of scope** here—add them in **separate job steps** or a caller workflow *after* publish, if your policy needs them.
- **Branch / environment / approval** gates are **not** enforced inside the composite. Use **caller workflow** `if:` and GitHub **Environments** (see [Caller patterns](#caller-patterns) below).
- Composites only receive **string** inputs. Boolean-like flags are the literal strings **`"true"`** and **`"false"`** (see table below).

| Typical "boolean" input | Set to |
|------------------------|--------|
| `sign_source`, `verify_source`, `promote_to_destination`, `sign_destination`, `verify_destination` | `"true"` or `"false"` |

### Caller patterns

Reusable **workflows** can set `concurrency:`, `environment:`, and `if: github.ref_protected` on a **job**. A **composite** cannot—so apply these on the **job** (or parent workflow) that invokes this action:

- **Concurrency:** add a `concurrency:` group on the publish job (for example keyed by `repository` + `tag` or by digest) so overlapping releases do not clobber one another.
- **Protected branches / tags:** use `if: github.ref_protected` (or your org's rules) on the same job for production releases.
- **Environments (manual approval):** set `environment: production` (or similar) on the **job** so the step that uses this action runs under environment protection rules, matching how org-infra sign workflows use `sign_environment`.

## Promotion and trust

- Set `promote_to_destination: "true"` and **`destination_*`** inputs to copy the published tag to a
  second registry (for example GHCR -> Quay or GHCR -> another org registry).
- Standard path defaults (no extra inputs needed):
  - `trust_mode: resign`
  - `sign_destination: "true"`
  - `verify_destination: "true"`
- Optional compatibility trust modes remain available:
  - `copy-only`: copy payload tag only.
  - `copy-referrers`: recursive copy to include referrer graph when registry support is available.
- Source and destination verification default to issuer
  `https://token.actions.githubusercontent.com` (override with `cosign_certificate_oidc_issuer` when
  your signing environment differs).
- This repository's CI validates source-publish and source-signing behavior. Cross-registry
  promotion verification is authoritative in caller repositories where destination credentials
  and release controls live.

## Key inputs

| Input | Description |
|-------|-------------|
| `file` | Root Gemara artifact YAML (Policy, Catalog, or Guidance). **Required.** |
| `registry`, `repository`, `tag` | Source destination for publish. |
| `username`, `password` | Source registry auth. |
| `validate` | Run `gemara.Load` and `cue vet` schema validation before assemble (`"true"` / `"false"`). |
| `bundle_version` | Bundle format version (default `"1"`). |
| `working_directory` | Working directory relative to repo root for resolving `file`. |
| `sign_source`, `verify_source` | Source signature controls. |
| `promote_to_destination` | Enable promotion to `destination_*`. |
| `destination_registry`, `destination_repository`, `destination_tag`, `destination_username`, `destination_password` | Destination registry host, path without host, optional tag override, credentials. |
| `cosign_certificate_oidc_issuer` | Expected OIDC issuer for `cosign verify` (defaults to GitHub Actions). |
| `trust_mode` | `copy-only`, `copy-referrers`, or `resign`. |
| `verify_destination` | Destination signature verification control. |
| `allowed_identity_regex` | Optional cosign identity regex override. |

## Outputs

| Output | Description |
|--------|-------------|
| `digest` / `source_digest` | Source manifest digest, **always** `sha256:` + 64 hex (lowercase). |
| `source_ref` | `registry/repository@sha256:…` (digest normalized as above). |
| `destination_digest` | Destination digest after promotion. |
| `destination_ref` | Destination image reference with digest. |
| `verified_source` | `true` if source verify passed. |
| `verified_destination` | `true` if destination verify passed. |
| `trust_mode` | Effective trust mode used. |

## Minimal caller example

```yaml
permissions:
  contents: read
  packages: write
  id-token: write

jobs:
  publish:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - id: publish
        uses: gemaraproj/gemara-publish-action@<pinned-sha>
        with:
          registry: ghcr.io
          repository: ${{ github.repository }}
          tag: ${{ github.ref_name }}
          file: governance/policies/my-policy.yaml
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
          promote_to_destination: "true"
          destination_registry: quay.io
          destination_repository: myorg/my-policies
          destination_username: ${{ secrets.QUAY_ROBOT_USERNAME }}
          destination_password: ${{ secrets.QUAY_ROBOT_TOKEN }}
```

## Repository layout

- **`cmd/grc/`** — Gemara Registry CLI (`grc`), a small Go program used by the action. It wires `bundle.Assemble` / `bundle.Pack` and `oras.Copy`; SDK semantics stay in **go-gemara** `v0.4.0+`.
- **`testdata/`** — minimal Gemara catalog for CI validation and assembly tests.

## Pinning

Use a full commit SHA for production callers. Avoid floating refs.

## License

See [LICENSE](LICENSE).
