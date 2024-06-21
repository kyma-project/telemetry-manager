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
SELFMONITOR_NAMESPACE="self-monitor-load-test"
MAX_PIPELINE="false"
BACKPRESSURE_TEST="false"
TEST_TARGET="traces"
TEST_NAME="No Name"
TEST_DURATION=1200
OTEL_IMAGE="europe-docker.pkg.dev/kyma-project/prod/tpi/otel-collector:0.102.1-fbfb6cdc"

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

    kubectl label namespace kyma-system istio-injection=enabled --overwrite

    # Deploy prometheus
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
    helm repo update
    helm upgrade --install -n ${PROMETHEUS_NAMESPACE} ${HELM_PROM_RELEASE} prometheus-community/kube-prometheus-stack -f hack/load-tests/values.yaml --set grafana.adminPassword=myPwd

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

    if [ "$TEST_TARGET" = "self-monitor" ];
    then
        setup_selfmonitor
    fi
}

# shellcheck disable=SC2112
function setup_trace() {
    if "$MAX_PIPELINE"; then
        kubectl apply -f hack/load-tests/trace-max-pipeline.yaml
    fi
    # Deploy test setup
    cat hack/load-tests/trace-load-test-setup.yaml | sed -e  "s|OTEL_IMAGE|$OTEL_IMAGE|g" | kubectl apply -f -

    if "$BACKPRESSURE_TEST"; then
        kubectl apply -f hack/load-tests/trace-backpressure-config.yaml
    fi
}

function setup_metric() {
    if "$MAX_PIPELINE"; then
        kubectl apply -f hack/load-tests/metric-max-pipeline.yaml
    fi

    # Deploy test setup
   cat hack/load-tests/metric-load-test-setup.yaml | sed -e  "s|OTEL_IMAGE|$OTEL_IMAGE|g" | kubectl apply -f -

    if "$BACKPRESSURE_TEST"; then
        kubectl apply -f hack/load-tests/metric-backpressure-config.yaml
    fi
}

function setup_metric_agent() {
    # Deploy test setup
    cat hack/load-tests/metric-agent-test-setup.yaml | sed -e  "s|OTEL_IMAGE|$OTEL_IMAGE|g" | kubectl apply -f -

    if "$BACKPRESSURE_TEST"; then
        kubectl apply -f hack/load-tests/metric-agent-backpressure-config.yaml
    fi
}

function setup_fluentbit() {
    if "$MAX_PIPELINE"; then
        kubectl apply -f hack/load-tests/log-fluentbit-max-pipeline.yaml
    fi
    # Deploy test setup
    cat hack/load-tests/log-fluentbit-test-setup.yaml | sed -e  "s|OTEL_IMAGE|$OTEL_IMAGE|g" |  kubectl apply -f -

    if "$BACKPRESSURE_TEST"; then
        kubectl apply -f hack/load-tests/log-fluentbit-backpressure-config.yaml
    fi
}

