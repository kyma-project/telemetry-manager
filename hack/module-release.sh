#!/usr/bin/env bash

set -o errexit  # exit immediately when a command fails.
set -E          # needs to be set if we want the ERR trap
set -o pipefail # prevents errors in a pipeline from being masked
set -o nounset  # treat unset variables as an error


# Telemetry Module Release Script
# This script handles the creation and update of module releases for different channels
#
# Usage: module-release.sh <command> <version> <channel>
#
# Commands:
#   check-duplicate    Check if version is already released
#   setup-folder       Setup version folder (create or reuse existing)
#   update-config      Update module-config.yaml
#   update-releases    Update module-releases.yaml
#
# Channels: regular, fast, experimental

COMMAND="${1:-}"
VERSION="${2:-}"
CHANNEL="${3:-}"

 if [ -z "${COMMAND}" ] || [ -z "${VERSION}" ] || [ -z "${CHANNEL}" ]; then
   echo "Usage: module-release.sh <command> <version> <channel>"
   echo "Commands: check-duplicate | setup-folder | update-config | update-releases"
   echo "Channels: regular | fast | experimental"
   exit 1
 fi

MODULE_DIR="modules/telemetry"
MODULE_RELEASES="${MODULE_DIR}/module-releases.yaml"

 # Validate that the provided channel is supported
 validate_channel() {
   local channel="$1"
   case "${channel}" in
     regular|fast|experimental)
       # valid channel
       ;;
     *)
       echo "::error::Invalid channel '${channel}'. Allowed channels are: regular, fast, experimental." >&2
       exit 1
       ;;
   esac
 }

# Determine version tag based on channel
get_version_tag() {
  local version="$1"
  local channel="$2"

  validate_channel "${channel}"

  if [ "${channel}" = "experimental" ]; then
    echo "${version}-experimental"
  else
    echo "${version}"
  fi
}

# Check for duplicate version and prevent downgrades
check_duplicate() {
  local version="$1"
  local channel="$2"

  VERSION_TAG=$(get_version_tag "${version}" "${channel}")

  echo "Checking if version ${VERSION_TAG} is already released for ${channel} channel..."

  # Check current version in module-releases.yaml
  CURRENT_VERSION=$(yq ".channels[] | select(.channel == \"${channel}\") | .version" "${MODULE_RELEASES}")

  # Extract base version (strip -experimental suffix for comparison)
  CURRENT_BASE_VERSION="${CURRENT_VERSION%-experimental}"
  NEW_BASE_VERSION="${version}"

  # Use Python semver library for semantic version comparison
  # This prevents both duplicates and downgrades
  python3 - <<EOF
import sys
try:
    import semver
except ImportError:
    print("::error::Python semver module not installed. Install with: pip install semver", file=sys.stderr)
    sys.exit(1)

current = "${CURRENT_BASE_VERSION}"
new = "${NEW_BASE_VERSION}"

try:
    current_ver = semver.VersionInfo.parse(current)
    new_ver = semver.VersionInfo.parse(new)
except Exception as e:
    print(f"::error::Invalid semantic version format: {e}", file=sys.stderr)
    sys.exit(1)

if new_ver < current_ver:
    print(f"::error::Version downgrade not allowed for ${channel} channel", file=sys.stderr)
    print(f"Current version: ${CURRENT_VERSION} (base: {current})", file=sys.stderr)
    print(f"Requested version: ${VERSION_TAG} (base: {new})", file=sys.stderr)
    print(f"Please use a version greater than {current}", file=sys.stderr)
    sys.exit(1)
elif new_ver == current_ver:
    print(f"::error::Version ${VERSION_TAG} is already released for ${channel} channel", file=sys.stderr)
    print(f"Current version in module-releases.yaml: ${CURRENT_VERSION}", file=sys.stderr)
    print(f"Please use a newer version number", file=sys.stderr)
    sys.exit(1)

# Version is valid (new_ver > current_ver)
sys.exit(0)
EOF

  echo "✓ Version ${VERSION_TAG} is valid for ${channel} channel"
  echo "  Current version: ${CURRENT_VERSION} (base: ${CURRENT_BASE_VERSION})"
  echo "  New version: ${VERSION_TAG} (base: ${NEW_BASE_VERSION})"
  echo "  ✓ Version is newer than current"
}

