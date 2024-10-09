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
SELF_MONITOR_NAMESPACE="self-monitor-load-test"
MAX_PIPELINE="false"
BACKPRESSURE_TEST="false"
TEST_TARGET="traces"
TEST_NAME="No Name"
TEST_DURATION=1200
OTEL_IMAGE="europe-docker.pkg.dev/kyma-project/prod/kyma-otel-collector:0.111.0-main"
LOG_SIZE=2000
LOG_RATE=1000
PROMAPI="http://localhost:9090/api/v1/query"

function help() {
    echo "Usage: $0 -m <max_pipeline> -b <backpressure_test> -n <test_name> -t <test_target> -d <test_duration> -r <log_rate> -s <log_size>"
    echo "Options:"
    echo "  -m <max_pipeline>         Run the test with max pipeline configuration"
    echo "  -b <backpressure_test>    Run the test with backpressure configuration"
    echo "  -n <test_name>            Name of the test"
    echo "  -t <test_target>          Target of the test (traces, metrics, metricagent, logs-fluentbit, logs-otel, self-monitor)"
    echo "  -d <test_duration>        Duration of the test in seconds"
    echo "  -r <log_rate>             Rate of log generation in logs-otel test"
    echo "  -s <log_size>             Size of log in logs-otel test"
    echo "  -o <config>               Use alternative overlay (batch) for logs-otel test"
    exit 1
}

while getopts m:b:n:t:d:r:s:o: flag; do
    case "$flag" in
        m) MAX_PIPELINE="${OPTARG}" ;;
        b) BACKPRESSURE_TEST="${OPTARG}" ;;
        n) TEST_NAME="${OPTARG}" ;;
        t) TEST_TARGET="${OPTARG}" ;;
        d) TEST_DURATION=${OPTARG} ;;
        r) LOG_RATE=${OPTARG} ;;
        s) LOG_SIZE=${OPTARG} ;;
        o) OVERLAY=${OPTARG} ;;
        *) help ;;
    esac
done

image_clean=$(basename "$OTEL_IMAGE" | tr ":" "." )
mkdir -p tests
NAME="$TEST_TARGET"
[ "$MAX_PIPELINE" == "true" ] && NAME="$NAME-MultiPipeline"
[ "$BACKPRESSURE_TEST" == "true" ] && NAME="$NAME-BackPressure"

RESULTS_FILE="tests/$(echo "$NAME-$image_clean" | tr -cd '[[:alnum:]]._-').json"

function print_config() {
    echo "Test configuration:"
    echo "  Test Name: $TEST_NAME"
    echo "  Test Target: $TEST_TARGET"
    echo "  Test Duration: $TEST_DURATION"
    echo "  OTEL Image: $OTEL_IMAGE"
    echo "  Max Pipeline: $MAX_PIPELINE"
    echo "  Backpressure Test: $BACKPRESSURE_TEST"
    echo "  Log Rate: $LOG_RATE"
    echo "  Log Size: $LOG_SIZE"
    echo "  Overlay: $OVERLAY"
    echo "  Results File: $RESULTS_FILE"
}

function setup() {
    kubectl create namespace "$PROMETHEUS_NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

    [ "$TEST_TARGET" != "logs-otel" ] && kubectl label namespace kyma-system istio-injection=enabled --overwrite

    # Deploy prometheus
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
    helm repo update
    helm upgrade --install -n "$PROMETHEUS_NAMESPACE" "$HELM_PROM_RELEASE" prometheus-community/kube-prometheus-stack -f hack/load-tests/values.yaml --set grafana.adminPassword=myPwd

    case "$TEST_TARGET" in
        traces) setup_trace ;;
        metrics) setup_metric ;;
        metricagent) setup_metric_agent ;;
        logs-fluentbit) setup_fluentbit ;;
        logs-otel) setup_logs_otel ;;
        self-monitor) setup_selfmonitor ;;
    esac
}

function setup_trace() {
    if [[ "$MAX_PIPELINE" == "true" ]]; then
      kubectl apply -f hack/load-tests/trace-max-pipeline.yaml
    fi

    # Deploy test setup
    sed -e "s|OTEL_IMAGE|$OTEL_IMAGE|g" hack/load-tests/trace-load-test-setup.yaml | kubectl apply -f -

    if [[ "$BACKPRESSURE_TEST" == "true" ]]; then
      kubectl apply -f hack/load-tests/trace-backpressure-config.yaml
    fi
}

