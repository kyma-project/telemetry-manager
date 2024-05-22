package metricpipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/metricpipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestReconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	pipeline := testutils.NewMetricPipelineBuilder().WithName("test").Build()
	fakeClient := fake.NewClientBuilder().
		WithObjects(&pipeline).
		WithStatusSubresource(&pipeline).
		WithScheme(scheme).
		Build()

	overridesHandlerStub := &mocks.OverridesHandler{}
	overridesHandlerStub.On("LoadOverrides", context.Background()).Return(&overrides.Config{}, nil)

	pipelineLockStub := &mocks.PipelineLock{}
	pipelineLockStub.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)
	pipelineLockStub.On("IsLockHolder", mock.Anything, mock.Anything).Return(true, nil)

	gatewayProberStub := &mocks.DeploymentProber{}
	gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

	agentProberStub := &mocks.DeploymentProber{}
	agentProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

	flowHealthProberStub := &mocks.FlowHealthProber{}
	flowHealthProberStub.On("Probe", mock.Anything, mock.Anything).Return(prober.OTelPipelineProbeResult{}, nil)

	tlsCertValidatorStub := &mocks.TLSCertValidator{}
	tlsCertValidatorStub.On("ValidateCertificate", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	istioStatusCheckerStub := &mocks.IstioStatusChecker{}
	istioStatusCheckerStub.On("IsIstioActive", mock.Anything).Return(false)

	sut := Reconciler{
		Client: fakeClient,
		config: Config{
			Gateway: otelcollector.GatewayConfig{
				Config: otelcollector.Config{
					BaseName:  "gateway",
					Namespace: "default",
				},
				Deployment: otelcollector.DeploymentConfig{
					Image: "otel/opentelemetry-collector-contrib",
				},
				OTLPServiceName: "otlp",
			},
			Agent: otelcollector.AgentConfig{
				Config: otelcollector.Config{
					BaseName:  "agent",
					Namespace: "default",
				},
				DaemonSet: otelcollector.DaemonSetConfig{
					Image: "otel/opentelemetry-collector-contrib",
				},
			},
			MaxPipelines: 3,
		},
		pipelineLock:             pipelineLockStub,
		gatewayProber:            gatewayProberStub,
		agentProber:              agentProberStub,
		flowHealthProbingEnabled: false,
		flowHealthProber:         flowHealthProberStub,
		tlsCertValidator:         tlsCertValidatorStub,
		overridesHandler:         overridesHandlerStub,
		istioStatusChecker:       istioStatusCheckerStub,
	}

	_, err := sut.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name: "test",
		},
	})
	require.NoError(t, err)
}
