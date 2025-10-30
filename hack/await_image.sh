#!/usr/bin/env bash

# standard bash error handling
set -o nounset  # treat unset variables as an error and exit immediately.
set -o errexit  # exit immediately when a command fails.
set -E          # must be set if you want the ERR trap
set -o pipefail # prevents errors in a pipeline from being masked

# Expected variables:
#             IMAGE_REPO - binary image repository
#             IMAGE_TAG - event which triggered the workflow, for PRs it is the commit SHA, for push events it is the GITHUB_REF
#             QUERY_INTERVAL - time to wait between queries in seconds

PROTOCOL=docker://

# timeout after 15 minutes
TIMEOUT=900
START_TIME=$SECONDS

echo "Waiting for binary image: ${IMAGE_REPO}:${IMAGE_TAG}"
until $(skopeo list-tags ${PROTOCOL}${IMAGE_REPO} | jq '.Tags|any(. == env.TRIGGER)'); do
  if (( SECONDS - START_TIME > TIMEOUT )); then
    echo "Timeout reached: ${IMAGE_REPO}:${IMAGE_TAG} not found within $(( TIMEOUT/60 )) minutes"
    exit 1
  fi
  sleep "$QUERY_INTERVAL"
done

echo "Binary image: ${IMAGE_REPO}:${IMAGE_TAG} available"
