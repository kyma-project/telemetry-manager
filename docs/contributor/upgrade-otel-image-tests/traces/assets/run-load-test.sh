#!/bin/sh
PROMETHEUS_NAMESPACE="prometheus"
HELM_PROM_RELEASE="prometheus"
OTEL_VERSION="${OTEL_VERSION:-0.89.0}"
NAMESPACE="trace-load-test"

# shellcheck disable=SC2112
function setup() {
    # Deploy prometheus
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
    helm repo update
    helm upgrade --install -n ${PROMETHEUS_NAMESPACE} ${HELM_PROM_RELEASE} prometheus-community/kube-prometheus-stack -f values.yaml --set grafana.adminPassword=myPwd

    # Deploy test setup
    kubectl apply -f trace-load-test-setup.yaml
}

# shellcheck disable=SC2112
function wait_for_resources() {
    while [ -z $TRACEPIPELINE_READY ]; do
        TRACEPIPELINE_READY=$(kubectl get tracepipelines.telemetry.kyma-project.io load-test -o jsonpath='{.status.conditions[?(@.type=="Running")].type}')
        echo "Waiting for TracePipeline"
        [ -z "$TRACEPIPELINE_READY" ] && sleep 10
    done

    kubectl -n ${PROMETHEUS_NAMESPACE} rollout status statefulset prometheus-prometheus-kube-prometheus-prometheus
    kubectl -n ${NAMESPACE} rollout status deployment trace-load-generator
    kubectl -n ${NAMESPACE} rollout status deployment trace-receiver

    echo "Running Tests"
}

# shellcheck disable=SC2112
function cleanup() {
    RESULTS_DIRECTORY="$(pwd)/results"
    mkdir -pv $RESULTS_DIRECTORY
    RESULTS_TRACE_FILENAME="${RESULTS_DIRECTORY}/otel-${OTEL_VERSION}_traces_load_test.csv"
    kubectl -n ${PROMETHEUS_NAMESPACE} port-forward $(kubectl -n ${PROMETHEUS_NAMESPACE} get service -l app=kube-prometheus-stack-prometheus -oname) 9090 &
    sleep 3

    echo "Test results collecting"

    curl -g 'localhost:9090/api/v1/query?query=avg(sum(rate(otelcol_receiver_accepted_spans{service="telemetry-trace-collector-metrics"}[1m])))' | jq -r '.data.result[] | [ "Receiver accepted spans", "Average", .value[1] ] | @csv' >> $RESULTS_TRACE_FILENAME

    curl -g 'localhost:9090/api/v1/query?query=avg(sum(rate(otelcol_exporter_sent_spans{exporter="otlp/load-test"}[1m])))' | jq -r '.data.result[] | [ "Exporter exported spans", "Average", .value[1] ] | @csv' >> $RESULTS_TRACE_FILENAME

    curl -g 'localhost:9090/api/v1/query?query=avg(sum(otelcol_exporter_queue_size{service="telemetry-trace-collector-metrics"}))' | jq -r '.data.result[] | [ "Exporter queue size", "Average", .value[1] ] | @csv' >> $RESULTS_TRACE_FILENAME

    curl -fs --data-urlencode 'query=sum(container_memory_working_set_bytes{pod=~"telemetry-trace-collector.*", container=~".*"}) by (pod)' localhost:9090/api/v1/query | jq -r '.data.result[] | [ "Pod memory", .metric.pod, .value[1] ] | @csv' >> $RESULTS_TRACE_FILENAME
    kill %1

    kubectl delete -f trace-load-test-setup.yaml

    helm delete -n ${PROMETHEUS_NAMESPACE} ${HELM_PROM_RELEASE}
}

echo "Single TracePipeline Load Test"
echo "--------------------------------------------"
trap cleanup EXIT
setup
wait_for_resources
# wait 20 minutes until test finished
sleep 20m