function setup_metric() {
    if [[ "$MAX_PIPELINE" == "true" ]]; then
        kubectl apply -f hack/load-tests/metric-max-pipeline.yaml
    fi

    # Deploy test setup
    sed -e "s|OTEL_IMAGE|$OTEL_IMAGE|g" hack/load-tests/metric-load-test-setup.yaml | kubectl apply -f -

    if [[ "$BACKPRESSURE_TEST" == "true" ]]; then
        kubectl apply -f hack/load-tests/metric-backpressure-config.yaml
    fi
}

function setup_metric_agent() {
    # Deploy test setup
    sed -e "s|OTEL_IMAGE|$OTEL_IMAGE|g" hack/load-tests/metric-agent-test-setup.yaml | kubectl apply -f -

    if [[ "$BACKPRESSURE_TEST" == "true" ]]; then
      kubectl apply -f hack/load-tests/metric-agent-backpressure-config.yaml
    fi
}

function setup_fluentbit() {
    if [[ "$MAX_PIPELINE" == "true" ]]; then
      kubectl apply -f hack/load-tests/log-fluentbit-max-pipeline.yaml
    fi

    # Deploy test setup
    sed -e "s|OTEL_IMAGE|$OTEL_IMAGE|g" hack/load-tests/log-fluentbit-test-setup.yaml | kubectl apply -f -

    if [[ "$BACKPRESSURE_TEST" == "true" ]]; then
      kubectl apply -f hack/load-tests/log-fluentbit-backpressure-config.yaml
    fi
}

function setup_logs_otel() {
    cat > hack/load-tests/otel-logs/base/base.env <<EOF
LOG_RATE=$LOG_RATE
LOG_CONTENT=$(for i in $(seq $LOG_SIZE); do echo -n X; done)
EOF
    if [[ "$OVERLAY" == "batch" ]]; then
        kubectl apply -k hack/load-tests/otel-logs/batch
    else
        kubectl apply -k hack/load-tests/otel-logs/base
    fi
}

function setup_selfmonitor() {
    # Deploy test setup
    sed -e "s|OTEL_IMAGE|$OTEL_IMAGE|g" hack/load-tests/self-monitor-test-setup.yaml | kubectl apply -f -
}

function wait_for_resources() {
  kubectl -n "$PROMETHEUS_NAMESPACE" rollout status statefulset prometheus-prometheus-kube-prometheus-prometheus

  case "$TEST_TARGET" in
    traces) wait_for_trace_resources ;;
    metrics) wait_for_metric_resources ;;
    metricagent) wait_for_metric_agent_resources ;;
    logs-fluentbit) wait_for_fluentbit_resources ;;
    logs-otel) wait_for_otel_log_resources ;;
    self-monitor) wait_for_selfmonitor_resources ;;
  esac

  echo -e "\nRunning Tests\n"
}

function wait_for_trace_resources() {
    kubectl -n kyma-system rollout status deployment telemetry-trace-gateway
    kubectl -n ${TRACE_NAMESPACE} rollout status deployment trace-load-generator
    kubectl -n ${TRACE_NAMESPACE} rollout status deployment trace-receiver
}

function wait_for_metric_resources() {
    kubectl -n kyma-system rollout status deployment telemetry-metric-gateway
    kubectl -n ${METRIC_NAMESPACE} rollout status deployment metric-load-generator
    kubectl -n ${METRIC_NAMESPACE} rollout status deployment metric-receiver
}

function wait_for_metric_agent_resources() {
    kubectl -n kyma-system rollout status deployment telemetry-metric-gateway
    kubectl -n kyma-system rollout status daemonset telemetry-metric-agent
    kubectl -n ${METRIC_NAMESPACE} rollout status deployment metric-agent-load-generator
    kubectl -n ${METRIC_NAMESPACE} rollout status deployment metric-receiver
}

function wait_for_fluentbit_resources() {
    kubectl -n ${LOG_NAMESPACE} rollout status deployment log-receiver
    kubectl -n kyma-system rollout status daemonset telemetry-fluent-bit
    kubectl -n ${LOG_NAMESPACE} rollout status deployment log-load-generator
}

function wait_for_otel_log_resources() {
    kubectl -n ${LOG_NAMESPACE} rollout status deployment log-receiver
    kubectl -n ${LOG_NAMESPACE} rollout status deployment log-gateway
    kubectl -n ${LOG_NAMESPACE} rollout status deployment log-load-generator
}

