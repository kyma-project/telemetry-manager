#!/usr/bin/env bash

# standard bash error handling
set -o nounset  # treat unset variables as an error and exit immediately.
set -o errexit  # exit immediately when a command fails.
set -E          # needs to be set if we want the ERR trap
set -o pipefail # prevents errors in a pipeline from being masked

function get_kyma_filename() {

	local _OS_TYPE=$1
	local _OS_ARCH=$2

	[ "$_OS_TYPE" == "Linux"   ] && [ "$_OS_ARCH" == "x86_64" ] && echo "kyma-linux"     ||
	[ "$_OS_TYPE" == "Linux"   ] && [ "$_OS_ARCH" == "arm64"  ] && echo "kyma-linux-arm" ||
	[ "$_OS_TYPE" == "Windows" ] && [ "$_OS_ARCH" == "x86_64" ] && echo "kyma.exe"       ||
	[ "$_OS_TYPE" == "Windows" ] && [ "$_OS_ARCH" == "arm64"  ] && echo "kyma-arm.exe"   ||
	[ "$_OS_TYPE" == "Darwin"  ] && [ "$_OS_ARCH" == "x86_64" ] && echo "kyma-darwin"
}

get_kyma_filename "$@"
