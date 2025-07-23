package processors

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

func TestInsertClusterNameProcessorConfig(t *testing.T) {
	require := require.New(t)

	expectedAttributeActions := []config.AttributeAction{
		{
			Action: "insert",
			Key:    "k8s.cluster.name",
			Value:  "test-cluster",
		},
		{
			Action: "insert",
			Key:    "k8s.cluster.uid",
			Value:  "test-cluster-uid",
		},
		{
			Action: "insert",
			Key:    "cloud.provider",
			Value:  "test-cloud-provider",
		},
	}

	config := InsertClusterAttributesProcessorConfig("test-cluster", "test-cluster-uid", "test-cloud-provider")

	require.ElementsMatch(expectedAttributeActions, config.Attributes, "Attributes should match")
}

func TestDropKymaAttributesProcessorConfig(t *testing.T) {
	require := require.New(t)

	expectedAttributeActions := []config.AttributeAction{
		{
			Action:       "delete",
			RegexPattern: "kyma.*",
		},
	}

	config := DropKymaAttributesProcessorConfig()

	require.ElementsMatch(expectedAttributeActions, config.Attributes, "Attributes should match")
}
