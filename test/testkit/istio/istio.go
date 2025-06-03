package istio

var AccessLogAttributeKeys = []string{
	"method",
	"path",
	"protocol",
	"response_code",
	"response_flags",
	"response_code_details",
	"bytes_received",
	"bytes_sent",
	"duration",
	"upstream_service_time",
	"x_forwarded_for",
	"user_agent",
	"request_id",
	"authority",
	"upstream_host",
	"upstream_cluster",
	"upstream_local_address",
	"downstream_local_address",
	"downstream_remote_address",
	"route_name",
}

var AccessLogOTLPLogAttributeKeys = []string{
	"client.address",
	"client.port",
	"server.address",
	"server.port",
	"http.direction",
	"http.request.duration",
	"http.request.header.referer",
	"http.request.header.x-forwarded-for",
	"http.request.header.x-request-id",
	"http.request.method",
	"http.request.size",
	"http.response.size",
	"http.response.status_code",
	"url.path",
	"url.query",
	"url.scheme",
	"user_agent.original",
	/* TODO: Enable these attributes after added to the istio telemetry extension configuration
	"network.protocol.name",
	"network.protocol.version",
	 */
}
