package tracepipeline

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
	"github.com/kyma-project/telemetry-manager/internal/metrics"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline/stubs"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

// TestConfigMapUpdate verifies that valid pipelines are written to the OTLP Gateway Coordination ConfigMap
func TestConfigMapUpdate(t *testing.T) {
	tests := []struct {
		name              string
		pipeline          telemetryv1beta1.TracePipeline
		expectWritten     bool
		expectedConfigGen metav1.ConditionStatus
		expectedReason    string
		expectedMessage   string
	}{
		{
			name: "valid pipeline written to ConfigMap",
			pipeline: testutils.NewTracePipelineBuilder().
				WithName("valid-pipeline").
				WithOTLPOutput(testutils.OTLPEndpoint("http://backend:4317")).
				Build(),
			expectWritten:     true,
			expectedConfigGen: metav1.ConditionTrue,
			expectedReason:    conditions.ReasonGatewayConfigured,
			expectedMessage:   "TracePipeline specification is successfully applied to the configuration of OTLP gateway",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(&tt.pipeline).WithStatusSubresource(&tt.pipeline).Build()

			flowHealthProberStub := &mocks.FlowHealthProber{}
			flowHealthProberStub.On("Probe", mock.Anything, tt.pipeline.Name).Return(prober.OTelGatewayProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{Healthy: true},
			}, nil).Maybe()

			sut := testReconciler(fakeClient, flowHealthProberStub)

			_, err := sut.Reconcile(context.Background(), requestFor(tt.pipeline.Name))
			require.NoError(t, err)

			// Verify ConfigMap was written
			var configMap corev1.ConfigMap

			err = fakeClient.Get(context.Background(), types.NamespacedName{
				Name:      names.OTLPGatewayPipelinesSyncConfigMap,
				Namespace: "default",
			}, &configMap)

			if tt.expectWritten {
				require.NoError(t, err, "ConfigMap should exist")
				require.Contains(t, configMap.Data, "pipelines.yaml", "ConfigMap should contain pipelines.yaml key")
				require.Contains(t, configMap.Data["pipelines.yaml"], tt.pipeline.Name, "ConfigMap should contain pipeline reference")
			}

			// Verify status conditions
			var updatedPipeline telemetryv1beta1.TracePipeline

			err = fakeClient.Get(context.Background(), types.NamespacedName{Name: tt.pipeline.Name}, &updatedPipeline)
			require.NoError(t, err)

			requireHasStatusCondition(t, &updatedPipeline,
				conditions.TypeConfigurationGenerated,
				tt.expectedConfigGen,
				tt.expectedReason,
				tt.expectedMessage)

			flowHealthProberStub.AssertExpectations(t)
		})
	}
}