function wait_for_selfmonitor_resources() {
    kubectl -n kyma-system rollout status deployment telemetry-trace-gateway
    kubectl -n kyma-system rollout status deployment telemetry-metric-gateway
    kubectl -n kyma-system rollout status daemonset telemetry-metric-agent
    kubectl -n kyma-system rollout status daemonset telemetry-fluent-bit
    kubectl -n ${SELF_MONITOR_NAMESPACE} rollout status deployment telemetry-receiver
    kubectl -n ${SELF_MONITOR_NAMESPACE} rollout status deployment trace-load-generator
    kubectl -n ${SELF_MONITOR_NAMESPACE} rollout status deployment metric-load-generator
    kubectl -n ${SELF_MONITOR_NAMESPACE} rollout status deployment metric-agent-load-generator
}

function cleanup() {
    kubectl -n "$PROMETHEUS_NAMESPACE" port-forward "$(kubectl -n "$PROMETHEUS_NAMESPACE" get service -l app=kube-prometheus-stack-prometheus -oname)" 9090 &
    sleep 3

    echo -e "Test results collecting"
    case "$TEST_TARGET" in
        traces) get_result_and_cleanup_trace ;;
        metrics) get_result_and_cleanup_metric ;;
        metricagent) get_result_and_cleanup_metricagent ;;
        logs-fluentbit) get_result_and_cleanup_fluentbit ;;
        logs-otel) get_result_and_cleanup_log_otel ;;
        self-monitor) get_result_and_cleanup_selfmonitor ;;
    esac

    # export all variables starting with RESULT_
    export ${!RESULT_*}
    json=$(jq -n 'env | with_entries(select(.key | startswith("RESULT_")))| with_entries(.key |= sub("^RESULT_"; ""))')
    nodes=$(kubectl get nodes -ojson | jq '[.items[] | .metadata.labels["beta.kubernetes.io/instance-type"]]')

    template=$(
    cat <<EOF
    {
        "test_name": "$TEST_NAME",
        "test_target": "$TEST_TARGET",
        "max_pipeline": "$MAX_PIPELINE",
        "nodes": $nodes,
        "backpressure_test": "$BACKPRESSURE_TEST",
        "results": $json,
        "test_duration": "$TEST_DURATION",
        "overlay": "$OVERLAY"
    }
EOF
    )

    echo "$template" | jq . | tee -a "$RESULTS_FILE"

    helm delete -n "$PROMETHEUS_NAMESPACE" "$HELM_PROM_RELEASE"
    kubectl delete namespace "$PROMETHEUS_NAMESPACE"

}

function get_result_and_cleanup_trace() {
  RESULT_TYPE="span"
  QUERY_RECEIVED='query=round(sum(rate(otelcol_receiver_accepted_spans{service="telemetry-trace-gateway-metrics"}[20m])))'
  QUERY_EXPORTED='query=round(sum(rate(otelcol_exporter_sent_spans{exporter=~"otlp/load-test.*"}[20m])))'
  QUERY_QUEUE='query=avg(sum(otelcol_exporter_queue_size{service="telemetry-trace-gateway-metrics"}))'
  QUERY_MEMORY='query=round(sum(avg_over_time(container_memory_working_set_bytes{namespace="kyma-system", container="collector"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-trace-gateway"}[20m])) by (pod) / 1024 / 1024)'
  QUERY_CPU='query=round(sum(avg_over_time(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{namespace="kyma-system"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-trace-gateway"}[20m])) by (pod), 0.1)'

  RESULT_RECEIVED=$(curl -fs --data-urlencode "$QUERY_RECEIVED" $PROMAPI | jq -r '.data.result[] | .value[1]')
  RESULT_EXPORTED=$(curl -fs --data-urlencode "$QUERY_EXPORTED" $PROMAPI | jq -r '.data.result[] | .value[1]')
  RESULT_QUEUE=$(curl -fs --data-urlencode "$QUERY_QUEUE" $PROMAPI | jq -r '.data.result[] | .value[1]')
  RESULT_MEMORY=$(curl -fs --data-urlencode "$QUERY_MEMORY" $PROMAPI | jq -r '.data.result[] | .value[1]' | tr '\n' ',')
  RESULT_CPU=$(curl -fs --data-urlencode "$QUERY_CPU" $PROMAPI | jq -r '.data.result[] | .value[1]' | tr '\n' ',')
  RESULT_RESTARTS_COLLECTOR=$(kubectl -n kyma-system get pod -l app.kubernetes.io/name=telemetry-trace-gateway -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq -s 'add')

  kill %1

  if [[ "$MAX_PIPELINE" == "true" ]]; then
    kubectl delete -f hack/load-tests/trace-max-pipeline.yaml
  fi
  if [[ "$BACKPRESSURE_TEST" == "true" ]]; then
    kubectl delete -f hack/load-tests/trace-backpressure-config.yaml
  fi

  kubectl delete -f hack/load-tests/trace-load-test-setup.yaml
}

