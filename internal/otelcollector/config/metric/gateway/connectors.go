package gateway

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
)

func makeRoutingConnectorConfig(pipelineName string) RoutingConnector {
	attributesEnrichmentPipelineID := formatAttributesEnrichmentPipelineID(pipelineName)
	outputPipelineID := formatOutputPipelineID(pipelineName)

	return RoutingConnector{
		DefaultPipelines: []string{attributesEnrichmentPipelineID},
		ErrorMode:        "ignore",
		Table: []RoutingConnectorTableEntry{
			{
				Statement: fmt.Sprintf("route() where attributes[\"%s\"] == \"true\"", metric.SkipEnrichmentAttribute),
				Pipelines: []string{outputPipelineID},
			},
		},
	}
}
