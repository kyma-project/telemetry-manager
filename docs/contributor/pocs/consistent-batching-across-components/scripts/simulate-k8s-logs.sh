#!/usr/bin/env bash
#
# simulate-k8s-logs.sh
#
# Simulates live Kubernetes pods writing CRI-format logs in real time.
# Each "pod" is a background process appending lines at a configurable rate.
#
# Usage:
#   ./simulate-k8s-logs.sh [output_dir] [num_pods] [lines_per_sec_per_pod] [duration_sec] [complexity]
#
# Defaults:
#   output_dir            = ./var/log/pods
#   num_pods              = 10
#   lines_per_sec_per_pod = 100
#   duration_sec          = 300
#   complexity            = simple        ("simple" or "complex")
#
# Complexity modes (both target ~2 KB per line so byte volume is equivalent):
#   simple   ~4–5 attributes per log record after stanza json-parser
#            Heavy padding dominates the line.
#   complex  ~35+ attributes per record after json-parser, including
#            nested maps (http.request.headers, http.response.headers,
#            business) and arrays (feature_flags, tags).
#            Designed to stress `pdata.SizeProto` in the bytes-sizer path:
#            each KeyValue/AnyValue node is one recursive SizeProto call,
#            so more attributes → more calls per log at the same byte count.
#
# Stop early with Ctrl+C — all background writers will be cleaned up.

set -euo pipefail

OUTPUT_DIR="${1:-./var/log/pods}"
NUM_PODS="${2:-10}"
LINES_PER_SEC="${3:-100}"
DURATION="${4:-300}"
COMPLEXITY="${5:-simple}"

if [[ "$COMPLEXITY" != "simple" && "$COMPLEXITY" != "complex" ]]; then
    echo "Error: complexity (arg 5) must be 'simple' or 'complex' (got '$COMPLEXITY')" >&2
    exit 1
fi

# Padding strings tuned so both modes land at ~2 KB/line total.
#   simple:  CRI prefix + small JSON envelope (~150B) + pad(1847) ≈ 2 KB
#   complex: CRI prefix + ~1500B of literal JSON + pad(500) ≈ 2 KB
LOG_PADDING=$(printf '%1847s' '' | tr ' ' 'a')
COMPLEX_PADDING=$(printf '%500s' '' | tr ' ' 'a')

# Calculate sleep between lines (in seconds, as decimal)
if command -v bc &>/dev/null; then
    SLEEP_INTERVAL=$(echo "scale=4; 1 / $LINES_PER_SEC" | bc)
else
    SLEEP_INTERVAL=$(awk "BEGIN {printf \"%.4f\", 1/$LINES_PER_SEC}")
fi

PIDS=()

cleanup() {
    echo ""
    echo "Stopping all writers..."
    for pid in "${PIDS[@]}"; do
        kill "$pid" 2>/dev/null || true
    done
    wait 2>/dev/null
    echo "Done."
}
trap cleanup EXIT INT TERM

# ── Fake data ────────────────────────────────────────────────────────

NAMESPACES=("default" "production" "staging" "monitoring" "backend")
APPS=("api-gateway" "user-service" "order-processor" "payment-handler" "inventory-sync"
      "notification-worker" "auth-proxy" "cache-warmer" "metric-exporter" "log-aggregator")
CONTAINERS=("app" "sidecar" "istio-proxy")
METHODS=("GET" "POST" "PUT" "DELETE" "PATCH")
STATUSES=(200 201 204 301 400 401 403 404 500 502 503)

random_element() {
    local arr=("$@")
    echo "${arr[$((RANDOM % ${#arr[@]}))]}"
}

# ── Log line generators ──────────────────────────────────────────────

