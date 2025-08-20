package metricgateway

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
)

func routingConnectorConfig(pipelineName string) RoutingConnector {
	attributesEnrichmentPipelineID := formatMetricEnrichmentPipelineID(pipelineName)
	outputPipelineID := formatMetricOutputPipelineID(pipelineName)

	return RoutingConnector{
		DefaultPipelines: []string{attributesEnrichmentPipelineID},
		ErrorMode:        "ignore",
		Table: []RoutingConnectorTableEntry{
			{
				Statement: fmt.Sprintf("route() where attributes[\"%s\"] == \"true\"", common.SkipEnrichmentAttribute),
				Pipelines: []string{outputPipelineID},
			},
		},
	}
}