function get_result_and_cleanup_metric() {
    RESULT_TYPE="metric"
    QUERY_RECEIVED='query=round(sum(rate(otelcol_receiver_accepted_metric_points{service="telemetry-metric-gateway-metrics"}[20m])))'
    QUERY_EXPORTED='query=round(sum(rate(otelcol_exporter_sent_metric_points{exporter=~"otlp/load-test.*"}[20m])))'
    QUERY_QUEUE='query=avg(sum(otelcol_exporter_queue_size{service="telemetry-metric-gateway-metrics"}))'
    QUERY_MEMORY='query=round(sum(avg_over_time(container_memory_working_set_bytes{namespace="kyma-system", container="collector"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-metric-gateway"}[20m])) by (pod) / 1024 / 1024)'
    QUERY_CPU='query=round(sum(avg_over_time(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{namespace="kyma-system"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-metric-gateway"}[20m])) by (pod), 0.1)'

    RESULT_RECEIVED=$(curl -fs --data-urlencode "$QUERY_RECEIVED" $PROMAPI | jq -r '.data.result[] | .value[1]')
    RESULT_EXPORTED=$(curl -fs --data-urlencode "$QUERY_EXPORTED" $PROMAPI | jq -r '.data.result[] | .value[1]')
    RESULT_QUEUE=$(curl -fs --data-urlencode "$QUERY_QUEUE" $PROMAPI | jq -r '.data.result[] | .value[1]')
    RESULT_MEMORY=$(curl -fs --data-urlencode "$QUERY_MEMORY" $PROMAPI | jq -r '.data.result[] | .value[1]' | tr '\n' ',')
    RESULT_CPU=$(curl -fs --data-urlencode "$QUERY_CPU" $PROMAPI | jq -r '.data.result[] | .value[1]' | tr '\n' ',')
    RESULT_RESTARTS_GATEWAY=$(kubectl -n kyma-system get pod -l app.kubernetes.io/name=telemetry-metric-gateway -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq -s 'add')

    kill %1

    if [[ "$MAX_PIPELINE" == "true" ]]; then
      kubectl delete -f hack/load-tests/metric-max-pipeline.yaml
    fi

    if [[ "$BACKPRESSURE_TEST" == "true" ]]; then
      kubectl delete -f hack/load-tests/metric-backpressure-config.yaml
    fi

    kubectl delete -f hack/load-tests/metric-load-test-setup.yaml
}

function get_result_and_cleanup_metricagent() {
    RESULT_TYPE="metric"
    QUERY_RECEIVED='query=round(sum(rate(otelcol_receiver_accepted_metric_points{service="telemetry-metric-agent-metrics"}[20m])))'
    QUERY_EXPORTED='query=round(sum(rate(otelcol_exporter_sent_metric_points{service=~"telemetry-metric-agent-metrics"}[20m])))'
    QUERY_QUEUE='query=avg(sum(otelcol_exporter_queue_size{service="telemetry-metric-agent-metrics"}))'
    QUERY_MEMORY='query=round(sum(avg_over_time(container_memory_working_set_bytes{namespace="kyma-system", container="collector"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-metric-agent"}[20m])) by (pod) / 1024 / 1024)'
    QUERY_CPU='query=round(sum(avg_over_time(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{namespace="kyma-system"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-metric-agent"}[20m])) by (pod), 0.1)'

    RESULT_RECEIVED=$(curl -fs --data-urlencode "$QUERY_RECEIVED" $PROMAPI | jq -r '.data.result[] | .value[1]')
    RESULT_EXPORTED=$(curl -fs --data-urlencode "$QUERY_EXPORTED" $PROMAPI | jq -r '.data.result[] | .value[1]')
    RESULT_QUEUE=$(curl -fs --data-urlencode "$QUERY_QUEUE" $PROMAPI | jq -r '.data.result[] | .value[1]')
    RESULT_MEMORY=$(curl -fs --data-urlencode "$QUERY_MEMORY" $PROMAPI | jq -r '.data.result[] | .value[1]' | tr '\n' ',')
    RESULT_CPU=$(curl -fs --data-urlencode "$QUERY_CPU" $PROMAPI | jq -r '.data.result[] | .value[1]' | tr '\n' ',')
    RESULT_RESTARTS_GATEWAY=$(kubectl -n kyma-system get pod -l app.kubernetes.io/name=telemetry-metric-gateway -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq -s 'add')
    RESULT_RESTARTS_AGENT=$(kubectl -n kyma-system get pod -l app.kubernetes.io/name=telemetry-metric-agent -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq -s 'add')

    kill %1
    if [[ "$BACKPRESSURE_TEST" == "true" ]]; then
      kubectl delete -f hack/load-tests/metric-agent-backpressure-config.yaml
    fi
    kubectl delete -f hack/load-tests/metric-agent-test-setup.yaml
}

