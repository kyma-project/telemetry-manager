#!/bin/sh

# standard bash error handling
set -o nounset  # treat unset variables as an error and exit immediately.
set -o errexit  # exit immediately when a command fails.
set -E          # needs to be set if we want the ERR trap
set -o pipefail # prevents errors in a pipeline from being masked

PROMETHEUS_NAMESPACE="prometheus"
HELM_PROM_RELEASE="prometheus"
TRACE_NAMESPACE="trace-load-test"
METRIC_NAMESPACE="metric-load-test"
MAX_PIPELINE="false"
BACKPRESSURE_TEST="false"
TEST_TARGET="traces"
TEST_NAME="No Name"

while getopts m:b:n:t: flag; do
    case "$flag" in
        m) MAX_PIPELINE="${OPTARG}" ;;
        b) BACKPRESSURE_TEST="${OPTARG}" ;;
        n) TEST_NAME="${OPTARG}" ;;
        t) TEST_TARGET="${OPTARG}"
    esac
done

# shellcheck disable=SC2112
function setup() {

    kubectl create namespace $PROMETHEUS_NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
    # Deploy prometheus
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
    helm repo update
    helm upgrade --install -n ${PROMETHEUS_NAMESPACE} ${HELM_PROM_RELEASE} prometheus-community/kube-prometheus-stack -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/performance-tests/traces/assets/values.yaml --set grafana.adminPassword=myPwd

    if [ "$TEST_TARGET" = "traces" ];
    then
        setup_trace
    fi

    if [ "$TEST_TARGET" = "metrics" ];
    then
        setup_metric
    fi

    if [ "$TEST_TARGET" = "metricagent" ];
    then
        setup_metric_agent
    fi
}

# shellcheck disable=SC2112
function setup_trace() {
    if "$MAX_PIPELINE"; then
        kubectl apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/performance-tests/traces/assets/trace-max-pipeline.yaml
    fi
    # Deploy test setup
    kubectl apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/performance-tests/traces/assets/trace-load-test-setup.yaml

    if "$BACKPRESSURE_TEST"; then
        kubectl apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/performance-tests/traces/assets/trace-backpressure-config.yaml
    fi
}

function setup_metric() {
    if "$MAX_PIPELINE"; then
        kubectl apply -f metric-max-pipeline.yaml
    fi

    # Deploy test setup
    kubectl apply -f metric-load-test-setup.yaml

    if "$BACKPRESSURE_TEST"; then
        kubectl apply -f metric-backpressure-config.yaml
    fi
}

function setup_metric_agent() {
    if "$MAX_PIPELINE"; then
        kubectl apply -f metric-agent-max-pipeline.yaml
    fi

    # Deploy test setup
    kubectl apply -f metric-agent-test-setup.yaml

    if "$BACKPRESSURE_TEST"; then
        kubectl apply -f metric-backpressure-config.yaml
    fi
}

# shellcheck disable=SC2112
function wait_for_resources() {

  kubectl -n ${PROMETHEUS_NAMESPACE} rollout status statefulset prometheus-prometheus-kube-prometheus-prometheus

  if [ "$TEST_TARGET" = "traces" ];
  then
      wait_for_trace_resources
  fi

  if [ "$TEST_TARGET" = "metrics" ];
  then
      wait_for_metric_resources
  fi

  if [ "$TEST_TARGET" = "metricagent" ];
  then
      wait_for_metric_agent_resources
  fi

  echo "Running Tests"
}

# shellcheck disable=SC2112
function wait_for_trace_resources() {

    kubectl -n kyma-system rollout status deployment telemetry-trace-collector
    kubectl -n ${TRACE_NAMESPACE} rollout status deployment trace-load-generator
    kubectl -n ${TRACE_NAMESPACE} rollout status deployment trace-receiver
}

# shellcheck disable=SC2112
function wait_for_metric_resources() {

    kubectl -n kyma-system rollout status deployment telemetry-metric-gateway

    kubectl -n ${METRIC_NAMESPACE} rollout status deployment metric-load-generator
    kubectl -n ${METRIC_NAMESPACE} rollout status deployment metric-receiver
}

