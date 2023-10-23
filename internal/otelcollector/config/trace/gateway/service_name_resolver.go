package gateway

import (
	"fmt"
)

func makeResolveServiceNameConfig() *TransformProcessor {
	attributes := []string{
		"kyma.kubernetes_io_app_name",
		"kyma.app_name",
		"k8s.deployment.name",
		"k8s.daemonset.name",
		"k8s.statefulset.name",
		"k8s.job.name",
		"k8s.pod.name",
	}

	var statements []string
	for _, attr := range attributes {
		statements = append(statements, inferServiceNameFromAttr(attr))
	}
	statements = append(statements, setDefaultServiceName())

	return &TransformProcessor{
		ErrorMode: "ignore",
		MetricStatements: []TransformProcessorTraceStatements{
			{
				Context:    "resource",
				Statements: statements,
			},
		},
	}
}

const serviceNameNotDefinedCondition = "attributes[\"service.name\"] == nil or attributes[\"service.name\"] == \"\" or attributes[\"service.name\"] == \"unknown_service\""

func inferServiceNameFromAttr(attrKey string) string {
	return fmt.Sprintf(
		"set(attributes[\"service.name\"], attributes[\"%s\"]) where %s",
		attrKey,
		serviceNameNotDefinedCondition,
	)
}

func setDefaultServiceName() string {
	return fmt.Sprintf(
		"set(attributes[\"service.name\"], \"unknown_service\") where %s",
		serviceNameNotDefinedCondition,
	)
}
