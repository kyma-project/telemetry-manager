#!/usr/bin/env bash

set -o errexit  # exit immediately when a command fails.
set -E          # needs to be set if we want the ERR trap
set -o pipefail # prevents errors in a pipeline from being masked
set -o nounset  # treat unset variables as an error

# Install yq with checksum verification
# This script installs a pinned version of yq from mikefarah/yq with SHA-256 verification
#
# Usage: install-yq.sh [version] [install-path]
#
# Arguments:
#   version       - yq version to install (default: 4.44.1)
#   install-path  - Installation path (default: /usr/local/bin/yq)
#
# Environment variables:
#   YQ_VERSION      - Alternative way to specify version
#   YQ_INSTALL_PATH - Alternative way to specify install path

YQ_VERSION="${1:-${YQ_VERSION:-4.44.1}}"
YQ_INSTALL_PATH="${2:-${YQ_INSTALL_PATH:-/usr/local/bin/yq}}"

# Detect platform
OS="$(uname -s)"
ARCH="$(uname -m)"

case "${OS}" in
  Linux*)
    YQ_BINARY="yq_linux_amd64"
    ;;
  Darwin*)
    if [ "${ARCH}" = "arm64" ]; then
      YQ_BINARY="yq_darwin_arm64"
    else
      YQ_BINARY="yq_darwin_amd64"
    fi
    ;;
  *)
    echo "::error::Unsupported operating system: ${OS}"
    exit 1
    ;;
esac

echo "Detected platform: ${OS} ${ARCH} -> ${YQ_BINARY}"

YQ_URL="https://github.com/mikefarah/yq/releases/download/v${YQ_VERSION}/${YQ_BINARY}"
CHECKSUMS_URL="https://github.com/mikefarah/yq/releases/download/v${YQ_VERSION}/checksums"
HASHES_ORDER_URL="https://github.com/mikefarah/yq/releases/download/v${YQ_VERSION}/checksums_hashes_order"

echo "Installing yq v${YQ_VERSION} to ${YQ_INSTALL_PATH}..."

# Create temporary directory for downloads
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "${TEMP_DIR}"' EXIT

YQ_TEMP_PATH="${TEMP_DIR}/yq"
CHECKSUMS_FILE="${TEMP_DIR}/checksums"
HASHES_ORDER_FILE="${TEMP_DIR}/checksums_hashes_order"

# Download files
echo "Downloading yq binary..."
wget -q "${YQ_URL}" -O "${YQ_TEMP_PATH}"

echo "Downloading checksums..."
wget -q "${CHECKSUMS_URL}" -O "${CHECKSUMS_FILE}"

echo "Downloading checksum order metadata..."
wget -q "${HASHES_ORDER_URL}" -O "${HASHES_ORDER_FILE}"

# Compute SHA-256 column dynamically
echo "Computing SHA-256 checksum column..."
SHA256_LINE_NUM=$(grep -n "^SHA-256$" "${HASHES_ORDER_FILE}" | cut -d: -f1)

if [ -z "${SHA256_LINE_NUM}" ]; then
  echo "::error::Failed to find SHA-256 in checksums_hashes_order"
  exit 1
fi

SHA256_COLUMN=$((SHA256_LINE_NUM + 1))

# Extract expected checksum
echo "Extracting expected SHA-256 checksum..."
EXPECTED_SHA256="$(grep "^${YQ_BINARY}[[:space:]]" "${CHECKSUMS_FILE}" | sed 's/  /\t/g' | cut -f${SHA256_COLUMN})"

if [ -z "${EXPECTED_SHA256}" ]; then
  echo "::error::Failed to extract expected SHA-256 checksum for ${YQ_BINARY}"
  exit 1
fi

# Calculate actual checksum
echo "Calculating actual SHA-256 checksum..."
ACTUAL_SHA256="$(sha256sum "${YQ_TEMP_PATH}" | cut -d' ' -f1)"

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

# Check if we need sudo for installation
INSTALL_DIR=$(dirname "${YQ_INSTALL_PATH}")
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
