#!/usr/bin/env bash

set -o errexit  # exit immediately when a command fails.
set -E          # needs to be set if we want the ERR trap
set -o pipefail # prevents errors in a pipeline from being masked
set -o nounset  # treat unset variables as an error

# Script version for tracking and debugging
SCRIPT_VERSION="3.0.0"
SCRIPT_NAME="$(basename "$0")"

# Debug mode - set DEBUG=true to enable verbose logging
DEBUG="${DEBUG:-false}"

# Debug logging function
debug_log() {
  if [ "${DEBUG}" = "true" ]; then
    echo "[DEBUG] $*" >&2
  fi
}

# Log script version if debug mode is enabled
if [ "${DEBUG}" = "true" ]; then
  echo "[DEBUG] ${SCRIPT_NAME} version ${SCRIPT_VERSION}" >&2
fi

# Telemetry Module Release Script
# This script handles the complete release workflow for telemetry modules
#
# Usage: module-release.sh <command> [args...]
#
# Commands:
#   install-yq [version] [install-path]     Install yq with checksum verification
#   check-duplicate <version> <channel>     Check if version is already released
#   setup-folder <version> <channel>        Setup version folder (create or reuse)
#   update-config <version> <channel> [folder]  Update module-config.yaml
#   update-releases <version> <channel>     Update module-releases.yaml
#   create-pr <version> <channel> <output-file>  Create or reuse PR for release
#
# Channels: regular, fast, experimental, dev

COMMAND="${1:-}"

if [ -z "${COMMAND}" ]; then
  echo "Usage: module-release.sh <command> [args...]"
  echo ""
  echo "Commands:"
  echo "  install-yq [version] [install-path]     Install yq"
  echo "  check-duplicate <version> <channel>     Check if version is released"
  echo "  setup-folder <version> <channel>        Setup version folder"
  echo "  update-config <version> <channel> [folder]  Update module-config.yaml"
  echo "  update-releases <version> <channel>     Update module-releases.yaml"
  echo "  create-pr <version> <channel> <output-file>  Create PR"
  echo ""
  echo "Channels: regular, fast, experimental"
  exit 1
fi

shift  # Remove command from arguments