// TestSecretReferenceValidation verifies that pipelines with missing secrets are not written to ConfigMap
func TestSecretReferenceValidation(t *testing.T) {
	tests := []struct {
		name                  string
		setupPipeline         func() telemetryv1beta1.TracePipeline
		setupSecret           func() *corev1.Secret
		includeSecret         bool
		secretValidatorError  error
		expectConfigGenerated bool
		expectedConfigStatus  metav1.ConditionStatus
		expectedConfigReason  string
		expectedConfigMessage string
	}{
		{
			name: "referenced secret missing",
			setupPipeline: func() telemetryv1beta1.TracePipeline {
				return testutils.NewTracePipelineBuilder().
					WithOTLPOutput(testutils.OTLPBasicAuthFromSecret("some-secret", "some-namespace", "user", "password")).
					Build()
			},
			setupSecret:           func() *corev1.Secret { return nil },
			includeSecret:         false,
			secretValidatorError:  fmt.Errorf("%w: Secret 'some-secret' of Namespace 'some-namespace'", secretref.ErrSecretRefNotFound),
			expectConfigGenerated: false,
			expectedConfigStatus:  metav1.ConditionFalse,
			expectedConfigReason:  conditions.ReasonReferencedSecretMissing,
			expectedConfigMessage: "One or more referenced Secrets are missing: Secret 'some-secret' of Namespace 'some-namespace'",
		},
		{
			name: "referenced secret exists",
			setupPipeline: func() telemetryv1beta1.TracePipeline {
				return testutils.NewTracePipelineBuilder().
					WithOTLPOutput(testutils.OTLPEndpointFromSecret("existing", "default", "endpoint")).
					Build()
			},
			setupSecret: func() *corev1.Secret {
				return &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "existing",
						Namespace: "default",
					},
					Data: map[string][]byte{"endpoint": []byte("http://backend:4317")},
				}
			},
			includeSecret:         true,
			secretValidatorError:  nil,
			expectConfigGenerated: true,
			expectedConfigStatus:  metav1.ConditionTrue,
			expectedConfigReason:  conditions.ReasonGatewayConfigured,
			expectedConfigMessage: "TracePipeline specification is successfully applied to the configuration of OTLP gateway",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := tt.setupPipeline()

			var clientObjs []client.Object

			clientObjs = append(clientObjs, &pipeline)
			if tt.includeSecret {
				clientObjs = append(clientObjs, tt.setupSecret())
			}

			fakeClient := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(clientObjs...).WithStatusSubresource(&pipeline).Build()

			flowHealthProberStub := &mocks.FlowHealthProber{}
			flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelGatewayProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{Healthy: true},
			}, nil).Maybe()

			opts := []ValidatorOption{}
			if tt.secretValidatorError != nil {
				opts = append(opts, WithSecretRefValidator(stubs.NewSecretRefValidator(tt.secretValidatorError)))
			}

			sut := testReconcilerWithValidator(fakeClient, flowHealthProberStub, opts...)

			_, err := sut.Reconcile(context.Background(), requestFor(pipeline.Name))
			require.NoError(t, err)

			// Verify ConfigMap state
			var configMap corev1.ConfigMap

			err = fakeClient.Get(context.Background(), types.NamespacedName{
				Name:      names.OTLPGatewayPipelinesSyncConfigMap,
				Namespace: "default",
			}, &configMap)

			if tt.expectConfigGenerated {
				require.NoError(t, err, "ConfigMap should exist")
				require.Contains(t, configMap.Data["pipelines.yaml"], pipeline.Name, "ConfigMap should contain pipeline reference")
			} else if err == nil {
				// ConfigMap might exist but shouldn't contain this pipeline
				require.NotContains(t, configMap.Data["pipelines.yaml"], pipeline.Name, "ConfigMap should not contain invalid pipeline reference")
			}

			// Verify status
			var updatedPipeline telemetryv1beta1.TracePipeline

			err = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)
			require.NoError(t, err)

			requireHasStatusCondition(t, &updatedPipeline,
				conditions.TypeConfigurationGenerated,
				tt.expectedConfigStatus,
				tt.expectedConfigReason,
				tt.expectedConfigMessage)

			if !tt.expectConfigGenerated {
				requireHasStatusCondition(t, &updatedPipeline,
					conditions.TypeFlowHealthy,
					metav1.ConditionFalse,
					conditions.ReasonSelfMonConfigNotGenerated,
					"No spans delivered to backend because TracePipeline specification is not applied to the configuration of OTLP gateway. Check the 'ConfigurationGenerated' condition for more details")
			}

			flowHealthProberStub.AssertExpectations(t)
		})
	}
}

// TestMaxPipelineLimit verifies that pipelines exceeding the limit are not written to ConfigMap
func TestMaxPipelineLimit(t *testing.T) {
	pipeline := testutils.NewTracePipelineBuilder().Build()
	fakeClient := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

	pipelineLockStub := &mocks.PipelineLock{}
	pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(resourcelock.ErrMaxPipelinesExceeded)
	pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(resourcelock.ErrMaxPipelinesExceeded)

	flowHealthProberStub := &mocks.FlowHealthProber{}
	flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelGatewayProbeResult{}, nil).Maybe()

	validator := newTestValidator(WithValidatorPipelineLock(pipelineLockStub))

	sut := testReconcilerWithPipelineLock(fakeClient, flowHealthProberStub, pipelineLockStub, validator)

	_, err := sut.Reconcile(context.Background(), requestFor(pipeline.Name))
	require.NoError(t, err)

	// Verify ConfigMap doesn't contain this pipeline
	var configMap corev1.ConfigMap

	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      names.OTLPGatewayPipelinesSyncConfigMap,
		Namespace: "default",
	}, &configMap)
	if err == nil {
		require.NotContains(t, configMap.Data["pipelines.yaml"], pipeline.Name, "ConfigMap should not contain pipeline that exceeded limit")
	}

	// Verify status
	var updatedPipeline telemetryv1beta1.TracePipeline

	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)
	require.NoError(t, err)

	requireHasStatusCondition(t, &updatedPipeline,
		conditions.TypeConfigurationGenerated,
		metav1.ConditionFalse,
		conditions.ReasonMaxPipelinesExceeded,
		"Maximum pipeline count limit exceeded",
	)

	requireHasStatusCondition(t, &updatedPipeline,
		conditions.TypeFlowHealthy,
		metav1.ConditionFalse,
		conditions.ReasonSelfMonConfigNotGenerated,
		"No spans delivered to backend because TracePipeline specification is not applied to the configuration of OTLP gateway. Check the 'ConfigurationGenerated' condition for more details",
	)

	pipelineLockStub.AssertExpectations(t)
	flowHealthProberStub.AssertExpectations(t)
}

