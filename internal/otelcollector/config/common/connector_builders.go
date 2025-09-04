package common

import (
	"fmt"
)

// =============================================================================
// ENRICHMENT CONNECTOR BUILDERS
// =============================================================================

func EnrichmentRoutingConnectorConfig(defaultPipelineIDs, outputPipelineIDs []string) RoutingConnector {
	return RoutingConnector{
		DefaultPipelines: defaultPipelineIDs,
		ErrorMode:        "ignore",
		Table: []RoutingConnectorTableEntry{
			{
				Statement: fmt.Sprintf("route() where attributes[\"%s\"] == \"true\"", SkipEnrichmentAttribute),
				Pipelines: outputPipelineIDs,
			},
		},
	}
}
