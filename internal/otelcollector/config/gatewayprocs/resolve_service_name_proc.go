package gatewayprocs

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/ottlexpr"
)

func ResolveServiceNameStatements() []config.TransformProcessorStatements {
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

	return []config.TransformProcessorStatements{
		{
			Context:    "resource",
			Statements: statements,
		},
	}
}

// serviceNameNotDefinedBasicCondition specifies the cases for which the service.name attribute is not defined
// without considering the "unknown_service" and the "unknown_service:<process.executable.name>" cases
const serviceNameNotDefinedBasicCondition = "attributes[\"service.name\"] == nil or attributes[\"service.name\"] == \"\""

func inferServiceNameFromAttr(attrKey string) string {
	// serviceNameNotDefinedCondition builds up on the serviceNameNotDefinedBasicCondition
	// to consider the "unknown_service" and the "unknown_service:<process.executable.name>" cases
	serviceNameNotDefinedCondition := fmt.Sprintf(
		"%s or %s",
		serviceNameNotDefinedBasicCondition,
		ottlexpr.IsMatch("attributes[\"service.name\"]", "^unknown_service(:.+)?$"),
	)
	return fmt.Sprintf(
		"set(attributes[\"service.name\"], attributes[\"%s\"]) where %s",
		attrKey,
		serviceNameNotDefinedCondition,
	)
}

func setDefaultServiceName() string {
	return fmt.Sprintf(
		"set(attributes[\"service.name\"], \"unknown_service\") where %s",
		serviceNameNotDefinedBasicCondition,
	)
}
