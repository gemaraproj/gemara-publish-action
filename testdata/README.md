# Test fixtures

## `minimal-catalog.yaml`

Minimal **Gemara ControlCatalog** used by CI to exercise `grcli validate` and
`grcli publish --dry-run`. Contains a single family and control — enough to
validate that grcli can parse the YAML, validate it against the Gemara CUE
spec, and run assemble/pack without errors.

The fixture includes `metadata.version: "0.0.1"` which grcli requires for
packing. Once grcli supports `--version` ([gemaraproj/grcli#3](https://github.com/gemaraproj/grcli/issues/3)),
test fixtures without this field will also be exercised.

Used in `.github/workflows/ci.yml` as the dry-run target.
