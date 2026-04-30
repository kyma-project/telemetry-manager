#!/usr/bin/env bash

# standard bash error handling
set -o nounset  # treat unset variables as an error and exit immediately.
set -o errexit  # exit immediately when a command fails.
set -E          # ensure ERR trap is inherited by functions, command substitutions, and subshells.
set -o pipefail # prevents errors in a pipeline from being masked

source .env

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
OTEL_IMAGE=$ENV_OTEL_COLLECTOR_IMAGE
TELEMETRY_GEN_IMAGE=$ENV_TEST_TELEMETRYGEN_IMAGE
LOG_SIZE=2000
LOG_RATE=1000
PROMAPI=""
DOMAIN=""
OVERLAY=""

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
    echo "  Telemetry Gen Image: $TELEMETRY_GEN_IMAGE"
    echo "  Max Pipeline: $MAX_PIPELINE"
    echo "  Backpressure Test: $BACKPRESSURE_TEST"
    echo "  Log Rate: $LOG_RATE"
    echo "  Log Size: $LOG_SIZE"
    echo "  Overlay: $OVERLAY"
    echo "  Results File: $RESULTS_FILE"
}

function setup() {
    echo -e "Deploying prometheus stack"
    kubectl create namespace "$PROMETHEUS_NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

    [ "$TEST_TARGET" != "logs-otel" ] && kubectl label namespace kyma-system istio-injection=enabled --overwrite

    # Deploy prometheus
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
    helm repo update
    helm upgrade --install -n "$PROMETHEUS_NAMESPACE" "$HELM_PROM_RELEASE" prometheus-community/kube-prometheus-stack -f hack/load-tests/values.yaml --set grafana.adminPassword=myPwd

    DOMAIN=$(kubectl -n kube-system get cm shoot-info --ignore-not-found=true -ojsonpath={.data.domain})

    if [[ -n "$DOMAIN" ]]; then
      kubectl apply -f https://github.com/kyma-project/api-gateway/releases/latest/download/api-gateway-manager.yaml
      kubectl apply -f https://github.com/kyma-project/api-gateway/releases/latest/download/apigateway-default-cr.yaml
      sed -e "s|DOMAIN|$DOMAIN|g" hack/load-tests/prometheus-setup.yaml | sed -e "s|PROMETHEUS_NAMESPACE|$PROMETHEUS_NAMESPACE|g" | kubectl apply -f -
      PROMAPI="https://prometheus.$DOMAIN/api/v1/query"
    else
      PROMAPI="http://localhost:8080/api/v1/query"
    fi
    echo -e "Prometheus stack deployed accessable at: $PROMAPI"

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
    echo -e "Deploying trace test setup"
    if [[ "$MAX_PIPELINE" == "true" ]]; then
      kubectl apply -f hack/load-tests/trace-max-pipeline.yaml
    fi

    # Deploy test setup
    sed -e "s|OTEL_IMAGE|$OTEL_IMAGE|g" hack/load-tests/trace-load-test-setup.yaml | sed -e "s|TELEMETRY_GEN_IMAGE|$TELEMETRY_GEN_IMAGE|g" | kubectl apply -f -

    if [[ "$BACKPRESSURE_TEST" == "true" ]]; then
      kubectl apply -f hack/load-tests/trace-backpressure-config.yaml
    fi
}

function setup_metric() {
    echo -e "Deploying metric test setup"
    if [[ "$MAX_PIPELINE" == "true" ]]; then
        kubectl apply -f hack/load-tests/metric-max-pipeline.yaml
    fi

    sed -e "s|OTEL_IMAGE|$OTEL_IMAGE|g" hack/load-tests/metric-load-test-setup.yaml | sed -e "s|TELEMETRY_GEN_IMAGE|$TELEMETRY_GEN_IMAGE|g" | kubectl apply -f -

    if [[ "$BACKPRESSURE_TEST" == "true" ]]; then
        kubectl apply -f hack/load-tests/metric-backpressure-config.yaml
    fi
}

function setup_metric_agent() {
    echo -e "Deploying Metric Agent test setup"
    sed -e "s|OTEL_IMAGE|$OTEL_IMAGE|g" hack/load-tests/metric-agent-test-setup.yaml | sed -e "s|TELEMETRY_GEN_IMAGE|$TELEMETRY_GEN_IMAGE|g" | kubectl apply -f -

    if [[ "$BACKPRESSURE_TEST" == "true" ]]; then
      kubectl apply -f hack/load-tests/metric-agent-backpressure-config.yaml
    fi
}