// TestFlowHealthCondition verifies flow health status conditions
func TestFlowHealthCondition(t *testing.T) {
	tests := []struct {
		name            string
		probe           prober.OTelGatewayProbeResult
		probeErr        error
		expectedStatus  metav1.ConditionStatus
		expectedReason  string
		expectedMessage string
	}{
		{
			name:            "prober fails",
			probeErr:        assert.AnError,
			expectedStatus:  metav1.ConditionUnknown,
			expectedReason:  conditions.ReasonSelfMonGatewayProbingFailed,
			expectedMessage: "Could not determine the health of the telemetry flow because the self monitor probing of gateway failed",
		},
		{
			name: "healthy",
			probe: prober.OTelGatewayProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{Healthy: true},
			},
			expectedStatus:  metav1.ConditionTrue,
			expectedReason:  conditions.ReasonSelfMonFlowHealthy,
			expectedMessage: "No problems detected in the telemetry flow",
		},
		{
			name: "throttling",
			probe: prober.OTelGatewayProbeResult{
				Throttling: true,
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonGatewayThrottling,
			expectedMessage: "OTLP gateway is unable to receive spans at current rate. See troubleshooting: " + conditions.LinkGatewayThrottling,
		},
		{
			name: "some data dropped",
			probe: prober.OTelGatewayProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonGatewaySomeDataDropped,
			expectedMessage: "Backend is reachable, but rejecting spans. Some spans are dropped. See troubleshooting: " + conditions.LinkNotAllDataArriveAtBackend,
		},
		{
			name: "all data dropped",
			probe: prober.OTelGatewayProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonSelfMonGatewayAllDataDropped,
			expectedMessage: "Backend is not reachable or rejecting spans. All spans are dropped. See troubleshooting: " + conditions.LinkNoDataArriveAtBackend,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := testutils.NewTracePipelineBuilder().Build()
			fakeClient := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

			flowHealthProberStub := &mocks.FlowHealthProber{}
			flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(tt.probe, tt.probeErr)

			sut := testReconciler(fakeClient, flowHealthProberStub)

			_, err := sut.Reconcile(context.Background(), requestFor(pipeline.Name))
			if tt.probeErr != nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			var updatedPipeline telemetryv1beta1.TracePipeline

			err = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)
			require.NoError(t, err)

			requireHasStatusCondition(t, &updatedPipeline,
				conditions.TypeFlowHealthy,
				tt.expectedStatus,
				tt.expectedReason,
				tt.expectedMessage,
			)

			flowHealthProberStub.AssertExpectations(t)
		})
	}
}

