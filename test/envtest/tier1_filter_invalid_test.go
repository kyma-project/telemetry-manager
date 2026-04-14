package envtest

// Tier 1 envtest example — webhook admission rejection.
// Original e2e test: test/e2e/traces/filter_invalid_test.go

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

func TestFilterInvalid(t *testing.T) {
	fix := setupWebhookOnly(t)

	pipeline := telemetryv1beta1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "filter-invalid-envtest",
		},
		Spec: telemetryv1beta1.TracePipelineSpec{
			Output: telemetryv1beta1.TracePipelineOutput{
				OTLP: &telemetryv1beta1.OTLPOutput{
					Endpoint: telemetryv1beta1.ValueType{Value: "https://backend.example.com:4317"},
				},
			},
			Filters: []telemetryv1beta1.FilterSpec{
				{
					Conditions: []string{
						`Len(resource.attributes["k8s.namespace.name"]) > 0`, // valid: has context prefix
						`attributes["foo"] == "bar"`,                         // invalid: missing context prefix
					},
				},
			},
		},
	}

	err := fix.client.Create(fix.ctx, &pipeline)

	require.Error(t, err, "expected webhook to reject pipeline with invalid filter condition")
	assert.Contains(t, err.Error(), "OTTL", "error should mention OTTL validation failure")
}

func TestFilterValid(t *testing.T) {
	fix := setupWebhookOnly(t)

	pipeline := telemetryv1beta1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "filter-valid-envtest",
		},
		Spec: telemetryv1beta1.TracePipelineSpec{
			Output: telemetryv1beta1.TracePipelineOutput{
				OTLP: &telemetryv1beta1.OTLPOutput{
					Endpoint: telemetryv1beta1.ValueType{Value: "https://backend.example.com:4317"},
				},
			},
			Filters: []telemetryv1beta1.FilterSpec{
				{
					Conditions: []string{
						`Len(resource.attributes["k8s.namespace.name"]) > 0`,
					},
				},
			},
		},
	}

	err := fix.client.Create(fix.ctx, &pipeline)

	require.NoError(t, err, "expected webhook to accept pipeline with valid filter condition")

	t.Cleanup(func() {
		_ = fix.client.Delete(fix.ctx, &pipeline)
	})
}
