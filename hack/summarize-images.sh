#!/usr/bin/env bash

# This script summarizes container images used in the environment
# and outputs them in markdown table format to GitHub step summary.

set -euo pipefail

# If GITHUB_STEP_SUMMARY is not set (local run), output to stdout
if [ -z "${GITHUB_STEP_SUMMARY:-}" ]; then
  GITHUB_STEP_SUMMARY="/dev/stdout"
fi

echo "## 📦 Container Images (europe-docker.pkg.dev)" >> "${GITHUB_STEP_SUMMARY}"
echo "" >> "${GITHUB_STEP_SUMMARY}"
echo "| Variable | Image | Tag | Alternative Tag |" >> "${GITHUB_STEP_SUMMARY}"
echo "|----------|-------|-----|-----------------|" >> "${GITHUB_STEP_SUMMARY}"

IMAGE_VARS=(
  "ENV_MANAGER_IMAGE"
  "ENV_FLUENTBIT_EXPORTER_IMAGE"
  "ENV_FLUENTBIT_IMAGE"
  "ENV_OTEL_COLLECTOR_IMAGE"
  "ENV_SELFMONITOR_IMAGE"
  "ENV_ALPINE_IMAGE"
)

# Source .env file
source .env

for var_name in "${IMAGE_VARS[@]}"; do
  # Get the image value
  image="${!var_name}"

  # Extract image name and tag
  if [[ "$image" =~ ^(.+):(.+)$ ]]; then
    image_name="${BASH_REMATCH[1]}"
    tag="${BASH_REMATCH[2]}"
    
    alternative_tag="N/A"
    
    # If tag is "main", try to get alternative tags from registry
    if [ "$tag" = "main" ]; then
        # Get all tags with their digests
        tags_with_digests=$(gcloud artifacts docker tags list "${image_name}" --format=json 2>/dev/null || echo "[]")
        
        # Get the digest for "main" tag
        main_digest=$(echo "${tags_with_digests}" | jq -r '.[] | select(.tag | endswith("/main")) | .version' 2>/dev/null | head -n1)
        
        if [ -n "${main_digest}" ] && [ "${main_digest}" != "null" ]; then
          # Find all tags with the same digest (excluding "main" itself)
          alternative_tags=$(echo "${tags_with_digests}" | \
            jq -r --arg digest "${main_digest}" '.[] | select(.version == $digest and (.tag | endswith("/main") | not)) | .tag' 2>/dev/null | \
            sed 's|.*/||' | \
            sort -V | \
            tr '\n' ', ' | \
            sed 's/,$//' || true)
          
          if [ -n "${alternative_tags}" ]; then
            alternative_tag="${alternative_tags}"
          else
            alternative_tag="⚠️ no matching tags found"
          fi
        fi
    fi
    
    echo "| \`$var_name\` | \`$image_name\` | \`$tag\` | \`$alternative_tag\` |" >> "${GITHUB_STEP_SUMMARY}"
  else
    echo "| \`$var_name\` | \`$image\` | N/A | N/A |" >> "${GITHUB_STEP_SUMMARY}"
  fi
done

echo "" >> "${GITHUB_STEP_SUMMARY}"