# SIMPLE: ~4-5 fields in the parsed JSON (level/msg/ts are consumed by
# the pipeline; a handful of extras stay as attributes). Padding dominates.
generate_json_line() {
    local ts="$1"
    local stream="stdout"
    ((RANDOM % 10 == 0)) && stream="stderr"

    local roll=$((RANDOM % 100))
    local level="info"
    local msg=""

    if ((roll < 70)); then
        level="info"
        local templates=(
            "request completed\",\"method\":\"GET\",\"path\":\"/api/v1/users\",\"status\":200,\"duration_ms\":$((RANDOM % 500 + 1))"
            "query executed\",\"table\":\"orders\",\"rows\":$((RANDOM % 1000)),\"duration_ms\":$((RANDOM % 200 + 1))"
            "cache hit\",\"key\":\"user:$((RANDOM % 99999))\",\"ttl_remaining\":$((RANDOM % 3600))"
            "job completed\",\"job_id\":\"job-$((RANDOM % 99999))\",\"queue\":\"default\",\"duration_ms\":$((RANDOM % 300))"
            "health check passed\",\"uptime_sec\":$((RANDOM % 86400))"
            "connection established\",\"peer\":\"10.0.$((RANDOM % 255)).$((RANDOM % 255)):$((RANDOM % 65535))\""
        )
        msg=$(random_element "${templates[@]}")
    elif ((roll < 90)); then
        level="warn"
        local templates=(
            "cache miss\",\"key\":\"session:$((RANDOM % 99999))\",\"fallback\":\"database\""
            "retrying request\",\"attempt\":$((RANDOM % 3 + 1)),\"max_attempts\":3,\"reason\":\"timeout\""
            "connection pool near capacity\",\"active\":$((RANDOM % 10 + 40)),\"max\":50"
            "slow query detected\",\"duration_ms\":$((RANDOM % 5000 + 1000)),\"table\":\"events\""
        )
        msg=$(random_element "${templates[@]}")
    else
        level="error"
        local templates=(
            "query failed\",\"error\":\"connection refused\",\"table\":\"users\""
            "upstream error\",\"host\":\"payment-svc\",\"status\":503"
            "context deadline exceeded\",\"timeout\":\"30s\""
            "authentication failed\",\"error\":\"token expired\""
        )
        msg=$(random_element "${templates[@]}")
    fi

    echo "${ts} ${stream} F {\"level\":\"${level}\",\"ts\":\"${ts}\",\"msg\":\"${msg},\"padding\":\"${LOG_PADDING}\"}"
}

# COMPLEX: ~40 top-level fields + 3 nested objects + 2 arrays.
# After the stanza pipeline, level/msg/trace_id/span_id/trace_flags are
# consumed into log record fields; the rest stay as attributes. Net:
# ~35+ KeyValue nodes per record, each costing a SizeProto call when the
# bytes sizer measures the batch.
generate_complex_json_line() {
    local ts="$1"
    local stream="stdout"
    ((RANDOM % 10 == 0)) && stream="stderr"

    local level="info"
    local roll=$((RANDOM % 100))
    if   ((roll >= 70 && roll < 90)); then level="warn"
    elif ((roll >= 90));              then level="error"
    fi

    local trace_id
    trace_id=$(printf '%08x%08x%08x%08x' $RANDOM $RANDOM $RANDOM $RANDOM)
    local span_id
    span_id=$(printf '%08x%08x' $RANDOM $RANDOM)

    local app; app=$(random_element "${APPS[@]}")
    local ns;  ns=$(random_element "${NAMESPACES[@]}")
    local method; method=$(random_element "${METHODS[@]}")
    local status; status=$(random_element "${STATUSES[@]}")

    local req_id="req-${RANDOM}${RANDOM}"
    local user_id=$((RANDOM % 100000))
    local tenant_id="tenant-$((RANDOM % 1000))"
    local session_id="sess-${RANDOM}${RANDOM}"
    local duration_ms=$((RANDOM % 500 + 1))
    local bytes_sent=$((RANDOM % 100000))
    local rows=$((RANDOM % 1000))

    # Single-line JSON body. The stanza json-parser will explode each
    # top-level key into an attribute; nested maps become nested AnyValue.
    # `\$1` / `\$2` are escaped so bash doesn't try to expand them.
    local body
    body=$(cat <<EOF
{"level":"${level}","ts":"${ts}","msg":"request completed","service":"${app}","service.version":"2.$((RANDOM%20)).$((RANDOM%10))","service.namespace":"${ns}","deployment.environment":"prod","k8s.cluster.name":"prod-us-east-1","k8s.node.name":"ip-10-0-$((RANDOM%255))-$((RANDOM%255))","k8s.pod.name":"${app}-$((RANDOM%99999))","k8s.container.name":"app","k8s.namespace.name":"${ns}","request_id":"${req_id}","trace_id":"${trace_id}","span_id":"${span_id}","trace_flags":"01","user_id":${user_id},"tenant_id":"${tenant_id}","session_id":"${session_id}","http.method":"${method}","http.url":"/api/v1/orders/$((RANDOM%99999))","http.route":"/api/v1/orders/:id","http.status_code":${status},"http.request_content_length":$((RANDOM%10000)),"http.response_content_length":${bytes_sent},"http.user_agent":"Mozilla/5.0 Chrome/130.0.0.0","net.peer.ip":"10.0.$((RANDOM%255)).$((RANDOM%255))","net.peer.port":$((RANDOM%65535)),"net.host.name":"api.example.com","duration_ms":${duration_ms},"db.system":"postgresql","db.name":"orders","db.statement":"SELECT * FROM orders WHERE tenant_id = \$1 AND status = \$2","db.rows_affected":${rows},"db.duration_ms":$((RANDOM%100)),"cache.hit":true,"cache.key":"orders:${tenant_id}:list","feature_flags":["new-checkout","improved-search","v2-pricing"],"tags":["api","orders","v1"],"http.request.headers":{"accept":"application/json","content-type":"application/json","x-forwarded-for":"203.0.113.$((RANDOM%255))","x-request-id":"${req_id}"},"http.response.headers":{"content-type":"application/json","cache-control":"no-store","x-request-id":"${req_id}"},"business":{"order_count":$((RANDOM%100)),"total_value_cents":$((RANDOM%10000000)),"currency":"USD","customer_tier":"gold"},"padding":"${COMPLEX_PADDING}"}
EOF
)

    echo "${ts} ${stream} F ${body}"
}