#==============================================================================
# COMMAND: install-yq
#==============================================================================
install_yq() {
  # Install yq with checksum verification
  # Usage: install-yq [version] [install-path]

  local YQ_VERSION="${1:-${YQ_VERSION:-4.44.1}}"
  local YQ_INSTALL_PATH="${2:-${YQ_INSTALL_PATH:-/usr/local/bin/yq}}"

  debug_log "install_yq: version=${YQ_VERSION}, path=${YQ_INSTALL_PATH}"

  # Detect platform
  local OS="$(uname -s)"
  local ARCH="$(uname -m)"
  local YQ_BINARY
  local DOWNLOAD_CMD
  local DOWNLOAD_OUTPUT_FLAG
  local SHA256_CMD

  case "${OS}" in
    Linux*)
      case "${ARCH}" in
        x86_64)
          YQ_BINARY="yq_linux_amd64"
          ;;
        aarch64|arm64)
          YQ_BINARY="yq_linux_arm64"
          ;;
        i386|i686)
          YQ_BINARY="yq_linux_386"
          ;;
        *)
          echo "::error::Unsupported Linux architecture: ${ARCH}"
          echo "Supported architectures: x86_64, aarch64/arm64, i386/i686"
          exit 1
          ;;
      esac
      DOWNLOAD_CMD="wget -q"
      DOWNLOAD_OUTPUT_FLAG="-O"
      SHA256_CMD="sha256sum"
      ;;
    Darwin*)
      case "${ARCH}" in
        arm64)
          YQ_BINARY="yq_darwin_arm64"
          ;;
        x86_64)
          YQ_BINARY="yq_darwin_amd64"
          ;;
        *)
          echo "::error::Unsupported macOS architecture: ${ARCH}"
          echo "Supported architectures: arm64, x86_64"
          exit 1
          ;;
      esac
      DOWNLOAD_CMD="curl -sfL"
      DOWNLOAD_OUTPUT_FLAG="-o"
      SHA256_CMD="shasum -a 256"
      ;;
    *)
      echo "::error::Unsupported operating system: ${OS}"
      echo "Supported operating systems: Linux, Darwin (macOS)"
      exit 1
      ;;
  esac

  echo "Detected platform: ${OS} ${ARCH} -> ${YQ_BINARY}"

  local YQ_URL="https://github.com/mikefarah/yq/releases/download/v${YQ_VERSION}/${YQ_BINARY}"
  local CHECKSUMS_URL="https://github.com/mikefarah/yq/releases/download/v${YQ_VERSION}/checksums"
  local HASHES_ORDER_URL="https://github.com/mikefarah/yq/releases/download/v${YQ_VERSION}/checksums_hashes_order"

  echo "Installing yq v${YQ_VERSION} to ${YQ_INSTALL_PATH}..."

  # Create temporary directory
  local TEMP_DIR=$(mktemp -d)
  trap "rm -rf '${TEMP_DIR}'" EXIT

  local YQ_TEMP_PATH="${TEMP_DIR}/yq"
  local CHECKSUMS_FILE="${TEMP_DIR}/checksums"
  local HASHES_ORDER_FILE="${TEMP_DIR}/checksums_hashes_order"

  # Download files
  echo "Downloading yq binary..."
  ${DOWNLOAD_CMD} "${YQ_URL}" ${DOWNLOAD_OUTPUT_FLAG} "${YQ_TEMP_PATH}"

  echo "Downloading checksums..."
  ${DOWNLOAD_CMD} "${CHECKSUMS_URL}" ${DOWNLOAD_OUTPUT_FLAG} "${CHECKSUMS_FILE}"

  echo "Downloading checksum order metadata..."
  ${DOWNLOAD_CMD} "${HASHES_ORDER_URL}" ${DOWNLOAD_OUTPUT_FLAG} "${HASHES_ORDER_FILE}"

  # Compute SHA-256 column
  echo "Computing SHA-256 checksum column..."
  local SHA256_LINE_NUM=$(grep -n "^SHA-256$" "${HASHES_ORDER_FILE}" | cut -d: -f1)

  if [ -z "${SHA256_LINE_NUM}" ]; then
    echo "::error::Failed to find SHA-256 in checksums_hashes_order"
    exit 1
  fi

  local SHA256_COLUMN=$((SHA256_LINE_NUM + 1))

  # Extract expected checksum
  echo "Extracting expected SHA-256 checksum..."
  local EXPECTED_SHA256="$(grep "^${YQ_BINARY}[[:space:]]" "${CHECKSUMS_FILE}" | sed 's/  /\t/g' | cut -f${SHA256_COLUMN})"

  if [ -z "${EXPECTED_SHA256}" ]; then
    echo "::error::Failed to extract expected SHA-256 checksum for ${YQ_BINARY}"
    exit 1
  fi

  # Calculate actual checksum
  echo "Calculating actual SHA-256 checksum..."
  local ACTUAL_SHA256="$(${SHA256_CMD} "${YQ_TEMP_PATH}" | cut -d' ' -f1)"

  # Verify checksum
  echo "Verifying checksum..."
  if [ "${EXPECTED_SHA256}" != "${ACTUAL_SHA256}" ]; then
    echo "::error::yq checksum mismatch"
    echo "Expected: ${EXPECTED_SHA256}"
    echo "Got:      ${ACTUAL_SHA256}"
    exit 1
  fi

  echo "✓ Checksum verified"

  # Install yq
  echo "Installing yq to ${YQ_INSTALL_PATH}..."
  chmod +x "${YQ_TEMP_PATH}"

  # Check if we need sudo
  local INSTALL_DIR=$(dirname "${YQ_INSTALL_PATH}")
  if [ -w "${INSTALL_DIR}" ]; then
    mv "${YQ_TEMP_PATH}" "${YQ_INSTALL_PATH}"
  else
    sudo mv "${YQ_TEMP_PATH}" "${YQ_INSTALL_PATH}"
  fi

  # Verify installation
  if ! command -v yq &> /dev/null; then
    echo "::warning::yq not in PATH. Installed at ${YQ_INSTALL_PATH}"
  else
    echo "✓ yq ${YQ_VERSION} installed and verified successfully"
    yq --version
  fi
}

#==============================================================================
# MODULE RELEASE COMMANDS
#==============================================================================

MODULE_DIR="modules/telemetry"
MODULE_RELEASES="${MODULE_DIR}/module-releases.yaml"