function get_result_and_cleanup_log_otel() {
  RESULT_TYPE="log"
  QUERY_RECEIVED='query=round(sum(rate(otelcol_receiver_accepted_log_records{service="log-gateway-metrics"}[20m])))'
  QUERY_EXPORTED='query=round(sum(rate(otelcol_exporter_sent_log_records{service=~"log-gateway-metrics"}[20m])))'
  QUERY_QUEUE='query=avg(sum(otelcol_exporter_queue_size{service="log-gateway-metrics"}))'
  QUERY_MEMORY='query=round(sum(avg_over_time(container_memory_working_set_bytes{namespace="log-load-test", container="collector"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="log-load-test", workload="log-gateway"}[20m])) by (pod) / 1024 / 1024)'
  QUERY_CPU='query=round(sum(avg_over_time(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{namespace="log-load-test"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="log-load-test", workload="log-gateway"}[20m])) by (pod), 0.1)'

  RESULT_RECEIVED=$(curl -fs --data-urlencode "$QUERY_RECEIVED" $PROMAPI | jq -r '.data.result[] | .value[1]')
  RESULT_EXPORTED=$(curl -fs --data-urlencode "$QUERY_EXPORTED" $PROMAPI | jq -r '.data.result[] | .value[1]')
  RESULT_QUEUE=$(curl -fs --data-urlencode "$QUERY_QUEUE" $PROMAPI | jq -r '.data.result[] | .value[1]')
  RESULT_MEMORY=$(curl -fs --data-urlencode "$QUERY_MEMORY" $PROMAPI | jq -r '.data.result[] | .value[1]' | tr '\n' ',')
  RESULT_CPU=$(curl -fs --data-urlencode "$QUERY_CPU" $PROMAPI | jq -r '.data.result[] | .value[1]' | tr '\n' ',')
  RESULT_RESTARTS_GATEWAY=$(kubectl -n log-load-test get pod -l app.kubernetes.io/name=log-gateway -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq -s 'add')
  RESULT_RESTARTS_GENERATOR=$(kubectl -n log-load-test get pod -l app.kubernetes.io/name=log-load-generator -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq -s 'add')

  kill %1

  if [[ "$OVERLAY" == "batch" ]]; then
    kubectl delete -k hack/load-tests/otel-logs/batch
  else
    kubectl delete -k hack/load-tests/otel-logs/base
  fi
}

