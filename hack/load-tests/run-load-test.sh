#!/usr/bin/env bash

# standard bash error handling
set -o nounset  # treat unset variables as an error and exit immediately.
set -o errexit  # exit immediately when a command fails.
set -E          # needs to be set if we want the ERR trap
set -o pipefail # prevents errors in a pipeline from being masked

PROMETHEUS_NAMESPACE="prometheus"
HELM_PROM_RELEASE="prometheus"
TRACE_NAMESPACE="trace-load-test"
METRIC_NAMESPACE="metric-load-test"
LOG_NAMESPACE="log-load-test"
MAX_PIPELINE="false"
BACKPRESSURE_TEST="false"
TEST_TARGET="traces"
TEST_NAME="No Name"
TEST_DURATION=1200

while getopts m:b:n:t:d: flag; do
    case "$flag" in
        m) MAX_PIPELINE="${OPTARG}" ;;
        b) BACKPRESSURE_TEST="${OPTARG}" ;;
        n) TEST_NAME="${OPTARG}" ;;
        t) TEST_TARGET="${OPTARG}" ;;
        d) TEST_DURATION=${OPTARG} ;;
    esac
done

# shellcheck disable=SC2112
function setup() {

    kubectl create namespace $PROMETHEUS_NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
    # Deploy prometheus
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
    helm repo update
    helm upgrade --install -n ${PROMETHEUS_NAMESPACE} ${HELM_PROM_RELEASE} prometheus-community/kube-prometheus-stack -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/telemetry-load-test/assets/values.yaml --set grafana.adminPassword=myPwd

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

   if [ "$TEST_TARGET" = "logs-fluentbit" ];
    then
        setup_fluentbit
  fi
}

# shellcheck disable=SC2112
function setup_trace() {
    if "$MAX_PIPELINE"; then
        kubectl apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/telemetry-load-test/assets/trace-max-pipeline.yaml
    fi
    # Deploy test setup
    kubectl apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/telemetry-load-test/assets/trace-load-test-setup.yaml

    if "$BACKPRESSURE_TEST"; then
        kubectl apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/telemetry-load-test/assets/trace-backpressure-config.yaml
    fi
}

function setup_metric() {
    if "$MAX_PIPELINE"; then
        kubectl apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/telemetry-load-test/assets/metric-max-pipeline.yaml
    fi

    # Deploy test setup
    kubectl apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/telemetry-load-test/assets/metric-load-test-setup.yaml

    if "$BACKPRESSURE_TEST"; then
        kubectl apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/telemetry-load-test/assets/metric-backpressure-config.yaml
    fi
}

function setup_metric_agent() {
    # Deploy test setup
    kubectl apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/telemetry-load-test/assets/metric-agent-test-setup.yaml

    if "$BACKPRESSURE_TEST"; then
        kubectl apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/telemetry-load-test/assets/metric-agent-backpressure-config.yaml
    fi
}

