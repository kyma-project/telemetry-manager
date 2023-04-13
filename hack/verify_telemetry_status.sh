#!/bin/bash

function get_telemetry_status () {
	local number=1
	while [[ $number -le 100 ]] ; do
		echo ">--> checking telemetry status #$number"
		local STATUS=$(kubectl get telemetry -n kyma-system telemetry-default -o jsonpath='{.status.state}')
		echo "telemetry status: ${STATUS:='UNKNOWN'}"
		[[ "$STATUS" == "Ready" ]] && return 0
		sleep 5
        	((number = number + 1))
	done

	kubectl get all --all-namespaces
	exit 1
}

get_telemetry_status