function setup_fluentbit() {
    echo -e "Deploying fluentbit test setup"
    if [[ "$MAX_PIPELINE" == "true" ]]; then
      kubectl apply -f hack/load-tests/log-fluentbit-max-pipeline.yaml
    fi

    # Deploy test setup
    sed -e "s|OTEL_IMAGE|$OTEL_IMAGE|g" hack/load-tests/log-fluentbit-test-setup.yaml | sed -e "s|TELEMETRY_GEN_IMAGE|$TELEMETRY_GEN_IMAGE|g" | kubectl apply -f -

    if [[ "$BACKPRESSURE_TEST" == "true" ]]; then
      kubectl apply -f hack/load-tests/log-fluentbit-backpressure-config.yaml
    fi
}

function setup_logs_otel() {
    echo -e "Deploying otel log test setup"
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
    echo -e "Deploying self-monitor test setup"
    sed -e "s|OTEL_IMAGE|$OTEL_IMAGE|g" hack/load-tests/self-monitor-test-setup.yaml | sed -e "s|TELEMETRY_GEN_IMAGE|$TELEMETRY_GEN_IMAGE|g" | kubectl apply -f -
}

function wait_for_resources() {
  wait_for_prometheus_resources

  case "$TEST_TARGET" in
    traces) wait_for_trace_resources ;;
    metrics) wait_for_metric_resources ;;
    metricagent) wait_for_metric_agent_resources ;;
    logs-fluentbit) wait_for_fluentbit_resources ;;
    logs-otel) wait_for_otel_log_resources ;;
    self-monitor) wait_for_selfmonitor_resources ;;
  esac

  echo -e "\nAll resources are ready\n"
}

function wait_for_rollout() {
    echo -e "\nWaiting for rollout of $2 in namespace $1"
    local namespace=$1
    local resource=$2
    local label=${3:-$(echo "$resource" | sed 's|.*/||')}
    if ! kubectl -n "$namespace" rollout status "$resource" --timeout=90s; then
        echo -e "\nERROR: Rollout timed out for $resource in namespace $namespace"
        echo -e "\nPod status:"
        kubectl -n "$namespace" get pods -o wide
        echo -e "\nPod descriptions:"
        kubectl -n "$namespace" describe pods -l "app.kubernetes.io/name=$label"
        echo -e "\nPod logs:"
        kubectl -n "$namespace" logs -l "app.kubernetes.io/name=$label" --tail=50 --all-containers
        exit 1
    fi
}

function wait_for_prometheus_resources() {
    wait_for_rollout "$PROMETHEUS_NAMESPACE" "statefulset/prometheus-prometheus-kube-prometheus-prometheus"
}

function wait_for_trace_resources() {
    wait_for_rollout ${TRACE_NAMESPACE} "deployment/trace-receiver"
    wait_for_rollout kyma-system "daemonset/telemetry-otlp-gateway"
    wait_for_rollout ${TRACE_NAMESPACE} "deployment/trace-load-generator"
}

function wait_for_metric_resources() {
    wait_for_rollout ${METRIC_NAMESPACE} "deployment/metric-receiver"
    wait_for_rollout kyma-system "daemonset/telemetry-otlp-gateway"
    wait_for_rollout ${METRIC_NAMESPACE} "deployment/metric-load-generator"
}

function wait_for_metric_agent_resources() {
    wait_for_rollout ${METRIC_NAMESPACE} "deployment/metric-receiver"
    wait_for_rollout kyma-system "daemonset/telemetry-otlp-gateway"
    wait_for_rollout kyma-system "daemonset/telemetry-metric-agent"
    wait_for_rollout ${METRIC_NAMESPACE} "deployment/metric-agent-load-generator"
}

function wait_for_fluentbit_resources() {
    wait_for_rollout ${LOG_NAMESPACE} "deployment/log-receiver"
    wait_for_rollout kyma-system "daemonset/telemetry-fluent-bit" "fluent-bit"
    wait_for_rollout ${LOG_NAMESPACE} "deployment/log-load-generator"
}

