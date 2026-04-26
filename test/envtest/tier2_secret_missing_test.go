package envtest

// Tier 2 envtest example — controller status condition check.
// Original e2e test: test/e2e/traces/secret_missing_test.go

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
)

func TestSecretMissing(t *testing.T) {
	fix := setupWithController(t)

	const (
		secretName    = "trace-endpoint-secret"
		secretNs      = "default"
		endpointKey   = "traces-endpoint"
		endpointValue = "http://localhost:4317"
		pipelineName  = "secret-missing-envtest"
	)

	pipeline := telemetryv1beta1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: pipelineName,
		},
		Spec: telemetryv1beta1.TracePipelineSpec{
			Output: telemetryv1beta1.TracePipelineOutput{
				OTLP: &telemetryv1beta1.OTLPOutput{
					Endpoint: telemetryv1beta1.ValueType{
						ValueFrom: &telemetryv1beta1.ValueFromSource{
							SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
								Name:      secretName,
								Namespace: secretNs,
								Key:       endpointKey,
							},
						},
					},
				},
			},
		},
	}

	require.NoError(t, fix.client.Create(fix.ctx, &pipeline))

	t.Cleanup(func() {
		_ = fix.client.Delete(fix.ctx, &pipeline)
	})

	// Wait for the controller to set ReferencedSecretMissing condition
	require.Eventually(t, func() bool {
		var p telemetryv1beta1.TracePipeline
		if err := fix.client.Get(fix.ctx, types.NamespacedName{Name: pipelineName}, &p); err != nil {
			return false
		}

		cond := meta.FindStatusCondition(p.Status.Conditions, conditions.TypeConfigurationGenerated)

		return cond != nil &&
			cond.Status == metav1.ConditionFalse &&
			cond.Reason == conditions.ReasonReferencedSecretMissing
	}, 30*time.Second, 200*time.Millisecond,
		"expected ConfigurationGenerated=False/ReferencedSecretMissing",
	)

	// Verify FlowHealthy is also False
	var p telemetryv1beta1.TracePipeline
	require.NoError(t, fix.client.Get(fix.ctx, types.NamespacedName{Name: pipelineName}, &p))

	flowHealthy := meta.FindStatusCondition(p.Status.Conditions, conditions.TypeFlowHealthy)
	require.NotNil(t, flowHealthy)
	assert.Equal(t, metav1.ConditionFalse, flowHealthy.Status)
	assert.Equal(t, conditions.ReasonSelfMonConfigNotGenerated, flowHealthy.Reason)

	// Create the secret — the pipeline should heal
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNs,
		},
		StringData: map[string]string{
			endpointKey: endpointValue,
		},
	}
	require.NoError(t, fix.client.Create(fix.ctx, secret))

	t.Cleanup(func() {
		_ = fix.client.Delete(fix.ctx, secret)
	})

	// In the real operator, a secret watcher triggers re-reconciliation when referenced secrets change.
	// In envtest we use mocked dependencies, so we nudge the controller by updating the pipeline.
	var current telemetryv1beta1.TracePipeline
	require.NoError(t, fix.client.Get(fix.ctx, types.NamespacedName{Name: pipelineName}, &current))

	if current.Annotations == nil {
		current.Annotations = map[string]string{}
	}

	current.Annotations["envtest-trigger"] = "secret-created"
	require.NoError(t, fix.client.Update(fix.ctx, &current))

	// Wait for the pipeline to become healthy (ConfigurationGenerated=True)
	require.Eventually(t, func() bool {
		var p telemetryv1beta1.TracePipeline
		if err := fix.client.Get(fix.ctx, types.NamespacedName{Name: pipelineName}, &p); err != nil {
			return false
		}

		cond := meta.FindStatusCondition(p.Status.Conditions, conditions.TypeConfigurationGenerated)

		return cond != nil && cond.Status == metav1.ConditionTrue
	}, 30*time.Second, 200*time.Millisecond,
		"expected pipeline to heal after secret creation (ConfigurationGenerated=True)",
	)
}
