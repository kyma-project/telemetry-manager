package metricagent

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
)

func deleteServiceNameProcessorConfig() *common.ResourceProcessor {
	return &common.ResourceProcessor{
		Attributes: []common.AttributeAction{
			{
				Action: "delete",
				Key:    "service.name",
			},
		},
	}
}

func insertSkipEnrichmentAttributeProcessorConfig() *common.TransformProcessor {
	metricsToSkipEnrichment := []string{
		"node",
		"statefulset",
		"daemonset",
		"deployment",
		"job",
	}

	return common.MetricTransformProcessorConfig([]common.TransformProcessorStatements{{
		Conditions: metricNameConditionsWithIsMatch(metricsToSkipEnrichment),
		Statements: []string{fmt.Sprintf("set(resource.attributes[\"%s\"], \"true\")", common.SkipEnrichmentAttribute)},
	}})
}

func dropNonPVCVolumesMetricsProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				common.JoinWithAnd(
					// identify volume metrics by checking existence of "k8s.volume.name" resource attribute
					common.ResourceAttributeIsNotNil("k8s.volume.name"),
					common.ResourceAttributeNotEquals("k8s.volume.type", "persistentVolumeClaim"),
				),
			},
		},
	}
}

func metricNameConditionsWithIsMatch(metrics []string) []string {
	var conditions []string

	for _, m := range metrics {
		condition := common.IsMatch("metric.name", fmt.Sprintf("^k8s.%s.*", m))
		conditions = append(conditions, condition)
	}

	return conditions
}

func dropVirtualNetworkInterfacesProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Datapoint: []string{
				common.JoinWithAnd(
					common.IsMatch("metric.name", "^k8s.node.network.*"),
					common.Not(common.IsMatch("attributes[\"interface\"]", "^(eth|en).*")),
				),
			},
		},
	}
}
