#!/usr/bin/env bash

# standard bash error handling
set -o nounset  # treat unset variables as an error and exit immediately.
set -o errexit  # exit immediately when a command fails.
set -E          # must be set if you want the ERR trap
set -o pipefail # prevents errors in a pipeline from being masked

# Variables (you need to set these)
REPO_OWNER="kyma-project"
REPO_NAME="telemetry-manager"
WORKFLOW_NAME="Build Image"

# retry until workflow conclusion is success and status is completed
# timeout after 15 minutes

TIMEOUT=900

START_TIME=$SECONDS

found=false
status=""
conclusion=""

until [[ $status == "completed" ]]; do
    # Wait for timeout
    if (( SECONDS - START_TIME > TIMEOUT )); then
        echo "Timeout reached: Workflow not found within $(( TIMEOUT/60 )) minutes"
        exit 1
    fi

    echo "Waiting for workflow: $WORKFLOW_NAME"

    # Get the latest workflow run status
    response=$(curl -L \
        -H "Accept: application/vnd.github+json" \
        -H "Authorization: Bearer $GITHUB_TOKEN" \
        -H "X-GitHub-Api-Version: 2022-11-28" \
        https://api.github.com/repos/$REPO_OWNER/$REPO_NAME/actions/runs)


    # Check if .workflow_runs exists and is not null
    if [ "$(echo "$response" | jq -r '.workflow_runs')" == "null" ]; then
        echo "$response"
        exit 1
    fi

    # Extract all head_sha and status from response and put it into an array
    workflows=$(echo $response | jq -c '.workflow_runs[] | {name, head_sha, status, conclusion}')

    # Iterate over the JSON objects
    while IFS= read -r workflow; do
        workflow_name=$(echo "$workflow" | jq -r '.name')
        head_sha=$(echo "$workflow" | jq -r '.head_sha')
        status=$(echo "$workflow" | jq -r '.status')
        conclusion=$(echo "$workflow" | jq -r '.conclusion')

        if [[ "$head_sha" == "$COMMIT_SHA" && "$workflow_name" == "$WORKFLOW_NAME" ]]; then
            found=true
            break
        fi
    done <<< "$workflows"

    if [ "$found" = false ]; then
        echo "Workflow not found"
        exit 1
    fi

    # Output the results
    echo "Latest head SHA: $head_sha"
    echo "Status: $status"
    echo "Conclusion: $conclusion"
    echo ""

    sleep 10
done

if [ $conclusion != "success" ]; then
    echo "Workflow $status with conclusion: $conclusion"
    exit 1
fi

echo "Workflow '$WORKFLOW_NAME' $status with conclusion: $conclusion"