# shellcheck disable=SC2112
function setup_selfmonitor() {
    # Deploy test setup
    cat hack/load-tests/self-monitor-test-setup.yaml | sed -e  "s|OTEL_IMAGE|$OTEL_IMAGE|g" | kubectl apply -f -
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

  if [ "$TEST_TARGET" = "self-monitor" ];
  then
      wait_for_selfmonitor_resources
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
function wait_for_selfmonitor_resources() {

    kubectl -n kyma-system rollout status deployment telemetry-trace-collector
    kubectl -n kyma-system rollout status deployment telemetry-metric-gateway
    kubectl -n kyma-system rollout status daemonset telemetry-metric-agent
    kubectl -n kyma-system rollout status daemonset telemetry-fluent-bit
    kubectl -n ${SELFMONITOR_NAMESPACE} rollout status deployment telemetry-receiver
    kubectl -n ${SELFMONITOR_NAMESPACE} rollout status deployment trace-load-generator
    kubectl -n ${SELFMONITOR_NAMESPACE} rollout status deployment metric-load-generator
    kubectl -n ${SELFMONITOR_NAMESPACE} rollout status deployment metric-agent-load-generator
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

    if [ "$TEST_TARGET" = "self-monitor" ]; then
        get_result_and_cleanup_selfmonitor
    fi

    helm delete -n ${PROMETHEUS_NAMESPACE} ${HELM_PROM_RELEASE}

    kubectl delete namespace $PROMETHEUS_NAMESPACE

}

# shellcheck disable=SC2112
function get_result_and_cleanup_trace() {
    RECEIVED=$(curl -fs --data-urlencode 'query=round(sum(rate(otelcol_receiver_accepted_spans{service="telemetry-trace-collector-metrics"}[20m])))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

    EXPORTED=$(curl -fs --data-urlencode 'query=round(sum(rate(otelcol_exporter_sent_spans{exporter=~"otlp/load-test.*"}[20m])))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

    QUEUE=$(curl -fs --data-urlencode 'query=avg(sum(otelcol_exporter_queue_size{service="telemetry-trace-collector-metrics"}))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

    MEMORY=$(curl -fs --data-urlencode 'query=round(sum(avg_over_time(container_memory_working_set_bytes{namespace="kyma-system", container="collector"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-trace-collector"}[20m])) by (pod) / 1024 / 1024)' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

    CPU=$(curl -fs --data-urlencode 'query=round(sum(avg_over_time(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{namespace="kyma-system"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-trace-collector"}[20m])) by (pod), 0.1)' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

    kill %1

    restarts=$(kubectl -n kyma-system get pod -l app.kubernetes.io/name=telemetry-trace-collector -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq | awk '{sum += $1} END {print sum}')

    if "$MAX_PIPELINE"; then
      kubectl delete -f hack/load-tests/trace-max-pipeline.yaml
    fi

    if "$BACKPRESSURE_TEST"; then
        kubectl delete -f hack/load-tests/trace-backpressure-config.yaml
    fi

    kubectl delete -f hack/load-tests/trace-load-test-setup.yaml

    echo "\nTrace Gateway got $restarts time restarted\n"

    echo "\nPrinting Test Results for $TEST_NAME $TEST_TARGET, Multi Pipeline $MAX_PIPELINE, Backpressure $BACKPRESSURE_TEST\n"
    printf "|%-10s|%-30s|%-30s|%-30s|%-30s|%-30s|\n" "" "Receiver Accepted Span/sec" "Exporter Exported Span/sec" "Exporter Queue Size" "Pod Memory Usage(MB)" "Pod CPU Usage"
    printf "|%-10s|%-30s|%-30s|%-30s|%-30s|%-30s|\n" "$TEST_NAME" "$RECEIVED" "$EXPORTED" "$QUEUE" "${MEMORY//$'\n'/,}" "${CPU//$'\n'/,}"
}

# shellcheck disable=SC2112
function get_result_and_cleanup_metric() {
    RECEIVED=$(curl -fs --data-urlencode 'query=round(sum(rate(otelcol_receiver_accepted_metric_points{service="telemetry-metric-gateway-metrics"}[20m])))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

    EXPORTED=$(curl -fs --data-urlencode 'query=round(sum(rate(otelcol_exporter_sent_metric_points{exporter=~"otlp/load-test.*"}[20m])))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

    QUEUE=$(curl -fs --data-urlencode 'query=avg(sum(otelcol_exporter_queue_size{service="telemetry-metric-gateway-metrics"}))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

    MEMORY=$(curl -fs --data-urlencode 'query=round(sum(avg_over_time(container_memory_working_set_bytes{namespace="kyma-system", container="collector"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-metric-gateway"}[20m])) by (pod) / 1024 / 1024)' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

    CPU=$(curl -fs --data-urlencode 'query=round(sum(avg_over_time(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{namespace="kyma-system"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-metric-gateway"}[20m])) by (pod), 0.1)' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')
    kill %1

    restarts=$(kubectl -n kyma-system get pod -l app.kubernetes.io/name=telemetry-metric-gateway -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq | awk '{sum += $1} END {print sum}')

    if "$MAX_PIPELINE"; then
      kubectl delete -f hack/load-tests/metric-max-pipeline.yaml
    fi

    if "$BACKPRESSURE_TEST"; then
        kubectl delete -f hack/load-tests/metric-backpressure-config.yaml
    fi

    kubectl delete -f hack/load-tests/metric-load-test-setup.yaml

    echo "\nMetric Gateway got $restarts time restarted\n"

    print_metric_result "$TEST_NAME" "$TEST_TARGET" "$MAX_PIPELINE" "$BACKPRESSURE_TEST" "$RECEIVED" "$EXPORTED" "$QUEUE" "$MEMORY" "$CPU"
}

# shellcheck disable=SC2112
function get_result_and_cleanup_metricagent() {
   RECEIVED=$(curl -fs --data-urlencode 'query=round(sum(rate(otelcol_receiver_accepted_metric_points{service="telemetry-metric-agent-metrics"}[20m])))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

   EXPORTED=$(curl -fs --data-urlencode 'query=round(sum(rate(otelcol_exporter_sent_metric_points{service=~"telemetry-metric-agent-metrics"}[20m])))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

   QUEUE=$(curl -fs --data-urlencode 'query=avg(sum(otelcol_exporter_queue_size{service="telemetry-metric-agent-metrics"}))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

   MEMORY=$(curl -fs --data-urlencode 'query=round(sum(avg_over_time(container_memory_working_set_bytes{namespace="kyma-system", container="collector"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-metric-agent"}[20m])) by (pod) / 1024 / 1024)' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

   CPU=$(curl -fs --data-urlencode 'query=round(sum(avg_over_time(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{namespace="kyma-system"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-metric-agent"}[20m])) by (pod), 0.1)' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

   kill %1

   restartsGateway=$(kubectl -n kyma-system get pod -l app.kubernetes.io/name=telemetry-metric-gateway -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq | awk '{sum += $1} END {print sum}')

   restartsAgent=$(kubectl -n kyma-system get pod -l app.kubernetes.io/name=telemetry-metric-agent -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq | awk '{sum += $1} END {print sum}')

   if "$BACKPRESSURE_TEST"; then
       kubectl delete -f hack/load-tests/metric-agent-backpressure-config.yaml
   fi

   kubectl delete -f hack/load-tests/metric-agent-test-setup.yaml

   echo "\nTest run for $TEST_DURATION seconds\n"
   echo "\nMetric Gateway got $restartsGateway time restarted\n"
   echo "\nMetric Agent got $restartsAgent time restarted\n"

   print_metric_result "$TEST_NAME" "$TEST_TARGET" "$MAX_PIPELINE" "$BACKPRESSURE_TEST" "$RECEIVED" "$EXPORTED" "$QUEUE" "$MEMORY" "$CPU"
}

# shellcheck disable=SC2112
function get_result_and_cleanup_fluentbit() {
   RECEIVED=$(curl -fs --data-urlencode 'query=round((sum(rate(fluentbit_input_bytes_total{service="telemetry-fluent-bit-metrics", name=~"load-test-.*"}[20m])) / 1024))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

   EXPORTED=$(curl -fs --data-urlencode 'query=round((sum(rate(fluentbit_output_proc_bytes_total{service="telemetry-fluent-bit-metrics", name=~"load-test-.*"}[20m])) / 1024))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

   QUEUE=$(curl -fs --data-urlencode 'query=round(sum(avg_over_time(telemetry_fsbuffer_usage_bytes{service="telemetry-fluent-bit-exporter-metrics"}[20m])) / 1024)' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

   MEMORY=$(curl -fs --data-urlencode 'query=round(sum(avg_over_time(container_memory_working_set_bytes{namespace="kyma-system", container="fluent-bit"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-fluent-bit"}[20m])) by (pod) / 1024 / 1024)' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

   CPU=$(curl -fs --data-urlencode 'query=round(sum(avg_over_time(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{namespace="kyma-system"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-fluent-bit"}[20m])) by (pod), 0.1)' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

   kill %1

   restarts=$(kubectl -n kyma-system get pod -l app.kubernetes.io/name=fluent-bit -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq | awk '{sum += $1} END {print sum}')

   if "$MAX_PIPELINE"; then
       kubectl delete -f hack/load-tests/log-fluentbit-max-pipeline.yaml
   fi

   if "$BACKPRESSURE_TEST"; then
       kubectl delete -f hack/load-tests/log-fluentbit-backpressure-config.yaml
   fi

   kubectl delete -f hack/load-tests/log-fluentbit-test-setup.yaml

   echo "\nLogPipeline Pods got $restarts time restarted\n"

   echo "\nPrinting Test Results for $TEST_NAME $TEST_TARGET, Multi Pipeline $MAX_PIPELINE, Backpressure $BACKPRESSURE_TEST\n"
   printf "|%-10s|%-35s|%-35s|%-30s|%-30s|%-30s|\n" "" "Input Bytes Processing Rate/sec" "Output Bytes Processing Rate/sec" "Filesystem Buffer Usage" "Pod Memory Usage(MB)" "Pod CPU Usage"
   printf "|%-10s|%-35s|%-35s|%-30s|%-30s|%-30s|\n" "$TEST_NAME" "$RECEIVED" "$EXPORTED" "$QUEUE" "${MEMORY//$'\n'/,}" "${CPU//$'\n'/,}"
}

# shellcheck disable=SC2112
function get_result_and_cleanup_selfmonitor() {
   # ingestion rate per second for scrape samples https://valyala.medium.com/prometheus-storage-technical-terms-for-humans-4ab4de6c3d48
   SCRAPESAMPLES=$(curl -fs --data-urlencode 'query=round(sum(sum_over_time(scrape_samples_scraped{service="telemetry-self-monitor-metrics"}[20m]) / 1200))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

   SERIESCREATED=$(curl -fs --data-urlencode 'query=round(sum(max_over_time(prometheus_tsdb_head_series{service="telemetry-self-monitor-metrics"}[20m])))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

   HEADSTORAGESIZE=$(curl -fs --data-urlencode 'query=round(sum(max_over_time(prometheus_tsdb_head_chunks_storage_size_bytes{service="telemetry-self-monitor-metrics"}[20m])))' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

   MEMORY=$(curl -fs --data-urlencode 'query=round(sum(avg_over_time(container_memory_working_set_bytes{namespace="kyma-system", container="self-monitor"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-self-monitor"}[20m])) by (pod) / 1024 / 1024)' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

   CPU=$(curl -fs --data-urlencode 'query=round(sum(avg_over_time(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{namespace="kyma-system"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-self-monitor"}[20m])) by (pod), 0.1)' localhost:9090/api/v1/query | jq -r '.data.result[] | .value[1]')

   kill %1

   restarts=$(kubectl -n kyma-system get pod -l app.kubernetes.io/name=telemetry-self-monitor -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq | awk '{sum += $1} END {print sum}')

   kubectl delete -f hack/load-tests/self-monitor-test-setup.yaml

   echo "\Self Monitor Pods got $restarts time restarted\n"

   echo "\nPrinting Test Results for $TEST_NAME $TEST_TARGET\n"
   printf "|%-10s|%-30s|%-30s|%-30s|%-30s|%-30s|\n" "" "Scrape Samples/sec" "Total Series Created" "Head Chunk Storage Size/bytes" "Pod Memory Usage(MB)" "Pod CPU Usage"
   printf "|%-10s|%-35s|%-35s|%-30s|%-30s|%-30s|\n" "$TEST_NAME" "$SCRAPESAMPLES" "$SERIESCREATED" "$HEADSTORAGESIZE" "${MEMORY//$'\n'/,}" "${CPU//$'\n'/,}"
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