# Validate that the provided channel is supported
validate_channel() {
  local channel="$1"
  case "${channel}" in
    regular|fast|experimental|dev)
      # valid channel
      ;;
    *)
      echo "::error::Invalid channel '${channel}'. Allowed channels are: regular, fast, experimental, dev." >&2
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

# Sanitize error output to remove potential tokens or sensitive data
sanitize_error() {
  local error_text="$1"
  # Remove potential tokens (common patterns)
  error_text=$(echo "${error_text}" | sed -E 's/(ghp|gho|ghr|ghs)_[A-Za-z0-9_]{36,}/[REDACTED_TOKEN]/g')
  error_text=$(echo "${error_text}" | sed -E 's/Bearer [A-Za-z0-9._-]+/Bearer [REDACTED]/g')
  error_text=$(echo "${error_text}" | sed -E 's/Authorization: [^ ]+/Authorization: [REDACTED]/g')
  echo "${error_text}"
}

#==============================================================================
# COMMAND: check-duplicate
#==============================================================================
check_duplicate() {
  local version="$1"
  local channel="$2"

  debug_log "check_duplicate called with version=${version}, channel=${channel}"

  local VERSION_TAG=$(get_version_tag "${version}" "${channel}")
  debug_log "VERSION_TAG=${VERSION_TAG}"

  echo "Checking if version ${VERSION_TAG} is already released for ${channel} channel..."

  # Check current version in module-releases.yaml
  local CURRENT_VERSION=$(yq -r ".channels[] | select(.channel == \"${channel}\") | .version" "${MODULE_RELEASES}")
  debug_log "CURRENT_VERSION from module-releases.yaml: ${CURRENT_VERSION}"

  # Extract base version
  local CURRENT_BASE_VERSION="${CURRENT_VERSION%-experimental}"
  local NEW_BASE_VERSION="${version}"

  # Get script directory for check-version.py location
  local SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  debug_log "SCRIPT_DIR=${SCRIPT_DIR}"

  # Use external Python script for semantic version comparison
  python3 "${SCRIPT_DIR}/check-version.py" \
    "${CURRENT_BASE_VERSION}" \
    "${NEW_BASE_VERSION}" \
    "${channel}" \
    "${CURRENT_VERSION}" \
    "${VERSION_TAG}"

  echo "✓ Version ${VERSION_TAG} is valid for ${channel} channel"
  echo "  Current version: ${CURRENT_VERSION} (base: ${CURRENT_BASE_VERSION})"
  echo "  New version: ${VERSION_TAG} (base: ${NEW_BASE_VERSION})"
  echo "  ✓ Version is newer than current"
}

#==============================================================================
# COMMAND: setup-folder
#==============================================================================

# Setup version folder - common logic
setup_folder_common() {
  local version="$1"
  local channel="$2"
  local folder_suffix="$3"
  local grep_pattern="$4"
  local channel_desc="$5"

  local TARGET_FOLDER="${MODULE_DIR}/${version}${folder_suffix}"

  # Check if folder already exists
  if [ -d "${TARGET_FOLDER}" ]; then
    echo "✓ Folder ${TARGET_FOLDER} already exists"
  else
    echo "Folder ${TARGET_FOLDER} does not exist. Creating from previous version..."

    # Find the most recent version folder
    local PREVIOUS_VERSION=$(ls -1 "${MODULE_DIR}" | grep -E "${grep_pattern}" | sort -V | tail -1 || true)

    if [ -z "${PREVIOUS_VERSION}" ]; then
      echo "::error::No previous ${channel_desc} version found in ${MODULE_DIR}"
      echo "  Searched for pattern: ${grep_pattern}"
      echo "  Cannot create new version folder without a previous version to copy from"
      echo "  Please ensure at least one ${channel_desc} version folder exists"
      ls -1 "${MODULE_DIR}" | head -10
      exit 1
    fi

    local PREVIOUS_FOLDER="${MODULE_DIR}/${PREVIOUS_VERSION}"
    echo "Copying from previous ${channel_desc} version: ${PREVIOUS_VERSION}"

    # Validate previous folder has required files
    if [ ! -f "${PREVIOUS_FOLDER}/module-config.yaml" ]; then
      echo "::error::Previous ${channel_desc} version folder ${PREVIOUS_FOLDER} is invalid (missing module-config.yaml)"
      echo "  The folder exists but lacks required configuration file"
      echo "  This folder may have been created incorrectly or is corrupted"
      echo "  Please manually fix the folder or remove it before retrying"
      exit 1
    fi

    # Copy previous version to new version folder
    cp -r "${PREVIOUS_FOLDER}" "${TARGET_FOLDER}"
    echo "✓ Created ${TARGET_FOLDER} from ${PREVIOUS_FOLDER}"
  fi

  # Export TELEMETRY_FOLDER
  if [ -n "${GITHUB_ENV:-}" ]; then
    echo "TELEMETRY_FOLDER=${TARGET_FOLDER}" >> "$GITHUB_ENV"
  fi
  export TELEMETRY_FOLDER="${TARGET_FOLDER}"
}