// TestTLSCertificateValidation verifies TLS certificate validation
func TestTLSCertificateValidation(t *testing.T) {
	tests := []struct {
		name                    string
		tlsCertErr              error
		expectedStatus          metav1.ConditionStatus
		expectedReason          string
		expectedMessage         string
		expectGatewayConfigured bool
	}{
		{
			name:            "cert expired",
			tlsCertErr:      &tlscert.CertExpiredError{Expiry: time.Date(2020, time.November, 1, 0, 0, 0, 0, time.UTC)},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonTLSCertificateExpired,
			expectedMessage: "TLS certificate expired on 2020-11-01",
		},
		{
			name:                    "cert about to expire",
			tlsCertErr:              &tlscert.CertAboutToExpireError{Expiry: time.Date(2024, time.November, 1, 0, 0, 0, 0, time.UTC)},
			expectedStatus:          metav1.ConditionTrue,
			expectedReason:          conditions.ReasonTLSCertificateAboutToExpire,
			expectedMessage:         "TLS certificate is about to expire, configured certificate is valid until 2024-11-01",
			expectGatewayConfigured: true,
		},
		{
			name:            "cert decode failed",
			tlsCertErr:      tlscert.ErrCertDecodeFailed,
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  conditions.ReasonTLSConfigurationInvalid,
			expectedMessage: "TLS configuration invalid: failed to decode PEM block containing certificate",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := testutils.NewTracePipelineBuilder().WithOTLPOutput(testutils.OTLPClientMTLSFromString("ca", "fooCert", "fooKey")).Build()
			fakeClient := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

			flowHealthProberStub := &mocks.FlowHealthProber{}
			flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelGatewayProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{Healthy: true},
			}, nil).Maybe()

			opts := []ValidatorOption{
				WithSecretRefValidator(stubs.NewSecretRefValidator(tt.tlsCertErr)),
			}

			sut := testReconcilerWithValidator(fakeClient, flowHealthProberStub, opts...)

			_, err := sut.Reconcile(context.Background(), requestFor(pipeline.Name))
			require.NoError(t, err)

			// Verify ConfigMap state
			var configMap corev1.ConfigMap

			err = fakeClient.Get(context.Background(), types.NamespacedName{
				Name:      names.OTLPGatewayPipelinesSyncConfigMap,
				Namespace: "default",
			}, &configMap)

			if tt.expectGatewayConfigured {
				require.NoError(t, err, "ConfigMap should exist")
				require.Contains(t, configMap.Data["pipelines.yaml"], pipeline.Name, "ConfigMap should contain pipeline reference")
			} else if err == nil {
				require.NotContains(t, configMap.Data["pipelines.yaml"], pipeline.Name, "ConfigMap should not contain invalid pipeline")
			}

			// Verify status
			var updatedPipeline telemetryv1beta1.TracePipeline

			err = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)
			require.NoError(t, err)

			requireHasStatusCondition(t, &updatedPipeline,
				conditions.TypeConfigurationGenerated,
				tt.expectedStatus,
				tt.expectedReason,
				tt.expectedMessage,
			)

			if tt.expectedStatus == metav1.ConditionFalse {
				requireHasStatusCondition(t, &updatedPipeline,
					conditions.TypeFlowHealthy,
					metav1.ConditionFalse,
					conditions.ReasonSelfMonConfigNotGenerated,
					"No spans delivered to backend because TracePipeline specification is not applied to the configuration of OTLP gateway. Check the 'ConfigurationGenerated' condition for more details",
				)
			}

			flowHealthProberStub.AssertExpectations(t)
		})
	}
}

// TestOTTLSpecValidation verifies OTTL specification validation
func TestOTTLSpecValidation(t *testing.T) {
	tests := []struct {
		name             string
		validatorOption  func() ValidatorOption
		expectedErrorMsg string
	}{
		{
			name: "invalid transform spec",
			validatorOption: func() ValidatorOption {
				return WithTransformSpecValidator(stubs.NewTransformSpecValidator(
					&ottl.InvalidOTTLSpecError{Err: fmt.Errorf("invalid TransformSpec: error while parsing statements")},
				))
			},
			expectedErrorMsg: "OTTL specification is invalid, invalid TransformSpec: error while parsing statements. Fix the syntax error indicated by the message or see troubleshooting: " + conditions.LinkOTTLSpecInvalid,
		},
		{
			name: "invalid filter spec",
			validatorOption: func() ValidatorOption {
				return WithFilterSpecValidator(stubs.NewFilterSpecValidator(
					&ottl.InvalidOTTLSpecError{Err: fmt.Errorf("invalid FilterSpec: error while parsing conditions")},
				))
			},
			expectedErrorMsg: "OTTL specification is invalid, invalid FilterSpec: error while parsing conditions. Fix the syntax error indicated by the message or see troubleshooting: " + conditions.LinkOTTLSpecInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := testutils.NewTracePipelineBuilder().Build()
			fakeClient := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

			flowHealthProberStub := &mocks.FlowHealthProber{}
			flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelGatewayProbeResult{}, nil).Maybe()

			sut := testReconcilerWithValidator(fakeClient, flowHealthProberStub, tt.validatorOption())

			_, err := sut.Reconcile(context.Background(), requestFor(pipeline.Name))
			require.NoError(t, err)

			// Verify ConfigMap doesn't contain this pipeline
			var configMap corev1.ConfigMap

			err = fakeClient.Get(context.Background(), types.NamespacedName{
				Name:      names.OTLPGatewayPipelinesSyncConfigMap,
				Namespace: "default",
			}, &configMap)
			if err == nil {
				require.NotContains(t, configMap.Data["pipelines.yaml"], pipeline.Name, "ConfigMap should not contain invalid pipeline")
			}

			// Verify status
			var updatedPipeline telemetryv1beta1.TracePipeline

			err = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)
			require.NoError(t, err)

			requireHasStatusCondition(t, &updatedPipeline,
				conditions.TypeConfigurationGenerated,
				metav1.ConditionFalse,
				conditions.ReasonOTTLSpecInvalid,
				tt.expectedErrorMsg,
			)

			requireHasStatusCondition(t, &updatedPipeline,
				conditions.TypeFlowHealthy,
				metav1.ConditionFalse,
				conditions.ReasonSelfMonConfigNotGenerated,
				"No spans delivered to backend because TracePipeline specification is not applied to the configuration of OTLP gateway. Check the 'ConfigurationGenerated' condition for more details",
			)

			flowHealthProberStub.AssertExpectations(t)
		})
	}
}