function setup_fluentbit() {
    if "$MAX_PIPELINE"; then
        kubectl apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/telemetry-load-test/assets/log-fluentbit-max-pipeline.yaml
    fi
    # Deploy test setup
    kubectl apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/telemetry-load-test/assets/log-fluentbit-test-setup.yaml

    if "$BACKPRESSURE_TEST"; then
        kubectl apply -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/telemetry-load-test/assets/log-fluentbit-backpressure-config.yaml
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

  if [ "$TEST_TARGET" = "logs-fluentbit" ];
  then
      wait_for_fluentbit_resources
  fi

  echo "\nRunning Tests\n"
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
function wait_for_fluentbit_resources() {
    kubectl -n ${LOG_NAMESPACE} rollout status deployment log-receiver
    kubectl -n kyma-system rollout status daemonset telemetry-fluent-bit
    kubectl -n ${LOG_NAMESPACE} rollout status deployment log-load-generator
}

# shellcheck disable=SC2112
function cleanup() {
    kubectl -n ${PROMETHEUS_NAMESPACE} port-forward $(kubectl -n ${PROMETHEUS_NAMESPACE} get service -l app=kube-prometheus-stack-prometheus -oname) 9090 &
    sleep 3

    echo "Test results collecting"
    if [ "$TEST_TARGET" = "traces" ]; then
        get_result_and_cleanup_trace
    fi

    if [ "$TEST_TARGET" = "metrics" ]; then
        get_result_and_cleanup_metric
    fi

    if [ "$TEST_TARGET" = "metricagent" ]; then
        get_result_and_cleanup_metricagent
    fi

    if [ "$TEST_TARGET" = "logs-fluentbit" ]; then
        get_result_and_cleanup_fluentbit
    fi

    helm delete -n ${PROMETHEUS_NAMESPACE} ${HELM_PROM_RELEASE}

    kubectl delete namespace $PROMETHEUS_NAMESPACE

}

# shellcheck disable=SC2112
function get_result_and_cleanup_trace() {
    RECEIVED=$(curl -fs --data-urlencode 'query=round(avg(sum(rate(otelcol_receiver_accepted_spans{service="telemetry-trace-collector-metrics"}[20m]))))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

    EXPORTED=$(curl -fs --data-urlencode 'query=round(avg(sum(rate(otelcol_exporter_sent_spans{exporter=~"otlp/load-test.*"}[20m]))))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

    QUEUE=$(curl -fs --data-urlencode 'query=avg(sum(otelcol_exporter_queue_size{service="telemetry-trace-collector-metrics"}))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

    MEMORY=$(curl -fs --data-urlencode 'query=round((sum(container_memory_working_set_bytes{namespace="kyma-system", container="collector"} * on(namespace,pod) group_left(workload) namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-trace-collector"}) by (pod)) / 1024 / 1024)' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

    CPU=$(curl -fs --data-urlencode 'query=round(sum(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{namespace="kyma-system"} * on(namespace,pod) group_left(workload) namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-trace-collector"}) by (pod), 0.1)' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

    kill %1

    restarts=$(kubectl -n kyma-system get pod -l app.kubernetes.io/name=telemetry-trace-collector -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq | awk '{sum += $1} END {print sum}')

    if "$MAX_PIPELINE"; then
      kubectl delete -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/telemetry-load-test/assets/trace-max-pipeline.yaml
    fi

    if "$BACKPRESSURE_TEST"; then
        kubectl delete -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/telemetry-load-test/assets/trace-backpressure-config.yaml
    fi

    kubectl delete -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/telemetry-load-test/assets/trace-load-test-setup.yaml

    echo "\nTrace Gateway got $restarts time restarted\n"

    echo "\nPrinting Test Results for $TEST_NAME $TEST_TARGET, Multi Pipeline $MAX_PIPELINE, Backpressure $BACKPRESSURE_TEST\n"
    printf "|%-10s|%-30s|%-30s|%-30s|%-30s|%-30s|\n" "" "Receiver Accepted Span/sec" "Exporter Exported Span/sec" "Exporter Queue Size" "Pod Memory Usage(MB)" "Pod CPU Usage"
    printf "|%-10s|%-30s|%-30s|%-30s|%-30s|%-30s|\n" "$TEST_NAME" "$RECEIVED" "$EXPORTED" "$QUEUE" "${MEMORY//$'\n'/,}" "${CPU//$'\n'/,}"
}

# shellcheck disable=SC2112
function get_result_and_cleanup_metric() {
    RECEIVED=$(curl -fs --data-urlencode 'query=round(avg(sum(rate(otelcol_receiver_accepted_metric_points{service="telemetry-metric-gateway-metrics"}[20m]))))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

    EXPORTED=$(curl -fs --data-urlencode 'query=round(avg(sum(rate(otelcol_exporter_sent_metric_points{exporter=~"otlp/load-test.*"}[20m]))))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

    QUEUE=$(curl -fs --data-urlencode 'query=avg(sum(otelcol_exporter_queue_size{service="telemetry-metric-gateway-metrics"}))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

    MEMORY=$(curl -fs --data-urlencode 'query=round((sum(container_memory_working_set_bytes{namespace="kyma-system", pod=~"telemetry-metric-gateway.*", container="collector"}) by (pod)) / 1024 / 1024)' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

    CPU=$(curl -fs --data-urlencode 'query=round(sum(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{namespace="kyma-system", pod=~"telemetry-metric-gateway.*"}) by (pod), 0.1)' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')
    kill %1

    restarts=$(kubectl -n kyma-system get pod -l app.kubernetes.io/name=telemetry-metric-gateway -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq | awk '{sum += $1} END {print sum}')

    if "$MAX_PIPELINE"; then
      kubectl delete -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/telemetry-load-test/assets/metric-max-pipeline.yaml
    fi

    if "$BACKPRESSURE_TEST"; then
        kubectl delete -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/telemetry-load-test/assets/metric-backpressure-config.yaml
    fi

    kubectl delete -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/telemetry-load-test/assets/metric-load-test-setup.yaml

    echo "\nMetric Gateway got $restarts time restarted\n"

    print_metric_result "$TEST_NAME" "$TEST_TARGET" "$MAX_PIPELINE" "$BACKPRESSURE_TEST" "$RECEIVED" "$EXPORTED" "$QUEUE" "$MEMORY" "$CPU"
}

# shellcheck disable=SC2112
function get_result_and_cleanup_metricagent() {
   RECEIVED=$(curl -fs --data-urlencode 'query=round(avg(sum(rate(otelcol_receiver_accepted_metric_points{service="telemetry-metric-agent-metrics"}[20m]))))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

   EXPORTED=$(curl -fs --data-urlencode 'query=round(avg(sum(rate(otelcol_exporter_sent_metric_points{service=~"telemetry-metric-agent-metrics"}[20m]))))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

   QUEUE=$(curl -fs --data-urlencode 'query=avg(sum(otelcol_exporter_queue_size{service="telemetry-metric-agent-metrics"}))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

   MEMORY=$(curl -fs --data-urlencode 'query=round((sum(container_memory_working_set_bytes{namespace="kyma-system", pod=~"telemetry-metric-agent.*", container="collector"}) by (pod)) / 1024 / 1024)' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

   CPU=$(curl -fs --data-urlencode 'query=round(sum(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{namespace="kyma-system", pod=~"telemetry-metric-agent.*"}) by (pod), 0.1)' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

   kill %1

   restartsGateway=$(kubectl -n kyma-system get pod -l app.kubernetes.io/name=telemetry-metric-gateway -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq | awk '{sum += $1} END {print sum}')

   restartsAgent=$(kubectl -n kyma-system get pod -l app.kubernetes.io/name=telemetry-metric-agent -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq | awk '{sum += $1} END {print sum}')

   if "$BACKPRESSURE_TEST"; then
       kubectl delete -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/telemetry-load-test/assets/metric-agent-backpressure-config.yaml
   fi

   kubectl delete -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/telemetry-load-test/assets/metric-agent-test-setup.yaml

   echo "\nTest run for $TEST_DURATION seconds\n"
   echo "\nMetric Gateway got $restartsGateway time restarted\n"
   echo "\nMetric Agent got $restartsAgent time restarted\n"

   print_metric_result "$TEST_NAME" "$TEST_TARGET" "$MAX_PIPELINE" "$BACKPRESSURE_TEST" "$RECEIVED" "$EXPORTED" "$QUEUE" "$MEMORY" "$CPU"
}

# shellcheck disable=SC2112
function get_result_and_cleanup_fluentbit() {
   RECEIVED=$(curl -fs --data-urlencode 'query=round((sum(rate(fluentbit_input_bytes_total{service="telemetry-fluent-bit-metrics", name="load-test-1"}[5m])) / 1024))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

   EXPORTED=$(curl -fs --data-urlencode 'query=round((sum(rate(fluentbit_output_proc_bytes_total{service="telemetry-fluent-bit-metrics"}[5m])) / 1024))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

   QUEUE=$(curl -fs --data-urlencode 'query=round((sum(rate(telemetry_fsbuffer_usage_bytes{service="telemetry-fluent-bit-exporter-metrics"}[5m])) / 1024))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

   MEMORY=$(curl -fs --data-urlencode 'query=round((sum(container_memory_working_set_bytes{namespace="kyma-system", container="fluent-bit"} * on(namespace,pod) group_left(workload) namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-fluent-bit"}) by (pod)) / 1024 / 1024)' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

   CPU=$(curl -fs --data-urlencode 'query=round(sum(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{namespace="kyma-system"} * on(namespace,pod) group_left(workload) namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-fluent-bit"}) by (pod), 0.1)' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

   kill %1

   restarts=$(kubectl -n kyma-system get pod -l app.kubernetes.io/name=fluent-bit -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq | awk '{sum += $1} END {print sum}')

   if "$MAX_PIPELINE"; then
       kubectl delete -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/telemetry-load-test/assets/log-fluentbit-max-pipeline.yaml
   fi

   if "$BACKPRESSURE_TEST"; then
       kubectl delete -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/telemetry-load-test/assets/log-fluentbit-backpressure-config.yaml
   fi

   kubectl delete -f https://raw.githubusercontent.com/kyma-project/telemetry-manager/main/docs/contributor/telemetry-load-test/assets/log-fluentbit-test-setup.yaml

   echo "\nLogPipeline Pods got $restarts time restarted\n"

   echo "\nPrinting Test Results for $TEST_NAME $TEST_TARGET, Multi Pipeline $MAX_PIPELINE, Backpressure $BACKPRESSURE_TEST\n"
   printf "|%-10s|%-35s|%-35s|%-30s|%-30s|%-30s|\n" "" "Input Bytes Processing Rate/sec" "Output Bytes Processing Rate/sec" "Filesystem Buffer Usage" "Pod Memory Usage(MB)" "Pod CPU Usage"
   printf "|%-10s|%-35s|%-35s|%-30s|%-30s|%-30s|\n" "$TEST_NAME" "$RECEIVED" "$EXPORTED" "$QUEUE" "${MEMORY//$'\n'/,}" "${CPU//$'\n'/,}"
}

function print_metric_result(){
    echo "\nPrinting Test Results for $1 $2, Multi Pipeline $3, Backpressure $4\n"
    printf "|%-10s|%-30s|%-30s|%-30s|%-30s|%-30s|\n" "" "Receiver Accepted Metric/sec" "Exporter Exported Metric/sec" "Exporter Queue Size" "Pod Memory Usage(MB)" "Pod CPU Usage"
    printf "|%-10s|%-30s|%-30s|%-30s|%-30s|%-30s|\n" "$1" "$5" "$6" "$7" "${8//$'\n'/,}" "${9//$'\n'/,}"
}

echo "$TEST_NAME Load Test for $TEST_TARGET, Multi Pipeline $MAX_PIPELINE, Backpressure $BACKPRESSURE_TEST, test duration $TEST_DURATION seconds"
echo "--------------------------------------------"

trap cleanup EXIT
setup
wait_for_resources
# wait 20 minutes until test finished
sleep $TEST_DURATION