setup_folder() {
  local version="$1"
  local channel="$2"

  # For dev, fast and regular channels
  if [ "${channel}" = "dev" ] || [ "${channel}" = "fast" ] || [ "${channel}" = "regular" ]; then
    setup_folder_common "${version}" "${channel}" "" '^[0-9]+\.[0-9]+\.[0-9]+$' "regular"
  elif [ "${channel}" = "experimental" ]; then
    setup_folder_common "${version}" "${channel}" "-experimental" '^[0-9]+\.[0-9]+\.[0-9]+-experimental$' "experimental"
  else
    echo "::error::Unknown channel: ${channel}"
    exit 1
  fi

  echo "✓ Setup complete. Folder: ${TELEMETRY_FOLDER}"
}

#==============================================================================
# COMMAND: update-config
#==============================================================================
update_config() {
  local version="$1"
  local channel="$2"
  local telemetry_folder="${3:-}"

  # If TELEMETRY_FOLDER is not provided, read from environment or compute it
  if [ -z "${telemetry_folder}" ]; then
    if [ -n "${TELEMETRY_FOLDER:-}" ]; then
      telemetry_folder="${TELEMETRY_FOLDER}"
    else
      if [ "${channel}" = "experimental" ]; then
        telemetry_folder="${MODULE_DIR}/${version}-experimental"
      else
        telemetry_folder="${MODULE_DIR}/${version}"
      fi
    fi
  fi

  local MODULE_CONFIG="${telemetry_folder}/module-config.yaml"

  if [ ! -f "${MODULE_CONFIG}" ]; then
    echo "::error::module-config.yaml not found at ${MODULE_CONFIG}"
    exit 1
  fi

  echo "Updating ${MODULE_CONFIG}..."

  local REPOSITORY_TAG="${version}"
  local VERSION_TAG=$(get_version_tag "${version}" "${channel}")

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
    yq -i ".manifest = \"telemetry-manager.yaml\"" "${MODULE_CONFIG}"
    echo "✓ Updated .manifest to telemetry-manager.yaml"

    yq -i ".securityScanEnabled = true" "${MODULE_CONFIG}"
    echo "✓ Updated .securityScanEnabled to true"
  fi

  # Verify updates
  local CURRENT_VERSION=$(yq -r '.version' "${MODULE_CONFIG}")
  local CURRENT_REPOSITORY_TAG=$(yq -r '.repositoryTag' "${MODULE_CONFIG}")

  if [ "$CURRENT_VERSION" != "$VERSION_TAG" ]; then
    echo "::error::Failed to update version field in ${MODULE_CONFIG}"
    echo "  Expected: ${VERSION_TAG}"
    echo "  Got: ${CURRENT_VERSION}"
    echo "  File: ${MODULE_CONFIG}"
    echo "  This usually means the yq command failed silently or the version field doesn't exist"
    echo "  Check that the YAML structure matches the expected format"
    exit 1
  fi

  # Validate version format is valid semver
  if ! echo "${CURRENT_VERSION}" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+)?$'; then
    echo "::error::Updated version has invalid semver format: ${CURRENT_VERSION}"
    echo "  File: ${MODULE_CONFIG}"
    echo "  Expected format: X.Y.Z or X.Y.Z-suffix (e.g., 1.2.3 or 1.2.3-experimental)"
    echo "  This indicates corrupted data or an incorrect yq update"
    exit 1
  fi

  if [ "$CURRENT_REPOSITORY_TAG" != "$REPOSITORY_TAG" ]; then
    echo "::error::Failed to update repositoryTag field in ${MODULE_CONFIG}"
    echo "  Expected: ${REPOSITORY_TAG}"
    echo "  Got: ${CURRENT_REPOSITORY_TAG}"
    echo "  File: ${MODULE_CONFIG}"
    echo "  This usually means the yq command failed silently or the repositoryTag field doesn't exist"
    echo "  Check that the YAML structure matches the expected format"
    exit 1
  fi

  # Validate repositoryTag format is valid semver (without suffix)
  if ! echo "${CURRENT_REPOSITORY_TAG}" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$'; then
    echo "::error::Updated repositoryTag has invalid semver format: ${CURRENT_REPOSITORY_TAG}"
    echo "  File: ${MODULE_CONFIG}"
    echo "  Expected format: X.Y.Z (e.g., 1.2.3)"
    echo "  This indicates corrupted data or an incorrect yq update"
    exit 1
  fi

  echo "✓ module-config.yaml updated and verified successfully"
  echo "  - Folder: ${telemetry_folder}"
  echo "  - Version: ${CURRENT_VERSION}"
  echo "  - Repository Tag: ${CURRENT_REPOSITORY_TAG}"
}

