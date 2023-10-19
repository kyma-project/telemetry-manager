package gateway

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/servicename"
)

func makeProcessorsConfig() Processors {
	return Processors{
		BaseProcessors: config.BaseProcessors{
			Batch:         makeBatchProcessorConfig(),
			MemoryLimiter: makeMemoryLimiterConfig(),
			K8sAttributes: makeK8sAttributesProcessorConfig(),
			Resource:      makeResourceProcessorConfig(),
		},
		CumulativeToDelta:  &CumulativeToDeltaProcessor{},
		ResolveServiceName: makeResolveServiceNameConfig(),
	}
}

func makeBatchProcessorConfig() *config.BatchProcessor {
	return &config.BatchProcessor{
		SendBatchSize:    1024,
		Timeout:          "10s",
		SendBatchMaxSize: 1024,
	}
}

func makeMemoryLimiterConfig() *config.MemoryLimiter {
	return &config.MemoryLimiter{
		CheckInterval:        "0.1s",
		LimitPercentage:      75,
		SpikeLimitPercentage: 10,
	}
}

func makeK8sAttributesProcessorConfig() *config.K8sAttributesProcessor {
	k8sAttributes := []string{
		"k8s.pod.name",
		"k8s.node.name",
		"k8s.namespace.name",
		"k8s.deployment.name",
		"k8s.statefulset.name",
		"k8s.daemonset.name",
		"k8s.cronjob.name",
		"k8s.job.name",
	}

	podAssociations := []config.PodAssociations{
		{
			Sources: []config.PodAssociation{{From: "resource_attribute", Name: "k8s.pod.ip"}},
		},
		{
			Sources: []config.PodAssociation{{From: "resource_attribute", Name: "k8s.pod.uid"}},
		},
		{
			Sources: []config.PodAssociation{{From: "connection"}},
		},
	}

	return &config.K8sAttributesProcessor{
		AuthType:    "serviceAccount",
		Passthrough: false,
		Extract: config.ExtractK8sMetadata{
			Metadata: k8sAttributes,
			Labels:   servicename.ExtractLabels(),
		},
		PodAssociation: podAssociations,
	}
}

func makeResourceProcessorConfig() *config.ResourceProcessor {
	return &config.ResourceProcessor{
		Attributes: []config.AttributeAction{
			{
				Action: "insert",
				Key:    "k8s.cluster.name",
				Value:  "${KUBERNETES_SERVICE_HOST}",
			},
		},
	}
}

func makeDropIfInputSourceRuntimeConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetric{
			DataPoint: []string{
				fmt.Sprintf("resource.attributes[\"%s\"] == \"%s\"", metric.InputSourceAttribute, metric.InputSourceRuntime),
			},
		},
	}
}

func makeDropIfInputSourcePrometheusConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetric{
			DataPoint: []string{
				fmt.Sprintf("resource.attributes[\"%s\"] == \"%s\"", metric.InputSourceAttribute, metric.InputSourcePrometheus),
			},
		},
	}
}

func makeDropIfInputSourceIstioConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetric{
			DataPoint: []string{
				fmt.Sprintf("resource.attributes[\"%s\"] == \"%s\"", metric.InputSourceAttribute, metric.InputSourceIstio),
			},
		},
	}
}

func makeResolveServiceNameConfig() *TransformProcessor {
	return &TransformProcessor{
		ErrorMode: "ignore",
		MetricStatements: []TransformProcessorMetricStatements{
			{
				Context: "resource",
				Statements: []string{
					"set(attributes[\"service.name\"], attributes[\"kyma.kubernetes_io_app_name\"]) where attributes[\"service.name\"] == nil",
					"set(attributes[\"service.name\"], attributes[\"kyma.app_name\"]) where attributes[\"service.name\"] == nil",
					"set(attributes[\"service.name\"], attributes[\"k8s.deployment.name\"]) where attributes[\"service.name\"] == nil",
					"set(attributes[\"service.name\"], attributes[\"k8s.daemonset.name\"]) where attributes[\"service.name\"] == nil",
					"set(attributes[\"service.name\"], attributes[\"k8s.statefulset.name\"]) where attributes[\"service.name\"] == nil",
					"set(attributes[\"service.name\"], attributes[\"k8s.job.name\"]) where attributes[\"service.name\"] == nil",
					"set(attributes[\"service.name\"], attributes[\"k8s.pod.name\"]) where attributes[\"service.name\"] == nil",
					"set(attributes[\"service.name\"], \"unknown_service\") where attributes[\"service.name\"] == nil",
				},
			},
		},
	}
}
