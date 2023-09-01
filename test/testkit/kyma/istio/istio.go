package istio

var AccessLogAttributeKeys = []string{
	"response_code",
	"response_flags",
	"response_code_details",
	"bytes_received",
	"bytes_sent",
	"duration",
	"user_agent",
	"request_id",
	"authority",
	"upstream_cluster",
	"downstream_local_address",
	"downstream_remote_address",
	"route_name",
}
