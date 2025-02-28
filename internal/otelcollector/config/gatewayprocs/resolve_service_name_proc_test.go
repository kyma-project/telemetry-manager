package gatewayprocs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveServiceNameStatements(t *testing.T) {
	require := require.New(t)

	expectedStatements := []string{
		"set(resource.attributes[\"service.name\"], resource.attributes[\"kyma.kubernetes_io_app_name\"]) where resource.attributes[\"service.name\"] == nil or resource.attributes[\"service.name\"] == \"\" or IsMatch(resource.attributes[\"service.name\"], \"^unknown_service(:.+)?$\")",
		"set(resource.attributes[\"service.name\"], resource.attributes[\"kyma.app_name\"]) where resource.attributes[\"service.name\"] == nil or resource.attributes[\"service.name\"] == \"\" or IsMatch(resource.attributes[\"service.name\"], \"^unknown_service(:.+)?$\")",
		"set(resource.attributes[\"service.name\"], resource.attributes[\"k8s.deployment.name\"]) where resource.attributes[\"service.name\"] == nil or resource.attributes[\"service.name\"] == \"\" or IsMatch(resource.attributes[\"service.name\"], \"^unknown_service(:.+)?$\")",
		"set(resource.attributes[\"service.name\"], resource.attributes[\"k8s.daemonset.name\"]) where resource.attributes[\"service.name\"] == nil or resource.attributes[\"service.name\"] == \"\" or IsMatch(resource.attributes[\"service.name\"], \"^unknown_service(:.+)?$\")",
		"set(resource.attributes[\"service.name\"], resource.attributes[\"k8s.statefulset.name\"]) where resource.attributes[\"service.name\"] == nil or resource.attributes[\"service.name\"] == \"\" or IsMatch(resource.attributes[\"service.name\"], \"^unknown_service(:.+)?$\")",
		"set(resource.attributes[\"service.name\"], resource.attributes[\"k8s.job.name\"]) where resource.attributes[\"service.name\"] == nil or resource.attributes[\"service.name\"] == \"\" or IsMatch(resource.attributes[\"service.name\"], \"^unknown_service(:.+)?$\")",
		"set(resource.attributes[\"service.name\"], resource.attributes[\"k8s.pod.name\"]) where resource.attributes[\"service.name\"] == nil or resource.attributes[\"service.name\"] == \"\" or IsMatch(resource.attributes[\"service.name\"], \"^unknown_service(:.+)?$\")",
		"set(resource.attributes[\"service.name\"], \"unknown_service\") where resource.attributes[\"service.name\"] == nil or resource.attributes[\"service.name\"] == \"\"",
	}

	statements := ResolveServiceNameStatements()

	require.Len(statements, 1)
	require.Equal(expectedStatements, statements[0].Statements)
}