function wait_for_otel_log_resources() {
    wait_for_rollout ${LOG_NAMESPACE} "deployment/log-receiver"
    wait_for_rollout ${LOG_NAMESPACE} "deployment/log-gateway"
    wait_for_rollout ${LOG_NAMESPACE} "deployment/log-load-generator"
}

function wait_for_selfmonitor_resources() {
  wait_for_rollout ${SELF_MONITOR_NAMESPACE} "deployment/telemetry-receiver"
    wait_for_rollout kyma-system "daemonset/telemetry-otlp-gateway"
    wait_for_rollout kyma-system "daemonset/telemetry-metric-agent"
    wait_for_rollout kyma-system "daemonset/telemetry-fluent-bit" "fluent-bit"
    wait_for_rollout ${SELF_MONITOR_NAMESPACE} "deployment/trace-load-generator"
    wait_for_rollout ${SELF_MONITOR_NAMESPACE} "deployment/metric-load-generator"
    wait_for_rollout ${SELF_MONITOR_NAMESPACE} "deployment/metric-agent-load-generator"
}

function cleanup() {
    echo -e "Check prometheus healthiness"
    wait_for_prometheus_resources

    echo -e "Check connectivity to prometheus using URL: $PROMAPI"
    PROMETHEUS_API_ENDPOINT_STATUS=$(curl -fs --data-urlencode "query=up" $PROMAPI | jq -r '.status')

    if [[ "$PROMETHEUS_API_ENDPOINT_STATUS" != "success" ]]; then
     echo "Prometheus API endpoint is not healthy"
     kill %1
    fi

    if [[ -z "$DOMAIN" ]]; then
     kubectl -n "$PROMETHEUS_NAMESPACE" port-forward "$(kubectl -n "$PROMETHEUS_NAMESPACE" get service -l app=kube-prometheus-stack-prometheus -oname)" 9090 &
     sleep 3
    fi

    echo -e "Collecting test results"
    case "$TEST_TARGET" in
        traces) get_result_and_cleanup_trace ;;
        metrics) get_result_and_cleanup_metric ;;
        metricagent) get_result_and_cleanup_metricagent ;;
        logs-fluentbit) get_result_and_cleanup_fluentbit ;;
        logs-otel) get_result_and_cleanup_log_otel ;;
        self-monitor) get_result_and_cleanup_selfmonitor ;;
    esac

    echo -e "Data collected, writing reports"

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
  QUERY_RECEIVED='query=round(sum(rate(otelcol_receiver_accepted_spans_total{service="telemetry-otlp-gateway-metrics"}[20m])))'
  QUERY_EXPORTED='query=round(sum(rate(otelcol_exporter_sent_spans_total{exporter=~"otlp_grpc/load-test.*"}[20m])))'
  QUERY_QUEUE='query=avg(sum(otelcol_exporter_queue_size{service="telemetry-otlp-gateway-metrics"}))'
  QUERY_MEMORY='query=round(sum(avg_over_time(container_memory_working_set_bytes{namespace="kyma-system", container="collector"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-otlp-gateway"}[20m])) by (pod) / 1024 / 1024)'
  QUERY_CPU='query=round(sum(avg_over_time(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{namespace="kyma-system"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-otlp-gateway"}[20m])) by (pod), 0.1)'

  RESULT_RECEIVED=$(curl -fs --data-urlencode "$QUERY_RECEIVED" $PROMAPI | jq -r '.data.result[] | .value[1]')
  RESULT_EXPORTED=$(curl -fs --data-urlencode "$QUERY_EXPORTED" $PROMAPI | jq -r '.data.result[] | .value[1]')
  RESULT_QUEUE=$(curl -fs --data-urlencode "$QUERY_QUEUE" $PROMAPI | jq -r '.data.result[] | .value[1]')
  RESULT_MEMORY=$(curl -fs --data-urlencode "$QUERY_MEMORY" $PROMAPI | jq -r '.data.result[] | .value[1]' | paste -sd,)
  RESULT_CPU=$(curl -fs --data-urlencode "$QUERY_CPU" $PROMAPI | jq -r '.data.result[] | .value[1]' | paste -sd,)
  RESULT_RESTARTS_COLLECTOR=$(kubectl -n kyma-system get pod -l app.kubernetes.io/name=telemetry-otlp-gateway -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq -s 'add')

  validate_telemetry_results "trace"

  if [[ -z "$DOMAIN" ]]; then
    echo -e "Killing port-forward"
    kill %1
  fi

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
    QUERY_RECEIVED='query=round(sum(rate(otelcol_receiver_accepted_metric_points_total{service="telemetry-otlp-gateway-metrics"}[20m])))'
    QUERY_EXPORTED='query=round(sum(rate(otelcol_exporter_sent_metric_points_total{exporter=~"otlp_grpc/load-test.*"}[20m])))'
    QUERY_QUEUE='query=avg(sum(otelcol_exporter_queue_size{service="telemetry-otlp-gateway-metrics"}))'
    QUERY_MEMORY='query=round(sum(avg_over_time(container_memory_working_set_bytes{namespace="kyma-system", container="collector"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-otlp-gateway"}[20m])) by (pod) / 1024 / 1024)'
    QUERY_CPU='query=round(sum(avg_over_time(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{namespace="kyma-system"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-otlp-gateway"}[20m])) by (pod), 0.1)'

    RESULT_RECEIVED=$(curl -fs --data-urlencode "$QUERY_RECEIVED" $PROMAPI | jq -r '.data.result[] | .value[1]')
    RESULT_EXPORTED=$(curl -fs --data-urlencode "$QUERY_EXPORTED" $PROMAPI | jq -r '.data.result[] | .value[1]')
    RESULT_QUEUE=$(curl -fs --data-urlencode "$QUERY_QUEUE" $PROMAPI | jq -r '.data.result[] | .value[1]')
    RESULT_MEMORY=$(curl -fs --data-urlencode "$QUERY_MEMORY" $PROMAPI | jq -r '.data.result[] | .value[1]' | paste -sd,)
    RESULT_CPU=$(curl -fs --data-urlencode "$QUERY_CPU" $PROMAPI | jq -r '.data.result[] | .value[1]' | paste -sd,)
    RESULT_RESTARTS_GATEWAY=$(kubectl -n kyma-system get pod -l app.kubernetes.io/name=telemetry-otlp-gateway -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq -s 'add')

    validate_telemetry_results "metric"

    if [[ -z "$DOMAIN" ]]; then
      echo -e "Killing port-forward"
      kill %1
    fi

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
    QUERY_RECEIVED='query=round(sum(rate(otelcol_receiver_accepted_metric_points_total{service="telemetry-metric-agent-metrics"}[20m])))'
    QUERY_EXPORTED='query=round(sum(rate(otelcol_exporter_sent_metric_points_total{exporter=~"otlp_grpc/load-test.*"}[20m])))'
    QUERY_QUEUE='query=avg(sum(otelcol_exporter_queue_size{service="telemetry-metric-agent-metrics"}))'
    QUERY_MEMORY='query=round(sum(avg_over_time(container_memory_working_set_bytes{namespace="kyma-system", container="collector"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-metric-agent"}[20m])) by (pod) / 1024 / 1024)'
    QUERY_CPU='query=round(sum(avg_over_time(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{namespace="kyma-system"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-metric-agent"}[20m])) by (pod), 0.1)'

    RESULT_RECEIVED=$(curl -fs --data-urlencode "$QUERY_RECEIVED" $PROMAPI | jq -r '.data.result[] | .value[1]')
    RESULT_EXPORTED=$(curl -fs --data-urlencode "$QUERY_EXPORTED" $PROMAPI | jq -r '.data.result[] | .value[1]')
    RESULT_QUEUE=$(curl -fs --data-urlencode "$QUERY_QUEUE" $PROMAPI | jq -r '.data.result[] | .value[1]')
    RESULT_MEMORY=$(curl -fs --data-urlencode "$QUERY_MEMORY" $PROMAPI | jq -r '.data.result[] | .value[1]' | paste -sd,)
    RESULT_CPU=$(curl -fs --data-urlencode "$QUERY_CPU" $PROMAPI | jq -r '.data.result[] | .value[1]' | paste -sd,)
    RESULT_RESTARTS_GATEWAY=$(kubectl -n kyma-system get pod -l app.kubernetes.io/name=telemetry-otlp-gateway -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq -s 'add')
    RESULT_RESTARTS_AGENT=$(kubectl -n kyma-system get pod -l app.kubernetes.io/name=telemetry-metric-agent -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq -s 'add')

    validate_telemetry_results "metric-agent"

    if [[ -z "$DOMAIN" ]]; then
      echo -e "Killing port-forward"
      kill %1
    fi

    if [[ "$BACKPRESSURE_TEST" == "true" ]]; then
      kubectl delete -f hack/load-tests/metric-agent-backpressure-config.yaml
    fi
    kubectl delete -f hack/load-tests/metric-agent-test-setup.yaml
}