# Dispatch based on complexity
if [[ "$COMPLEXITY" == "complex" ]]; then
    GENERATE_FN="generate_complex_json_line"
else
    GENERATE_FN="generate_json_line"
fi

# ── Writer function (runs as background process per pod) ─────────────

write_logs() {
    local log_file="$1"
    local duration="$2"
    local sleep_interval="$3"

    local end_time=$((SECONDS + duration))

    while ((SECONDS < end_time)); do
        local ts
        ts=$(date -u +"%Y-%m-%dT%H:%M:%S.%NZ" 2>/dev/null || date -u +"%Y-%m-%dT%H:%M:%S.000000000Z")

        "$GENERATE_FN" "$ts" >> "$log_file"

        sleep "$sleep_interval"
    done
}

# ── Create pods and start writers ────────────────────────────────────

echo "Simulating Kubernetes log output"
echo "  Output:     $OUTPUT_DIR"
echo "  Pods:       $NUM_PODS"
echo "  Rate:       $LINES_PER_SEC lines/sec/pod ($(( LINES_PER_SEC * NUM_PODS )) total lines/sec)"
echo "  Duration:   ${DURATION}s"
echo "  Complexity: ${COMPLEXITY}"
echo "  Sleep:      ${SLEEP_INTERVAL}s between lines"
echo ""

for ((p = 0; p < NUM_PODS; p++)); do
    namespace=$(random_element "${NAMESPACES[@]}")
    app=$(random_element "${APPS[@]}")
    pod_hash=$(printf '%010d' $((RANDOM * RANDOM)))
    pod_name="${app}-${pod_hash}"
    pod_uid=$(printf '%04x%04x-%04x-%04x-%04x-%04x%04x%04x' \
        $RANDOM $RANDOM $RANDOM $RANDOM $RANDOM $RANDOM $RANDOM $RANDOM)

    container=$(random_element "${CONTAINERS[@]}")

    pod_dir="${OUTPUT_DIR}/${namespace}_${pod_name}_${pod_uid}/${container}"
    mkdir -p "$pod_dir"
    log_file="${pod_dir}/0.log"

    # Create empty file
    : > "$log_file"

    echo "  [pod $((p+1))/$NUM_PODS] ${namespace}/${pod_name}/${container} → ${log_file}"

    # Start background writer
    write_logs "$log_file" "$DURATION" "$SLEEP_INTERVAL" &
    PIDS+=($!)
done

echo ""
echo "Writing logs... (Ctrl+C to stop early)"
echo ""

# Show progress
elapsed=0
while ((elapsed < DURATION)); do
    sleep 5
    elapsed=$((elapsed + 5))
    total_lines=0
    while IFS= read -r f; do
        lines=$(wc -l < "$f")
        total_lines=$((total_lines + lines))
    done < <(find "$OUTPUT_DIR" -name "*.log" 2>/dev/null)
    echo "  [${elapsed}s / ${DURATION}s] Total lines written: $total_lines"
done

echo ""
echo "Simulation complete."
