#!/usr/bin/env bash

# standard bash error handling
set -o nounset  # treat unset variables as an error and exit immediately.
set -o errexit  # exit immediately when a command fails.
set -E          # needs to be set if we want the ERR trap
set -o pipefail # prevents errors in a pipeline from being masked

readonly LOCALBIN=${LOCALBIN:-$(pwd)/bin}
readonly ARTIFACTS=${ARTIFACTS:-$(pwd)/artifacts}
readonly E2E_TESTS=${E2E_TESTS:-$(pwd)/test/e2e}
readonly INTEGRATION_TESTS=${INTEGRATION_TESTS:-$(pwd)/test/integration}
readonly GINKGO=${GINKGO:-$(LOCALBIN)/ginkgo}

function test_matchers() {
    echo "Setting-up test matchers"
    ${GINKGO} run .test/testkit/matchers/...
}

function artifacts_output() {
    mkdir -p ${ARTIFACTS}
    mv junit.xml ${ARTIFACTS}
}

function run_e2e_tests() {
    local test_suite=$1
    echo "Running e2e tests for suite: ${test_suite}"

    test_matchers
    ${GINKGO} run --tags e2e --junit-report=junit.xml --label-filter="${test_suite}" ${E2E_TESTS}
    artifacts_output
}

function run_upgrade_tests() {
    local test_suite=$1
    echo "Running upgrade tests for suite: ${test_suite}"
    
    ${GINKGO} run --tags e2e --junit-report=junit.xml --flake-attempts=5 --label-filter="${test_suite}" -v ${E2E_TESTS}
	artifacts_output
}

function run_integration_tests() {
    local test_suite=$1
    echo "Running integration tests for suite: ${test_suite}"
    
    test_matchers
    if [[ "${test_suite}" == "istio" ]]; then ./hack/deploy-istio.sh; fi
    ${GINKGO} run --tags ${test_suite} --junit-report=junit.xml "$INTEGRATION_TESTS/$test_suite"
    artifacts_output
}

function main() {
    local type=$1
    local test_suite=$2

    if [[ "${type}" == "e2e" ]] # E2E Tests
    then
        run_e2e_tests ${test_suite}
    elif [[ "${type}" == "upgrade" ]] # Upgrade Tests
    then
        run_upgrade_tests ${test_suite}
    elif [[ "${type}" == "integration" ]] # Integration Tests
    then
        run_integration_tests ${test_suite}
    else
        echo "Unknown test type: ${type}"
        exit 1
    fi
}

main $1 $2