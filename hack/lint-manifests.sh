#!/usr/bin/env bash

echo "Check that the CRDs manifests and their documentation are up-to-date"

make manifests

DIFF=$(git diff)
if [ -n "${DIFF}" ]; then 
    echo "ERROR: detected CRDs manifests and/or documentation that need to be updated"
    echo "
    To update the CRDs manifests and their documentation run:
        make manifests
    in the root of the repository and commit changes.
    "
    exit 1
fi

echo "CRDs manifests and their documentation are up-to-date"