#==============================================================================
# COMMAND: update-releases
#==============================================================================
update_releases() {
  local version="$1"
  local channel="$2"

  if [ ! -f "${MODULE_RELEASES}" ]; then
    echo "::error::module-releases.yaml not found at ${MODULE_RELEASES}"
    exit 1
  fi

  echo "Updating ${MODULE_RELEASES}..."

  local VERSION_TAG=$(get_version_tag "${version}" "${channel}")

  echo "Updating channel '${channel}' to version: ${VERSION_TAG}"

  # Update the version for the specified channel
  yq -i "(.channels[] | select(.channel == \"${channel}\") | .version) = \"${VERSION_TAG}\"" "${MODULE_RELEASES}"

  # Verify update
  local UPDATED_VERSION=$(yq -r ".channels[] | select(.channel == \"${channel}\") | .version" "${MODULE_RELEASES}")

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

#==============================================================================
# COMMAND: create-pr
#==============================================================================
create_pr() {
  local VERSION="${1:-}"
  local CHANNEL="${2:-}"
  local OUTPUT_FILE="${3:-}"

  debug_log "Script inputs: VERSION=${VERSION}, CHANNEL=${CHANNEL}, OUTPUT_FILE=${OUTPUT_FILE}"

  if [ -z "${VERSION}" ] || [ -z "${CHANNEL}" ] || [ -z "${OUTPUT_FILE}" ]; then
    echo "Usage: module-release.sh create-pr <version> <channel> <output-file>"
    echo ""
    echo "Arguments:"
    echo "  version      - Version being released (e.g., 1.2.3)"
    echo "  channel      - Release channel (regular, fast, experimental, dev)"
    echo "  output-file  - File to write PR URL and number"
    exit 1
  fi

  # Validate environment variables
  if [ -z "${GH_TOKEN:-}" ]; then
    echo "::error::GH_TOKEN environment variable is required"
    exit 1
  fi

  if [ -z "${GH_HOST:-}" ]; then
    echo "::error::GH_HOST environment variable is required"
    exit 1
  fi

  debug_log "Environment: GH_HOST=${GH_HOST}"

  local BRANCH_NAME="bump-telemetry-${VERSION}-${CHANNEL}"
  debug_log "BRANCH_NAME=${BRANCH_NAME}"

  # Check if PR already exists
  echo "Checking if PR already exists for branch: ${BRANCH_NAME}..."

  # Use gh --jq to extract values directly
  local GH_PR_LIST_ERR_FILE="$(mktemp)"
  set +e
  local PR_NUMBER=$(gh pr list --head "${BRANCH_NAME}" --json number --jq '.[0].number // empty' 2>"${GH_PR_LIST_ERR_FILE}")
  local GH_EXIT_CODE=$?
  set -e

  # Any non-zero exit code from gh is a fatal error
  if [ ${GH_EXIT_CODE} -ne 0 ]; then
    local GH_ERROR_OUTPUT="$(cat "${GH_PR_LIST_ERR_FILE}")"
    rm -f "${GH_PR_LIST_ERR_FILE}"
    # Sanitize error output before logging
    local SANITIZED_ERROR=$(sanitize_error "${GH_ERROR_OUTPUT}")
    echo "::error::Failed to check for existing PRs (exit code ${GH_EXIT_CODE}): ${SANITIZED_ERROR}"
    exit 1
  fi
  rm -f "${GH_PR_LIST_ERR_FILE}"

  # If PR_NUMBER is non-empty, a PR exists
  if [ -n "${PR_NUMBER}" ]; then
    local PR_URL=$(gh pr list --head "${BRANCH_NAME}" --json url --jq '.[0].url')
    echo "::notice::PR already exists for this branch"
    echo "✓ Using existing PR #${PR_NUMBER}: ${PR_URL}"
    echo "pr-url=${PR_URL}" >> "${OUTPUT_FILE}"
    echo "pr-number=${PR_NUMBER}" >> "${OUTPUT_FILE}"
    exit 0
  fi

  echo "No existing PR found. Creating new PR..."

  # Determine version tag and folder based on channel
  local VERSION_TAG
  local FOLDER_PATH
  if [ "${CHANNEL}" = "experimental" ]; then
    VERSION_TAG="${VERSION}-experimental"
    FOLDER_PATH="modules/telemetry/${VERSION}-experimental"
  else
    VERSION_TAG="${VERSION}"
    FOLDER_PATH="modules/telemetry/${VERSION}"
  fi

  local PR_TITLE="bump telemetry version to ${VERSION} on ${CHANNEL}"
  local PR_BODY=$(cat <<EOF
Bump telemetry version to ${VERSION} on ${CHANNEL} channel.

**Changes:**
- Created/Updated folder: ${FOLDER_PATH}
- Updated module-config.yaml:
  - version: ${VERSION_TAG}
  - repositoryTag: ${VERSION}
- Updated module-releases.yaml:
  - channels.${CHANNEL}.version: ${VERSION_TAG}
EOF
)

  local PR_URL=$(gh pr create \
    --base main \
    --head "${BRANCH_NAME}" \
    --title "${PR_TITLE}" \
    --body "${PR_BODY}")

  # Extract and validate PR number
  PR_NUMBER=$(echo "${PR_URL}" | grep -o '[0-9]\+$')
  if [[ ! "${PR_NUMBER}" =~ ^[0-9]+$ ]] || [ -z "${PR_NUMBER}" ]; then
    echo "::error::Failed to extract PR number from URL: ${PR_URL}"
    echo "Expected URL format: https://host/org/repo/pull/NUMBER"
    exit 1
  fi

  echo "pr-url=${PR_URL}" >> "${OUTPUT_FILE}"
  echo "pr-number=${PR_NUMBER}" >> "${OUTPUT_FILE}"
  echo "✓ Created PR #${PR_NUMBER}: ${PR_URL}"
}

#==============================================================================
# MAIN COMMAND DISPATCHER
#==============================================================================
case "${COMMAND}" in
  install-yq)
    install_yq "$@"
    ;;
  check-duplicate)
    if [ $# -ne 2 ]; then
      echo "Usage: module-release.sh check-duplicate <version> <channel>"
      exit 1
    fi
    check_duplicate "$1" "$2"
    ;;
  setup-folder)
    if [ $# -ne 2 ]; then
      echo "Usage: module-release.sh setup-folder <version> <channel>"
      exit 1
    fi
    setup_folder "$1" "$2"
    ;;
  update-config)
    if [ $# -lt 2 ] || [ $# -gt 3 ]; then
      echo "Usage: module-release.sh update-config <version> <channel> [folder]"
      exit 1
    fi
    update_config "$@"
    ;;
  update-releases)
    if [ $# -ne 2 ]; then
      echo "Usage: module-release.sh update-releases <version> <channel>"
      exit 1
    fi
    update_releases "$1" "$2"
    ;;
  create-pr)
    if [ $# -ne 3 ]; then
      echo "Usage: module-release.sh create-pr <version> <channel> <output-file>"
      exit 1
    fi
    create_pr "$@"
    ;;
  *)
    echo "Error: Unknown command '${COMMAND}'"
    echo ""
    echo "Usage: module-release.sh <command> [args...]"
    echo ""
    echo "Commands:"
    echo "  install-yq [version] [install-path]     Install yq"
    echo "  check-duplicate <version> <channel>     Check if version is released"
    echo "  setup-folder <version> <channel>        Setup version folder"
    echo "  update-config <version> <channel> [folder]  Update module-config.yaml"
    echo "  update-releases <version> <channel>     Update module-releases.yaml"
    echo "  create-pr <version> <channel> <output-file>  Create PR"
    echo ""
    echo "Channels: regular, fast, experimental"
    exit 1
    ;;
esac
