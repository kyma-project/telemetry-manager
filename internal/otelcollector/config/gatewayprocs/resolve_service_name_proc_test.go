package gatewayprocs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveServiceNameStatements(t *testing.T) {
	require := require.New(t)

	expectedStatements := []string{
		"set(attributes[\"service.name\"], attributes[\"kyma.kubernetes_io_app_name\"]) where attributes[\"service.name\"] == nil or attributes[\"service.name\"] == \"\" or IsMatch(attributes[\"service.name\"], \"^unknown_service(:.+)?$\")",
		"set(attributes[\"service.name\"], attributes[\"kyma.app_name\"]) where attributes[\"service.name\"] == nil or attributes[\"service.name\"] == \"\" or IsMatch(attributes[\"service.name\"], \"^unknown_service(:.+)?$\")",
		"set(attributes[\"service.name\"], attributes[\"k8s.deployment.name\"]) where attributes[\"service.name\"] == nil or attributes[\"service.name\"] == \"\" or IsMatch(attributes[\"service.name\"], \"^unknown_service(:.+)?$\")",
		"set(attributes[\"service.name\"], attributes[\"k8s.daemonset.name\"]) where attributes[\"service.name\"] == nil or attributes[\"service.name\"] == \"\" or IsMatch(attributes[\"service.name\"], \"^unknown_service(:.+)?$\")",
		"set(attributes[\"service.name\"], attributes[\"k8s.statefulset.name\"]) where attributes[\"service.name\"] == nil or attributes[\"service.name\"] == \"\" or IsMatch(attributes[\"service.name\"], \"^unknown_service(:.+)?$\")",
		"set(attributes[\"service.name\"], attributes[\"k8s.job.name\"]) where attributes[\"service.name\"] == nil or attributes[\"service.name\"] == \"\" or IsMatch(attributes[\"service.name\"], \"^unknown_service(:.+)?$\")",
		"set(attributes[\"service.name\"], attributes[\"k8s.pod.name\"]) where attributes[\"service.name\"] == nil or attributes[\"service.name\"] == \"\" or IsMatch(attributes[\"service.name\"], \"^unknown_service(:.+)?$\")",
		"set(attributes[\"service.name\"], \"unknown_service\") where attributes[\"service.name\"] == nil or attributes[\"service.name\"] == \"\"",
	}

	statements := ResolveServiceNameStatements()

	require.Len(statements, 1)
	require.Equal("resource", statements[0].Context)
	require.Equal(expectedStatements, statements[0].Statements)
}
