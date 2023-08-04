package istio

var AccessLogAttributeKeys []string {
	"method",
	"path",
	"protocol",
	"response_code",
	"response_flags",
	"response_code_details",
	"connection_termination_details",
	"upstream_transport_failure_reason",
	"bytes_received",
	"bytes_sent",
	"duration",
	"upstream_service_time"
	"x_forwarded_for",
	"user_agent",
	"request_id",
	"authority"
	"upstream_host",
	"upstream_cluster",
	"upstream_local_address",
	"downstream_local_address",
	"downstream_remote_address",
	"requested_server_name",
	"route_name",
	"traceparent",
	"tracestate"
}