# shellcheck disable=SC2112
function wait_for_metric_agent_resources() {

    kubectl -n kyma-system rollout status deployment telemetry-metric-gateway
    kubectl -n kyma-system rollout status daemonset telemetry-metric-agent
    kubectl -n ${METRIC_NAMESPACE} rollout status deployment metric-agent-load-generator
    kubectl -n ${METRIC_NAMESPACE} rollout status deployment metric-receiver
}

# shellcheck disable=SC2112
function cleanup() {
    kubectl -n ${PROMETHEUS_NAMESPACE} port-forward $(kubectl -n ${PROMETHEUS_NAMESPACE} get service -l app=kube-prometheus-stack-prometheus -oname) 9090 &
    sleep 3

    echo "Test results collecting"
    if [ "$TEST_TARGET" = "traces" ]; then
        RECEIVED=$(curl -fs --data-urlencode 'query=round(avg(sum(rate(otelcol_receiver_accepted_spans{service="telemetry-trace-collector-metrics"}[20m]))))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

        EXPORTED=$(curl -fs --data-urlencode 'query=round(avg(sum(rate(otelcol_exporter_sent_spans{exporter=~"otlp/load-test.*"}[20m]))))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

        QUEUE=$(curl -fs --data-urlencode 'query=avg(sum(otelcol_exporter_queue_size{service="telemetry-trace-collector-metrics"}))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

        MEMORY=$(curl -fs --data-urlencode 'query=round((sum(container_memory_working_set_bytes{namespace="kyma-system", container="collector"} * on(namespace,pod) group_left(workload) namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-trace-collector"}) by (pod)) / 1024 / 1024)' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

        CPU=$(curl -fs --data-urlencode 'query=round(sum(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{namespace="kyma-system"} * on(namespace,pod) group_left(workload) namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-trace-collector"}) by (pod), 0.1)' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

        kill %1

        if "$MAX_PIPELINE"; then
          kubectl delete -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/performance-tests/traces/assets/trace-max-pipeline.yaml
        fi

        if "$BACKPRESSURE_TEST"; then
            kubectl delete -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/performance-tests/traces/assets/trace-backpressure-config.yaml
        fi

        kubectl delete -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/performance-tests/traces/assets/trace-load-test-setup.yaml

        helm delete -n ${PROMETHEUS_NAMESPACE} ${HELM_PROM_RELEASE}

        kubectl delete namespace $PROMETHEUS_NAMESPACE

        echo "\nPrinting Test Results for $TEST_NAME $TEST_TARGET, Multi Pipeline $MAX_PIPELINE, Backpressure $BACKPRESSURE_TEST\n"
        printf "|%-10s|%-30s|%-30s|%-30s|%-30s|%-30s|\n" "" "Receiver Accepted Span/sec" "Exporter Exported Span/sec" "Exporter Queue Size" "Pod Memory Usage(MB)" "Pod CPU Usage"
        printf "|%-10s|%-30s|%-30s|%-30s|%-30s|%-30s|\n" "$TEST_NAME" "$RECEIVED" "$EXPORTED" "$QUEUE" "${MEMORY//$'\n'/,}" "${CPU//$'\n'/,}"
    fi

    if [ "$TEST_TARGET" = "metrics" ]; then
        RECEIVED=$(curl -fs --data-urlencode 'query=round(avg(sum(rate(otelcol_receiver_accepted_metric_points{service="telemetry-metric-gateway-metrics"}[20m]))))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

        EXPORTED=$(curl -fs --data-urlencode 'query=round(avg(sum(rate(otelcol_exporter_sent_metric_points{exporter=~"otlp/load-test.*"}[20m]))))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

        QUEUE=$(curl -fs --data-urlencode 'query=avg(sum(otelcol_exporter_queue_size{service="telemetry-metric-gateway-metrics"}))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

        MEMORY=$(curl -fs --data-urlencode 'query=round((sum(container_memory_working_set_bytes{namespace="kyma-system", pod=~"telemetry-metric-gateway.*", container="collector"}) by (pod)) / 1024 / 1024)' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

        CPU=$(curl -fs --data-urlencode 'query=round(sum(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{namespace="kyma-system", pod=~"telemetry-metric-gateway.*"}) by (pod), 0.1)' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')
        kill %1


        if "$MAX_PIPELINE"; then
          kubectl delete -f metric-max-pipeline.yaml
        fi

        if "$BACKPRESSURE_TEST"; then
            kubectl delete -f metric-backpressure-config.yaml
        fi

        kubectl delete -f metric-load-test-setup.yaml

        helm delete -n ${PROMETHEUS_NAMESPACE} ${HELM_PROM_RELEASE}

        kubectl delete namespace $PROMETHEUS_NAMESPACE

        echo "\nPrinting Test Results for $TEST_NAME $TEST_TARGET, Multi Pipeline $MAX_PIPELINE, Backpressure $BACKPRESSURE_TEST\n"
        printf "|%-10s|%-30s|%-30s|%-30s|%-30s|%-30s|\n" "" "Receiver Accepted Metric/sec" "Exporter Exported Metric/sec" "Exporter Queue Size" "Pod Memory Usage(MB)" "Pod CPU Usage"
        printf "|%-10s|%-30s|%-30s|%-30s|%-30s|%-30s|\n" "$TEST_NAME" "$RECEIVED" "$EXPORTED" "$QUEUE" "${MEMORY//$'\n'/,}" "${CPU//$'\n'/,}"

    fi

    if [ "$TEST_TARGET" = "metricagent" ]; then
        RECEIVED=$(curl -fs --data-urlencode 'query=round(avg(sum(rate(otelcol_receiver_accepted_metric_points{service="telemetry-metric-agent-metrics"}[20m]))))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

        EXPORTED=$(curl -fs --data-urlencode 'query=round(avg(sum(rate(otelcol_exporter_sent_metric_points{service=~"telemetry-metric-agent-metrics"}[20m]))))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

        QUEUE=$(curl -fs --data-urlencode 'query=avg(sum(otelcol_exporter_queue_size{service="telemetry-metric-agent-metrics"}))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

        MEMORY=$(curl -fs --data-urlencode 'query=round((sum(container_memory_working_set_bytes{namespace="kyma-system", pod=~"telemetry-metric-agent.*", container="collector"}) by (pod)) / 1024 / 1024)' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

        CPU=$(curl -fs --data-urlencode 'query=round(sum(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{namespace="kyma-system", pod=~"telemetry-metric-agent.*"}) by (pod), 0.1)' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

        kill %1

        if "$MAX_PIPELINE"; then
          kubectl delete -f metric-agent-max-pipeline.yaml
        fi

        if "$BACKPRESSURE_TEST"; then
            kubectl delete -f metric-backpressure-config.yaml
        fi

        kubectl delete -f metric-agent-test-setup.yaml

        helm delete -n ${PROMETHEUS_NAMESPACE} ${HELM_PROM_RELEASE}

        kubectl delete namespace $PROMETHEUS_NAMESPACE

        echo "\nPrinting Test Results for $TEST_NAME $TEST_TARGET, Multi Pipeline $MAX_PIPELINE, Backpressure $BACKPRESSURE_TEST\n"
        printf "|%-10s|%-30s|%-30s|%-30s|%-30s|%-30s|\n" "" "Receiver Accepted Metric/sec" "Exporter Exported Metric/sec" "Exporter Queue Size" "Pod Memory Usage(MB)" "Pod CPU Usage"
        printf "|%-10s|%-30s|%-30s|%-30s|%-30s|%-30s|\n" "$TEST_NAME" "$RECEIVED" "$EXPORTED" "$QUEUE" "${MEMORY//$'\n'/,}" "${CPU//$'\n'/,}"
    fi

}

echo "$TEST_NAME Load Test for $TEST_TARGET, Multi Pipeline $MAX_PIPELINE, Backpressure $BACKPRESSURE_TEST"
echo "--------------------------------------------"

trap cleanup EXIT
setup
wait_for_resources
# wait 20 minutes until test finished
sleep 1200