// TestAPIServerFailureHandling verifies proper handling of Kubernetes API server errors
func TestAPIServerFailureHandling(t *testing.T) {
	serverErr := errors.New("failed to get lock: server error")

	tests := []struct {
		name            string
		setupPipeline   func() telemetryv1beta1.TracePipeline
		setupClient     func(*testing.T, *telemetryv1beta1.TracePipeline) client.Client
		setupReconciler func(client.Client, *mocks.FlowHealthProber) *Reconciler
	}{
		{
			name: "request to Kubernetes API server failed when validating secret references",
			setupPipeline: func() telemetryv1beta1.TracePipeline {
				return testutils.NewTracePipelineBuilder().WithOTLPOutput(testutils.OTLPEndpointFromSecret(
					"existing",
					"default",
					"endpoint")).Build()
			},
			setupClient: func(t *testing.T, pipeline *telemetryv1beta1.TracePipeline) client.Client {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "existing",
						Namespace: "default",
					},
					Data: map[string][]byte{"endpoint": []byte("http://backend:4317")},
				}

				return fake.NewClientBuilder().WithScheme(testScheme).WithObjects(pipeline, secret).WithStatusSubresource(pipeline).Build()
			},
			setupReconciler: func(fakeClient client.Client, flowHealthProberStub *mocks.FlowHealthProber) *Reconciler {
				opts := []ValidatorOption{
					WithSecretRefValidator(stubs.NewSecretRefValidator(&errortypes.APIRequestFailedError{Err: serverErr})),
				}

				return testReconcilerWithValidator(fakeClient, flowHealthProberStub, opts...)
			},
		},
		{
			name: "request to Kubernetes API server failed when validating max pipeline count limit",
			setupPipeline: func() telemetryv1beta1.TracePipeline {
				return testutils.NewTracePipelineBuilder().WithName("pipeline").Build()
			},
			setupClient: func(t *testing.T, pipeline *telemetryv1beta1.TracePipeline) client.Client {
				return fake.NewClientBuilder().WithScheme(testScheme).WithObjects(pipeline).WithStatusSubresource(pipeline).Build()
			},
			setupReconciler: func(fakeClient client.Client, flowHealthProberStub *mocks.FlowHealthProber) *Reconciler {
				pipelineLockStub := &mocks.PipelineLock{}
				pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
				pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(&errortypes.APIRequestFailedError{Err: serverErr})

				validator := newTestValidator(WithValidatorPipelineLock(pipelineLockStub))

				return testReconcilerWithPipelineLock(fakeClient, flowHealthProberStub, pipelineLockStub, validator)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := tt.setupPipeline()
			fakeClient := tt.setupClient(t, &pipeline)

			flowHealthProberStub := &mocks.FlowHealthProber{}
			flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelGatewayProbeResult{}, nil).Maybe()

			sut := tt.setupReconciler(fakeClient, flowHealthProberStub)

			_, err := sut.Reconcile(context.Background(), requestFor(pipeline.Name))
			require.ErrorIs(t, err, serverErr)

			var updatedPipeline telemetryv1beta1.TracePipeline

			err = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)
			require.NoError(t, err)

			requireHasStatusCondition(t, &updatedPipeline,
				conditions.TypeConfigurationGenerated,
				metav1.ConditionFalse,
				conditions.ReasonValidationFailed,
				"Pipeline validation failed due to an error from the Kubernetes API server",
			)

			requireHasStatusCondition(t, &updatedPipeline,
				conditions.TypeFlowHealthy,
				metav1.ConditionFalse,
				conditions.ReasonSelfMonConfigNotGenerated,
				"No spans delivered to backend because TracePipeline specification is not applied to the configuration of OTLP gateway. Check the 'ConfigurationGenerated' condition for more details",
			)

			flowHealthProberStub.AssertExpectations(t)
		})
	}
}