# Setup version folder
setup_folder() {
  local version="$1"
  local channel="$2"

  # For fast and regular channels, folder name should be the version
  if [ "${channel}" = "fast" ] || [ "${channel}" = "regular" ]; then
    TARGET_FOLDER="${MODULE_DIR}/${version}"

    # Check if folder already exists
    if [ -d "${TARGET_FOLDER}" ]; then
      echo "✓ Folder ${TARGET_FOLDER} already exists"
    else
      echo "Folder ${TARGET_FOLDER} does not exist. Creating from previous version..."

      # Find the most recent version folder (sorted by semantic version)
      # Use || true to prevent errexit when grep finds no matches
      PREVIOUS_VERSION=$(ls -1 "${MODULE_DIR}" | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -1 || true)

      if [ -z "${PREVIOUS_VERSION}" ]; then
        echo "::error::No previous version found in ${MODULE_DIR}"
        exit 1
      fi

      PREVIOUS_FOLDER="${MODULE_DIR}/${PREVIOUS_VERSION}"
      echo "Copying from previous version: ${PREVIOUS_VERSION}"

      # Copy previous version to new version folder
      cp -r "${PREVIOUS_FOLDER}" "${TARGET_FOLDER}"
      echo "✓ Created ${TARGET_FOLDER} from ${PREVIOUS_FOLDER}"
    fi

    # Export TELEMETRY_FOLDER to GITHUB_ENV if in GitHub Actions, otherwise just export as env var
    if [ -n "${GITHUB_ENV:-}" ]; then
      echo "TELEMETRY_FOLDER=${TARGET_FOLDER}" >> "$GITHUB_ENV"
    fi
    export TELEMETRY_FOLDER="${TARGET_FOLDER}"

  elif [ "${channel}" = "experimental" ]; then
    # For experimental channel, folder name should be x.y.z-experimental
    TARGET_FOLDER="${MODULE_DIR}/${version}-experimental"

    # Check if folder already exists
    if [ -d "${TARGET_FOLDER}" ]; then
      echo "✓ Folder ${TARGET_FOLDER} already exists"
    else
      echo "Folder ${TARGET_FOLDER} does not exist. Creating from previous version..."

      # Find the most recent experimental version folder (only -experimental versions)
      # Use || true to prevent errexit when grep finds no matches
      PREVIOUS_VERSION=$(ls -1 "${MODULE_DIR}" | grep -E '^[0-9]+\.[0-9]+\.[0-9]+-experimental$' | sort -V | tail -1 || true)

      if [ -z "${PREVIOUS_VERSION}" ]; then
        echo "::error::No previous experimental version found in ${MODULE_DIR}"
        exit 1
      fi

      PREVIOUS_FOLDER="${MODULE_DIR}/${PREVIOUS_VERSION}"
      echo "Copying from previous experimental version: ${PREVIOUS_VERSION}"

      # Copy previous experimental version to new experimental folder
      cp -r "${PREVIOUS_FOLDER}" "${TARGET_FOLDER}"
      echo "✓ Created ${TARGET_FOLDER} from ${PREVIOUS_FOLDER}"
    fi

    # Export TELEMETRY_FOLDER to GITHUB_ENV if in GitHub Actions, otherwise just export as env var
    if [ -n "${GITHUB_ENV:-}" ]; then
      echo "TELEMETRY_FOLDER=${TARGET_FOLDER}" >> "$GITHUB_ENV"
    fi
    export TELEMETRY_FOLDER="${TARGET_FOLDER}"

  else
    echo "::error::Unknown channel: ${channel}"
    exit 1
  fi

  echo "✓ Setup complete. Folder: ${TARGET_FOLDER}"
}