function get_result_and_cleanup_log_otel() {
  RESULT_TYPE="log"
  QUERY_RECEIVED='query=round(sum(rate(otelcol_receiver_accepted_log_records_total{service=~"log-gateway-metrics"}[20m])))'
  QUERY_EXPORTED='query=round(sum(rate(otelcol_exporter_sent_log_records_total{service=~"log-gateway-metrics"}[20m])))'
  QUERY_QUEUE='query=avg(sum(otelcol_exporter_queue_size{service=~"log-gateway-metrics"}))'
  QUERY_MEMORY='query=round(sum(avg_over_time(container_memory_working_set_bytes{namespace="log-load-test", container="collector"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="log-load-test", workload="log-gateway"}[20m])) by (pod) / 1024 / 1024)'
  QUERY_CPU='query=round(sum(avg_over_time(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{namespace="log-load-test"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="log-load-test", workload="log-gateway"}[20m])) by (pod), 0.1)'

  RESULT_RECEIVED=$(curl -fs --data-urlencode "$QUERY_RECEIVED" $PROMAPI | jq -r '.data.result[] | .value[1]')
  RESULT_EXPORTED=$(curl -fs --data-urlencode "$QUERY_EXPORTED" $PROMAPI | jq -r '.data.result[] | .value[1]')
  RESULT_QUEUE=$(curl -fs --data-urlencode "$QUERY_QUEUE" $PROMAPI | jq -r '.data.result[] | .value[1]')
  RESULT_MEMORY=$(curl -fs --data-urlencode "$QUERY_MEMORY" $PROMAPI | jq -r '.data.result[] | .value[1]' | paste -sd,)
  RESULT_CPU=$(curl -fs --data-urlencode "$QUERY_CPU" $PROMAPI | jq -r '.data.result[] | .value[1]' | paste -sd,)
  RESULT_RESTARTS_GATEWAY=$(kubectl -n log-load-test get pod -l app.kubernetes.io/name=log-gateway -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq -s 'add')
  RESULT_RESTARTS_GENERATOR=$(kubectl -n log-load-test get pod -l app.kubernetes.io/name=log-load-generator -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq -s 'add')

  validate_telemetry_results "otel-logs"

  if [[ -z "$DOMAIN" ]]; then
    echo -e "Killing port-forward"
    kill %1
  fi

  if [[ "$OVERLAY" == "batch" ]]; then
    kubectl delete -k hack/load-tests/otel-logs/batch
  else
    kubectl delete -k hack/load-tests/otel-logs/base
  fi
}