// TestConfigMapRemoval verifies that non-reconcilable pipelines are removed from ConfigMap
func TestConfigMapRemoval(t *testing.T) {
	pipeline := testutils.NewTracePipelineBuilder().WithOTLPOutput(testutils.OTLPBasicAuthFromSecret("some-secret", "some-namespace", "user", "password")).Build()
	fakeClient := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

	flowHealthProberStub := &mocks.FlowHealthProber{}
	flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelGatewayProbeResult{}, nil).Maybe()

	opts := []ValidatorOption{
		WithSecretRefValidator(stubs.NewSecretRefValidator(fmt.Errorf("%w: Secret 'some-secret' of Namespace 'some-namespace'", secretref.ErrSecretRefNotFound))),
	}

	sut := testReconcilerWithValidator(fakeClient, flowHealthProberStub, opts...)

	_, err := sut.Reconcile(context.Background(), requestFor(pipeline.Name))
	require.NoError(t, err)

	// Verify ConfigMap doesn't contain this pipeline (or doesn't exist)
	var configMap corev1.ConfigMap

	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      names.OTLPGatewayPipelinesSyncConfigMap,
		Namespace: "default",
	}, &configMap)
	if err == nil {
		require.NotContains(t, configMap.Data["pipelines.yaml"], pipeline.Name, "ConfigMap should not contain non-reconcilable pipeline")
	} else {
		// ConfigMap might not exist yet, which is also valid
		require.True(t, apierrors.IsNotFound(err), "Error should be NotFound")
	}

	flowHealthProberStub.AssertExpectations(t)
}

// TestPipelineInfoTracking verifies that pipeline info metrics are tracked correctly
func TestPipelineInfoTracking(t *testing.T) {
	tests := []struct {
		name                 string
		pipeline             telemetryv1beta1.TracePipeline
		secret               *corev1.Secret
		expectedEndpoint     string
		expectedFeatureUsage []string
	}{
		{
			name: "pipeline without features",
			pipeline: testutils.NewTracePipelineBuilder().
				WithName("pipeline-1").
				WithOTLPOutput(testutils.OTLPEndpoint("test")).
				Build(),
			expectedEndpoint:     "test",
			expectedFeatureUsage: []string{},
		},
		{
			name: "pipeline with transform",
			pipeline: testutils.NewTracePipelineBuilder().
				WithName("pipeline-2").
				WithTransform(telemetryv1beta1.TransformSpec{
					Statements: []string{"set(resource.attributes[\"test\"], \"value\")"},
				}).
				WithOTLPOutput(testutils.OTLPEndpoint("test")).
				Build(),
			expectedEndpoint: "test",
			expectedFeatureUsage: []string{
				metrics.FeatureTransform,
			},
		},
		{
			name: "pipeline with filter",
			pipeline: testutils.NewTracePipelineBuilder().
				WithName("pipeline-3").
				WithFilter(telemetryv1beta1.FilterSpec{
					Conditions: []string{"resource.attributes[\"test\"] == \"value\""},
				}).
				WithOTLPOutput(testutils.OTLPEndpoint("test")).
				Build(),
			expectedEndpoint: "test",
			expectedFeatureUsage: []string{
				metrics.FeatureFilter,
			},
		},
		{
			name: "endpoint from secret",
			pipeline: testutils.NewTracePipelineBuilder().
				WithName("pipeline-endpoint-secret").
				WithOTLPOutput(testutils.OTLPEndpointFromSecret("endpoint-secret", "default", "host")).
				Build(),
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "endpoint-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"host": []byte("endpoint.example.com"),
				},
			},
			expectedEndpoint:     "endpoint.example.com",
			expectedFeatureUsage: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objs []client.Object

			objs = append(objs, &tt.pipeline)
			if tt.secret != nil {
				objs = append(objs, tt.secret)
			}

			fakeClient := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(objs...).WithStatusSubresource(&tt.pipeline).Build()

			flowHealthProberStub := &mocks.FlowHealthProber{}
			flowHealthProberStub.On("Probe", mock.Anything, tt.pipeline.Name).Return(prober.OTelGatewayProbeResult{
				PipelineProbeResult: prober.PipelineProbeResult{Healthy: true},
			}, nil).Maybe()

			validator, _ := ottl.NewTransformSpecValidator(ottl.SignalTypeTrace)
			sut := testReconcilerWithValidator(fakeClient, flowHealthProberStub, WithTransformSpecValidator(validator))

			_, err := sut.Reconcile(context.Background(), requestFor(tt.pipeline.Name))
			require.NoError(t, err)

			// Build expected label values
			labelValues := buildTracePipelineLabelValues(tt.pipeline.Name, tt.expectedEndpoint, tt.expectedFeatureUsage)

			metricValue := testutil.ToFloat64(metrics.TracePipelineInfo.WithLabelValues(labelValues...))
			require.Equal(t, float64(1), metricValue, "pipeline info metric should match")

			flowHealthProberStub.AssertExpectations(t)
		})
	}
}

