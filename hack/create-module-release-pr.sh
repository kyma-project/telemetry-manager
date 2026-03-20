#!/usr/bin/env bash

set -o errexit  # exit immediately when a command fails.
set -E          # needs to be set if we want the ERR trap
set -o pipefail # prevents errors in a pipeline from being masked
set -o nounset  # treat unset variables as an error

# Create or reuse PR for module release
# This script handles PR creation for telemetry module releases
#
# Usage: create-module-release-pr.sh <version> <channel> <output-file>
#
# Arguments:
#   version      - Version being released (e.g., 1.2.3)
#   channel      - Release channel (regular, fast, experimental)
#   output-file  - File to write PR URL and number (for GitHub Actions output)
#
# Environment variables required:
#   GH_TOKEN     - GitHub token for gh CLI authentication
#   GH_HOST      - GitHub host (e.g., github.tools.sap)
#
# Output format (written to output-file):
#   pr-url=<URL>
#   pr-number=<NUMBER>

VERSION="${1:-}"
CHANNEL="${2:-}"
OUTPUT_FILE="${3:-}"

if [ -z "${VERSION}" ] || [ -z "${CHANNEL}" ] || [ -z "${OUTPUT_FILE}" ]; then
  echo "Usage: $0 <version> <channel> <output-file>"
  echo ""
  echo "Arguments:"
  echo "  version      - Version being released (e.g., 1.2.3)"
  echo "  channel      - Release channel (regular, fast, experimental)"
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

BRANCH_NAME="bump-telemetry-${VERSION}-${CHANNEL}"

# Check if PR already exists for this branch
echo "Checking if PR already exists for branch: ${BRANCH_NAME}..."

# Use gh --jq to extract values directly, avoiding double jq parsing
# Redirect stderr to capture any real errors (auth, network, API)
GH_PR_LIST_ERR_FILE="$(mktemp)"
set +e  # Temporarily disable errexit to capture the exit code
PR_NUMBER=$(gh pr list --head "${BRANCH_NAME}" --json number --jq '.[0].number // empty' 2>"${GH_PR_LIST_ERR_FILE}")
GH_EXIT_CODE=$?
set -e  # Re-enable errexit

# Any non-zero exit code from gh is a fatal error
if [ ${GH_EXIT_CODE} -ne 0 ]; then
  GH_ERROR_OUTPUT="$(cat "${GH_PR_LIST_ERR_FILE}")"
  rm -f "${GH_PR_LIST_ERR_FILE}"
  echo "::error::Failed to check for existing PRs (exit code ${GH_EXIT_CODE}): ${GH_ERROR_OUTPUT}"
  exit 1
fi
rm -f "${GH_PR_LIST_ERR_FILE}"

# If PR_NUMBER is non-empty, a PR exists - get the URL
if [ -n "${PR_NUMBER}" ]; then
  PR_URL=$(gh pr list --head "${BRANCH_NAME}" --json url --jq '.[0].url')
  echo "::notice::PR already exists for this branch"
  echo "✓ Using existing PR #${PR_NUMBER}: ${PR_URL}"
  echo "pr-url=${PR_URL}" >> "${OUTPUT_FILE}"
  echo "pr-number=${PR_NUMBER}" >> "${OUTPUT_FILE}"
  exit 0
fi

echo "No existing PR found. Creating new PR..."

# Determine version tag and folder based on channel
if [ "${CHANNEL}" = "experimental" ]; then
  VERSION_TAG="${VERSION}-experimental"
  FOLDER_PATH="modules/telemetry/${VERSION}-experimental"
else
  VERSION_TAG="${VERSION}"
  FOLDER_PATH="modules/telemetry/${VERSION}"
fi

PR_TITLE="bump telemetry version to ${VERSION} on ${CHANNEL}"
PR_BODY=$(cat <<EOF
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

PR_URL=$(gh pr create \
  --base main \
  --head "${BRANCH_NAME}" \
  --title "${PR_TITLE}" \
  --body "${PR_BODY}")

PR_NUMBER=$(echo "${PR_URL}" | grep -o '[0-9]*$')
echo "pr-url=${PR_URL}" >> "${OUTPUT_FILE}"
echo "pr-number=${PR_NUMBER}" >> "${OUTPUT_FILE}"
echo "✓ Created PR #${PR_NUMBER}: ${PR_URL}"
