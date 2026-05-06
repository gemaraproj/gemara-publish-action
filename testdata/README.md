# Test fixtures

## `minimal-catalog.yaml`

Minimal **Gemara ControlCatalog** used by CI to exercise `tools/publish/` (go-gemara `Assemble` +
`Pack` dry-run). Contains a single family and control — enough to validate that the publisher
compiles, loads the YAML via `gemara.Load`, and runs assemble/pack without errors.

Used in `.github/workflows/ci.yml` as the dry-run target.