func buildTracePipelineLabelValues(pipelineName, endpoint string, enabledFeatures []string) []string {
	featuresSet := make(map[string]bool, len(enabledFeatures))
	for _, feature := range enabledFeatures {
		featuresSet[feature] = true
	}

	labelValues := []string{pipelineName, endpoint}

	for _, feature := range metrics.TracePipelineFeatures {
		if featuresSet[feature] {
			labelValues = append(labelValues, "true")
		} else {
			labelValues = append(labelValues, "false")
		}
	}

	return labelValues
}

// TestDeletingPipeline verifies that deleting pipelines are properly handled
func TestDeletingPipeline(t *testing.T) {
	now := metav1.Now()
	pipeline := testutils.NewTracePipelineBuilder().Build()
	pipeline.DeletionTimestamp = &now
	pipeline.Finalizers = []string{"telemetry.kyma-project.io/finalizer"} // Add finalizer to allow deletionTimestamp

	fakeClient := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

	flowHealthProberStub := &mocks.FlowHealthProber{}
	flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(prober.OTelGatewayProbeResult{}, nil).Maybe()

	sut := testReconciler(fakeClient, flowHealthProberStub)

	_, err := sut.Reconcile(context.Background(), requestFor(pipeline.Name))
	require.NoError(t, err)

	// Verify ConfigMap doesn't contain the deleting pipeline
	var configMap corev1.ConfigMap

	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      names.OTLPGatewayPipelinesSyncConfigMap,
		Namespace: "default",
	}, &configMap)
	if err == nil {
		if pipelineYaml, ok := configMap.Data["pipelines.yaml"]; ok {
			require.NotContains(t, pipelineYaml, pipeline.Name, "ConfigMap should not contain deleting pipeline")
		}
	}

	flowHealthProberStub.AssertExpectations(t)
}

// TestPipelineNotFound verifies handling of reconcile requests for non-existent pipelines
func TestPipelineNotFound(t *testing.T) {
	fakeClient := fake.NewClientBuilder().WithScheme(testScheme).Build()

	flowHealthProberStub := &mocks.FlowHealthProber{}
	sut := testReconciler(fakeClient, flowHealthProberStub)

	result, err := sut.Reconcile(context.Background(), requestFor("non-existent"))
	require.NoError(t, err)
	require.Zero(t, result.RequeueAfter)
}

// Helper function to create a reconcile request
func requestFor(name string) ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{Name: name},
	}
}

// Helper function to verify status conditions
func requireHasStatusCondition(t *testing.T, pipeline *telemetryv1beta1.TracePipeline,
	condType string,
	status metav1.ConditionStatus,
	reason string,
	message string) {
	cond := getCondition(pipeline, condType)
	require.NotNil(t, cond, "condition %s should exist", condType)
	require.Equal(t, status, cond.Status, "condition %s status should match", condType)
	require.Equal(t, reason, cond.Reason, "condition %s reason should match", condType)
	require.Equal(t, message, cond.Message, "condition %s message should match", condType)
}

func getCondition(pipeline *telemetryv1beta1.TracePipeline, condType string) *metav1.Condition {
	for i := range pipeline.Status.Conditions {
		if pipeline.Status.Conditions[i].Type == condType {
			return &pipeline.Status.Conditions[i]
		}
	}

	return nil
}
