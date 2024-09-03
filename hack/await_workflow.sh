#!/usr/bin/env bash

# standard bash error handling
set -o nounset  # treat unset variables as an error and exit immediately.
set -o errexit  # exit immediately when a command fails.
set -E          # must be set if you want the ERR trap
set -o pipefail # prevents errors in a pipeline from being masked

# Variables (you need to set these)
REPO_OWNER="kyma-project"
REPO_NAME="telemetry-manager"
CHECK_NAME="Build-Image-Success"

# retry until check conclusion is success and status is completed
# timeout after 15 minutes

TIMEOUT=900

START_TIME=$SECONDS

found=false
status=""
conclusion=""

until [[ $status == "completed" ]]; do
    # Wait for timeout
    if (( SECONDS - START_TIME > TIMEOUT )); then
        echo "Timeout reached: Check not finished within $(( TIMEOUT/60 )) minutes"
        exit 1
    fi

    echo "Waiting for check: $CHECK_NAME"

    # Get the latest check run status
    response=$(curl -L \
        -H "Accept: application/vnd.github+json" \
        -H "Authorization: Bearer $GITHUB_TOKEN" \
        -H "X-GitHub-Api-Version: 2022-11-28" \
        https://api.github.com/repos/$REPO_OWNER/$REPO_NAME/commits/$COMMIT_SHA/check-runs)


    # Check if .check_runs exists and is not null
    if [ "$(echo "$response" | jq -r '.check_runs')" == "null" ]; then
        echo "$response"
        exit 1
    fi

    # Extract all head_sha and status from response and put it into an array
    checks=$(echo "$response" | jq -c '.check_runs[] | {name, head_sha, status, conclusion}')

    # Iterate over the JSON objects
    while IFS= read -r check; do
        check_name=$(echo "$check" | jq -r '.name')
        head_sha=$(echo "$check" | jq -r '.head_sha')
        status=$(echo "$check" | jq -r '.status')
        conclusion=$(echo "$check" | jq -r '.conclusion')

        if [[ "$head_sha" == "$COMMIT_SHA" && "$check_name" == "$CHECK_NAME" ]]; then
            found=true
            break
        fi
    done <<< "$checks"

    if [ "$found" = false ]; then
        echo "Check not yet found."
    fi

    # Output the results
    echo "Latest head SHA: $head_sha"
    echo "Status: $status"
    echo "Conclusion: $conclusion"
    echo ""

    sleep 10
done

if [ "$conclusion" != "success" ]; then
    echo "Check $status with conclusion: $conclusion"
    exit 1
fi

echo "Check '$CHECK_NAME' $status with conclusion: $conclusion"