function get_result_and_cleanup_fluentbit() {
  RESULT_TYPE="log"

  QUERY_RECEIVED='query=round(sum(rate(fluentbit_input_bytes_total{service="telemetry-fluent-bit-metrics", name=~"load-test-.*"}[20m])) / 1024)'
  QUERY_EXPORTED='query=round(sum(rate(fluentbit_output_proc_bytes_total{service="telemetry-fluent-bit-metrics", name=~"load-test-.*"}[20m])) / 1024)'
  QUERY_RECEIVED_RECORDS='query=round(sum(rate(fluentbit_input_records_total{service="telemetry-fluent-bit-metrics", name=~"load-test-.*"}[20m])))'
  QUERY_EXPORTED_RECORDS='query=round(sum(rate(fluentbit_output_proc_records_total{service="telemetry-fluent-bit-metrics", name=~"load-test-.*"}[20m])))'
  QUERY_QUEUE='query=round(sum(avg_over_time(telemetry_fsbuffer_usage_bytes{service="telemetry-fluent-bit-exporter-metrics"}[20m])) / 1024)'
  QUERY_MEMORY='query=round(sum(avg_over_time(container_memory_working_set_bytes{namespace="kyma-system", container="fluent-bit"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-fluent-bit"}[20m])) by (pod) / 1024 / 1024)'
  QUERY_CPU='query=round(sum(avg_over_time(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{namespace="kyma-system"}[20m]) * on(namespace,pod) group_left(workload) avg_over_time(namespace_workload_pod:kube_pod_owner:relabel{namespace="kyma-system", workload="telemetry-fluent-bit"}[20m])) by (pod), 0.1)'

  RESULT_RECEIVED=$(curl -fs --data-urlencode "$QUERY_RECEIVED" $PROMAPI | jq -r '.data.result[] | .value[1]')
  RESULT_EXPORTED=$(curl -fs --data-urlencode "$QUERY_EXPORTED" $PROMAPI | jq -r '.data.result[] | .value[1]')
  RESULT_RECEIVED_RECORDS=$(curl -fs --data-urlencode "$QUERY_RECEIVED_RECORDS" $PROMAPI | jq -r '.data.result[] | .value[1]')
  RESULT_EXPORTED_RECORDS=$(curl -fs --data-urlencode "$QUERY_EXPORTED_RECORDS" $PROMAPI | jq -r '.data.result[] | .value[1]')
  RESULT_QUEUE=$(curl -fs --data-urlencode "$QUERY_QUEUE" $PROMAPI | jq -r '.data.result[] | .value[1]')
  RESULT_MEMORY=$(curl -fs --data-urlencode "$QUERY_MEMORY" $PROMAPI | jq -r '.data.result[] | .value[1]' | paste -sd,)
  RESULT_CPU=$(curl -fs --data-urlencode "$QUERY_CPU" $PROMAPI | jq -r '.data.result[] | .value[1]' | paste -sd,)
  RESULT_RESTARTS_FLUENTBIT=$(kubectl -n kyma-system get pod -l app.kubernetes.io/name=fluent-bit -ojsonpath='{.items[0].status.containerStatuses[*].restartCount}' | jq -s 'add')

  validate_telemetry_results "fluent-bit"

  if [[ -z "$DOMAIN" ]]; then
    echo -e "Killing port-forward"
    kill %1
  fi

  if [[ "$MAX_PIPELINE" == "true" ]]; then
    kubectl delete -f hack/load-tests/log-fluentbit-max-pipeline.yaml
  fi
  if [[ "$BACKPRESSURE_TEST" == "true" ]]; then
    kubectl delete -f hack/load-tests/log-fluentbit-backpressure-config.yaml
  fi

  kubectl delete -f hack/load-tests/log-fluentbit-test-setup.yaml
}

