#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Shared helpers for gemara-publish-action scripts.
# Sourced by publish.sh and finalize.sh.

# Normalize an OCI digest to lowercase sha256:<64hex>.
# Usage: DIGEST=$(normalize_oci_digest "$raw_digest")
normalize_oci_digest() {
  local d="${1//$'\r'/}"
  d="${d//$'\n'/}"
  d="${d,,}"
  if [[ "$d" =~ ^sha256:[a-f0-9]{64}$ ]]; then
    echo "$d"
    return 0
  fi
  if [[ "$d" =~ ^[a-f0-9]{64}$ ]]; then
    echo "sha256:$d"
    return 0
  fi
  echo "::error::Expected sha256 manifest digest, got: $1" >&2
  return 1
}

# Exit 0 when the Gemara YAML at $1 carries a non-empty metadata.version.
# Usage: yaml_has_metadata_version <file>
#   Degrades to "no version" (exit 1) on any parse failure, which makes the
#   caller fall back to the tag — the pre-existing behavior.
yaml_has_metadata_version() {
  python3 - "$1" << 'PYEOF'
import sys, yaml
try:
    with open(sys.argv[1]) as f:
        doc = yaml.safe_load(f)
    version = (doc or {}).get("metadata", {}).get("version")
    sys.exit(0 if version else 1)
except Exception:
    sys.exit(1)
PYEOF
}

# Validate that a resolved path stays inside GITHUB_WORKSPACE.
# Usage: assert_within_workspace "$resolved_path" ["label"]
#   Expects: GITHUB_WORKSPACE to be set.
assert_within_workspace() {
  local path="$1" label="${2:-path}"
  local workspace_dir
  workspace_dir="$(cd -- "${GITHUB_WORKSPACE}" && pwd -P)" || exit 1
  if [[ "${path}" != "${workspace_dir}" && "${path}" != "${workspace_dir%/}/"* ]]; then
    echo "::error::${label} resolves outside the workspace: ${path}"
    exit 1
  fi
}

# Validate that the working directory resolves inside GITHUB_WORKSPACE.
# Exits with error if the resolved path escapes the workspace.
# Usage: validate_working_directory
#   Expects: INPUT_WORKING_DIRECTORY and GITHUB_WORKSPACE to be set.
#   Side effect: changes directory to the validated path.
validate_working_directory() {
  local workspace_dir
  workspace_dir="$(cd -- "${GITHUB_WORKSPACE}" && pwd -P)" || exit 1
  cd -- "${workspace_dir%/}/${INPUT_WORKING_DIRECTORY}" || exit 1
  local resolved_dir
  resolved_dir="$(pwd -P)"
  if [[ "${resolved_dir}" != "${workspace_dir}" && "${resolved_dir}" != "${workspace_dir%/}/"* ]]; then
    echo "::error::working_directory resolves outside the workspace"
    exit 1
  fi
}
