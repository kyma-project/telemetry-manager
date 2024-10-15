#!/usr/bin/env bash

# standard bash error handling
set -o nounset  # treat unset variables as an error and exit immediately.
set -o errexit  # exit immediately when a command fails.
set -E          # must be set if you want the ERR trap
set -o pipefail # prevents errors in a pipeline from being masked

# Expected variables:
#             IMAGE_REPO - binary image repository
#             GITHUB_TOKEN - github token
#             COMMIT_SHA - commit SHA which triggered the workflow
#             QUERY_INTERVAL - time to wait between queries in seconds

PROTOCOL=docker://

# timeout after 15 minutes
TIMEOUT=900
START_TIME=$SECONDS

until $(skopeo list-tags ${PROTOCOL}${IMAGE_REPO} | jq '.Tags|any(. == env.COMMIT_SHA)'); do
  if (( SECONDS - START_TIME > TIMEOUT )); then
    echo "Timeout reached: ${IMAGE_REPO}:${COMMIT_SHA} not found within $(( TIMEOUT/60 )) minutes"
    exit 1
  fi
  echo "Waiting for binary image: ${IMAGE_REPO}:${COMMIT_SHA}"
  echo "Commit SHA: $COMMIT_SHA"
  sleep "$QUERY_INTERVAL"
done

echo "Binary image: ${IMAGE_REPO}:${COMMIT_SHA} available"