# Update module-config.yaml
update_config() {
  local version="$1"
  local channel="$2"
  local telemetry_folder="${3:-}"

  # If TELEMETRY_FOLDER is not provided, read from environment or compute it
  if [ -z "${telemetry_folder}" ]; then
    if [ -n "${TELEMETRY_FOLDER:-}" ]; then
      telemetry_folder="${TELEMETRY_FOLDER}"
    else
      # Compute folder path based on channel
      if [ "${channel}" = "experimental" ]; then
        telemetry_folder="${MODULE_DIR}/${version}-experimental"
      else
        telemetry_folder="${MODULE_DIR}/${version}"
      fi
    fi
  fi

  MODULE_CONFIG="${telemetry_folder}/module-config.yaml"

  if [ ! -f "${MODULE_CONFIG}" ]; then
    echo "::error::module-config.yaml not found at ${MODULE_CONFIG}"
    exit 1
  fi

  echo "Updating ${MODULE_CONFIG}..."

  REPOSITORY_TAG="${version}"
  VERSION_TAG=$(get_version_tag "${version}" "${channel}")

  echo "Setting version fields to: ${VERSION_TAG}"

  # Update version field
  yq -i ".version = \"${VERSION_TAG}\"" "${MODULE_CONFIG}"
  echo "✓ Updated .version to ${VERSION_TAG}"

  # Update repositoryTag field
  yq -i ".repositoryTag = \"${REPOSITORY_TAG}\"" "${MODULE_CONFIG}"
  echo "✓ Updated .repositoryTag to ${REPOSITORY_TAG}"

  # Update manifest and securityScanEnabled for experimental channel
  if [ "${channel}" = "experimental" ]; then
    yq -i ".manifest = \"telemetry-manager-experimental.yaml\"" "${MODULE_CONFIG}"
    echo "✓ Updated .manifest to telemetry-manager-experimental.yaml"

    yq -i ".securityScanEnabled = false" "${MODULE_CONFIG}"
    echo "✓ Updated .securityScanEnabled to false"

    # Remove security field if it exists
    if yq -e '.security' "${MODULE_CONFIG}" > /dev/null 2>&1; then
      yq -i "del(.security)" "${MODULE_CONFIG}"
      echo "✓ Removed .security field"
    fi
  else
    # For regular/fast channels
    yq -i ".manifest = \"telemetry-manager.yaml\"" "${MODULE_CONFIG}"
    echo "✓ Updated .manifest to telemetry-manager.yaml"

    yq -i ".securityScanEnabled = true" "${MODULE_CONFIG}"
    echo "✓ Updated .securityScanEnabled to true"
  fi

  # Verify updates
  CURRENT_VERSION=$(yq '.version' "${MODULE_CONFIG}")
  CURRENT_REPOSITORY_TAG=$(yq '.repositoryTag' "${MODULE_CONFIG}")

  if [ "$CURRENT_VERSION" != "$VERSION_TAG" ]; then
    echo "::error::Failed to update version field. Expected: ${VERSION_TAG}, Got: ${CURRENT_VERSION}"
    exit 1
  fi

  if [ "$CURRENT_REPOSITORY_TAG" != "$REPOSITORY_TAG" ]; then
    echo "::error::Failed to update repositoryTag field. Expected: ${REPOSITORY_TAG}, Got: ${CURRENT_REPOSITORY_TAG}"
    exit 1
  fi

  echo "✓ module-config.yaml updated and verified successfully"
  echo "  - Folder: ${telemetry_folder}"
  echo "  - Version: ${CURRENT_VERSION}"
  echo "  - Repository Tag: ${CURRENT_REPOSITORY_TAG}"
}

# Update module-releases.yaml
update_releases() {
  local version="$1"
  local channel="$2"

  if [ ! -f "${MODULE_RELEASES}" ]; then
    echo "::error::module-releases.yaml not found at ${MODULE_RELEASES}"
    exit 1
  fi

  echo "Updating ${MODULE_RELEASES}..."

  VERSION_TAG=$(get_version_tag "${version}" "${channel}")

  echo "Updating channel '${channel}' to version: ${VERSION_TAG}"

  # Update the version for the specified channel
  yq -i "(.channels[] | select(.channel == \"${channel}\") | .version) = \"${VERSION_TAG}\"" "${MODULE_RELEASES}"

  # Verify update
  UPDATED_VERSION=$(yq ".channels[] | select(.channel == \"${channel}\") | .version" "${MODULE_RELEASES}")

  if [ "$UPDATED_VERSION" != "$VERSION_TAG" ]; then
    echo "::error::Failed to update module-releases.yaml. Expected: ${VERSION_TAG}, Got: ${UPDATED_VERSION}"
    exit 1
  fi

  echo "✓ module-releases.yaml updated successfully"
  echo "  - Channel: ${channel}"
  echo "  - Version: ${UPDATED_VERSION}"

  # Display the updated file
  echo ""
  echo "Updated module-releases.yaml:"
  cat "${MODULE_RELEASES}"
}

# Main command dispatcher
case "${COMMAND}" in
  check-duplicate)
    check_duplicate "${VERSION}" "${CHANNEL}"
    ;;
  setup-folder)
    setup_folder "${VERSION}" "${CHANNEL}"
    ;;
  update-config)
    update_config "${VERSION}" "${CHANNEL}" "${TELEMETRY_FOLDER:-}"
    ;;
  update-releases)
    update_releases "${VERSION}" "${CHANNEL}"
    ;;
  *)
    echo "Error: Unknown command '${COMMAND}'"
    echo ""
    echo "Usage: module-release.sh <command> <version> <channel>"
    echo ""
    echo "Commands:"
    echo "  check-duplicate    Check if version is already released"
    echo "  setup-folder       Setup version folder (create or reuse existing)"
    echo "  update-config      Update module-config.yaml"
    echo "  update-releases    Update module-releases.yaml"
    echo ""
    echo "Channels: regular, fast, experimental"
    exit 1
    ;;
esac