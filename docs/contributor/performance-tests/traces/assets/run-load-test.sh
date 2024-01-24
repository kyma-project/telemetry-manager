#!/bin/sh

PROMETHEUS_NAMESPACE="prometheus"
HELM_PROM_RELEASE="prometheus"
NAMESPACE="trace-load-test"
MAX_PIPELINE="false"
BACKPRESSURE_TEST="false"

while getopts m:b: flag; do
    case "$flag" in
        m) MAX_PIPELINE="true" ;;
        b) BACKPRESSURE_TEST="true" ;;
    esac
done

# shellcheck disable=SC2112
function setup() {

    # Deploy prometheus
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
    helm repo update
    helm upgrade --install -n ${PROMETHEUS_NAMESPACE} ${HELM_PROM_RELEASE} prometheus-community/kube-prometheus-stack -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/performance-tests/traces/assets/values.yaml --set grafana.adminPassword=myPwd

    if "$MAX_PIPELINE"; then
        kubectl apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/performance-tests/traces/assets/trace-max-pipeline.yaml
    fi
    # Deploy test setup
    kubectl apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/performance-tests/traces/assets/trace-load-test-setup.yaml

    if "$BACKPRESSURE_TEST"; then
        kubectl apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/performance-tests/traces/assets/trace-backpressure-config.yaml
        sleep 3
        kubectl rollout restart deployment trace-receiver -n trace-load-test
    fi
}

# shellcheck disable=SC2112
function wait_for_resources() {
    while [ -z $TRACEPIPELINE_READY ]; do
        TRACEPIPELINE_READY=$(kubectl get tracepipelines.telemetry.kyma-project.io load-test-1 -o jsonpath='{.status.conditions[?(@.type=="Running")].type}')
        echo "Waiting for TracePipeline 1"
        [ -z "$TRACEPIPELINE_READY" ] && sleep 10
    done

    if "$MAX_PIPELINE"; then

        while [ -z $TRACEPIPELINE_READY ]; do
            TRACEPIPELINE_READY=$(kubectl get tracepipelines.telemetry.kyma-project.io load-test-2 -o jsonpath='{.status.conditions[?(@.type=="Running")].type}')
            echo "Waiting for TracePipeline 2"
            [ -z "$TRACEPIPELINE_READY" ] && sleep 10
        done

        while [ -z $TRACEPIPELINE_READY ]; do
            TRACEPIPELINE_READY=$(kubectl get tracepipelines.telemetry.kyma-project.io load-test-3 -o jsonpath='{.status.conditions[?(@.type=="Running")].type}')
            echo "Waiting for TracePipeline 3"
            [ -z "$TRACEPIPELINE_READY" ] && sleep 10
        done
    fi
    kubectl -n ${PROMETHEUS_NAMESPACE} rollout status statefulset prometheus-prometheus-kube-prometheus-prometheus
    kubectl -n ${NAMESPACE} rollout status deployment trace-load-generator
    kubectl -n ${NAMESPACE} rollout status deployment trace-receiver

    echo "Running Tests"
}

# shellcheck disable=SC2112
function cleanup() {
    kubectl -n ${PROMETHEUS_NAMESPACE} port-forward $(kubectl -n ${PROMETHEUS_NAMESPACE} get service -l app=kube-prometheus-stack-prometheus -oname) 9090 &
    sleep 3

    echo "Test results collecting"

    curl -fs --data-urlencode 'query=avg(sum(rate(otelcol_receiver_accepted_spans{service="telemetry-trace-collector-metrics"}[1m])))' localhost:9090/api/v1/query | jq -r '.data.result[] | [ "Receiver accepted spans", "Average", .value[1] ] | @csv' | xargs printf "\033[0;31m %s \033[0m \n"

    curl -fs --data-urlencode 'query=avg(sum(rate(otelcol_exporter_sent_spans{exporter=~"otlp/load-test.*"}[1m])))' localhost:9090/api/v1/query | jq -r '.data.result[] | [ "Exporter exported spans", "Average", .value[1] ] | @csv' | xargs printf "\033[0;31m %s \033[0m \n"

    curl -fs --data-urlencode 'query=avg(sum(otelcol_exporter_queue_size{service="telemetry-trace-collector-metrics"}))' localhost:9090/api/v1/query | jq -r '.data.result[] | [ "Exporter queue size", "Average", .value[1] ] | @csv' | xargs printf "\033[0;31m %s \033[0m \n"

    curl -fs --data-urlencode 'query=(sum(container_memory_working_set_bytes{namespace="kyma-system", container="collector"} * on(namespace,pod) group_left(workload) namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-trace-collector"}) by (pod)) / 1024 / 1024' localhost:9090/api/v1/query | jq -r '.data.result[] | [ "Pod memory", .metric.pod, .value[1] ] | @csv' | xargs printf "\033[0;31m %s \033[0m \n"

    curl -fs --data-urlencode 'query=sum(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{namespace="kyma-system"} * on(namespace,pod) group_left(workload) namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-trace-collector"}) by (pod)' localhost:9090/api/v1/query | jq -r '.data.result[] | [ "Pod CPU", .metric.pod, .value[1] ] | @csv' | xargs printf "\033[0;31m %s \033[0m \n"
    kill %1

    if "$MAX_PIPELINE"; then
      kubectl delete -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/performance-tests/traces/assets/trace-max-pipeline.yaml
    fi
    kubectl delete -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/performance-tests/traces/assets/trace-load-test-setup.yaml

    helm delete -n ${PROMETHEUS_NAMESPACE} ${HELM_PROM_RELEASE}
}

echo "TracePipeline Load Test"
echo "--------------------------------------------"
trap cleanup EXIT
setup
wait_for_resources
# wait 20 minutes until test finished
sleep 1200