function get_result_and_cleanup_fluentbit() {
  RESULT_TYPE="log"
  PROMAPI="http://localhost:9090/api/v1/query"
  QUERY_RECEIVED='query=round(sum(rate(fluentbit_input_bytes_total{service="telemetry-fluent-bit-metrics", name=~"load-test-.*"}[20m])) / 1024)'
  QUERY_EXPORTED='query=round(sum(rate(fluentbit_output_proc_bytes_total{service="telemetry-fluent-bit-metrics", name=~"load-test-.*"}[20m])) / 1024)'
  QUERY_QUEUE='query=round(sum(avg_over_time(telemetry_fsbuffer_usage_bytes{service="telemetry-fluent-bit-exporter-metrics"}[20m])) / 1024)'
  QUERY_MEMORY='query=round(sum(avg_over_time(container_memory_working_set_bytes{namespace="kyma-system", container="fluent-bit"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-fluent-bit"}[20m])) by (pod) / 1024 / 1024)'
  QUERY_CPU='query=round(sum(avg_over_time(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{namespace="kyma-system"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-fluent-bit"}[20m])) by (pod), 0.1)'

  RESULT_RECEIVED=$(curl -fs --data-urlencode "$QUERY_RECEIVED" $PROMAPI | jq -r '.data.result[] | .value[1]')
  RESULT_EXPORTED=$(curl -fs --data-urlencode "$QUERY_EXPORTED" $PROMAPI | jq -r '.data.result[] | .value[1]')
  RESULT_QUEUE=$(curl -fs --data-urlencode "$QUERY_QUEUE" $PROMAPI | jq -r '.data.result[] | .value[1]')
  RESULT_MEMORY=$(curl -fs --data-urlencode "$QUERY_MEMORY" $PROMAPI | jq -r '.data.result[] | .value[1]'  | tr '\n' ',')
  RESULT_CPU=$(curl -fs --data-urlencode "$QUERY_CPU" $PROMAPI | jq -r '.data.result[] | .value[1]' | tr '\n' ',')
  RESULT_RESTARTS_FLUENTBIT=$(kubectl -n kyma-system get pod -l app.kubernetes.io/name=fluent-bit -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq -s 'add')

  kill %1

  if [[ "$MAX_PIPELINE" == "true" ]]; then
    kubectl delete -f hack/load-tests/log-fluentbit-max-pipeline.yaml
  fi
  if [[ "$BACKPRESSURE_TEST" == "true" ]]; then
    kubectl delete -f hack/load-tests/log-fluentbit-backpressure-config.yaml
  fi

  kubectl delete -f hack/load-tests/log-fluentbit-test-setup.yaml
}

function get_result_and_cleanup_selfmonitor() {
    PROMAPI="http://localhost:9090/api/v1/query"
    QUERY_SCRAPESAMPLES='query=round(sum(sum_over_time(scrape_samples_scraped{service="telemetry-self-monitor-metrics"}[20m]) / 1200))'
    QUERY_SERIESCREATED='query=round(sum(max_over_time(prometheus_tsdb_head_series{service="telemetry-self-monitor-metrics"}[20m])))'
    QUERY_WALSTORAGESIZE='query=round(sum(max_over_time(prometheus_tsdb_wal_storage_size_bytes{service="telemetry-self-monitor-metrics"}[20m])))'
    QUERY_HEADSTORAGESIZE='query=round(sum(max_over_time(prometheus_tsdb_head_chunks_storage_size_bytes{service="telemetry-self-monitor-metrics"}[20m])))'
    QUERY_MEMORY='query=round(sum(avg_over_time(container_memory_working_set_bytes{namespace="kyma-system", container="self-monitor"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-self-monitor"}[20m])) by (pod) / 1024 / 1024)'
    QUERY_CPU='query=round(sum(avg_over_time(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{namespace="kyma-system"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-self-monitor"}[20m])) by (pod), 0.1)'

    RESULT_SCRAPESAMPLES=$(curl -fs --data-urlencode "$QUERY_SCRAPESAMPLES" $PROMAPI | jq -r '.data.result[0].value[1]')
    RESULT_SERIESCREATED=$(curl -fs --data-urlencode "$QUERY_SERIESCREATED" $PROMAPI | jq -r '.data.result[0].value[1]')
    RESULT_WALSTORAGESIZE=$(curl -fs --data-urlencode "$QUERY_WALSTORAGESIZE" $PROMAPI | jq -r '.data.result[0].value[1]')
    RESULT_HEADSTORAGESIZE=$(curl -fs --data-urlencode "$QUERY_HEADSTORAGESIZE" $PROMAPI | jq -r '.data.result[0].value[1]')
    RESULT_MEMORY=$(curl -fs --data-urlencode "$QUERY_MEMORY" $PROMAPI | jq -r '.data.result[0].value[1]')
    RESULT_CPU=$(curl -fs --data-urlencode "$QUERY_CPU" $PROMAPI | jq -r '.data.result[0].value[1]')
    RESULT_RESTARTS_SELFMONITOR=$(kubectl -n kyma-system get pod -l app.kubernetes.io/name=telemetry-self-monitor -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq -s 'add')

    kill %1
    kubectl delete -f hack/load-tests/self-monitor-test-setup.yaml
}



# cleanup on exit. cleanup also collects the results and writes them to a file
trap cleanup EXIT
print_config
setup
wait_for_resources
# wait for the test to finish
sleep $TEST_DURATION