function get_result_and_cleanup_selfmonitor() {
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

    if [[ -z "$DOMAIN" ]]; then
      echo -e "Killing port-forward"
      kill %1
    fi

    kubectl delete -f hack/load-tests/self-monitor-test-setup.yaml
}


function validate_telemetry_results() {
    local test_type=$1

    if [[ -z "$RESULT_RECEIVED" ]]; then
        echo "ERROR: RESULT_RECEIVED is empty for $test_type test. No data received from Prometheus query."
        exit 1
    fi

    if [[ -z "$RESULT_EXPORTED" ]]; then
        echo "ERROR: RESULT_EXPORTED is empty for $test_type test. No data exported according to Prometheus query."
        exit 1
    fi

    # Check if values are numeric zero (using awk for floating point comparison)
    if awk "BEGIN {exit !($RESULT_RECEIVED == 0)}"; then
        echo "ERROR: RESULT_RECEIVED is zero for $test_type test. No telemetry data was received during the test period."
        exit 1
    fi

    if awk "BEGIN {exit !($RESULT_EXPORTED == 0)}"; then
        echo "ERROR: RESULT_EXPORTED is zero for $test_type test. No telemetry data was exported during the test period."
        exit 1
    fi
}

# cleanup on exit. cleanup also collects the results and writes them to a file
trap cleanup EXIT
print_config
echo -e "Preparing test setup"
setup
echo -e "Waiting till test setup is ready"
wait_for_resources
echo -e "Test setup is ready, starting test"

for (( c=$TEST_DURATION; c>=0; c=c-60 ))
do  
  echo "Time remaining: $c seconds"
  sleep 60

done
