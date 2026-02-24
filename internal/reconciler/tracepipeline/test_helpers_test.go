package tracepipeline

import (
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline/stubs"
)

var testScheme = newTestScheme()

func newTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1beta1.AddToScheme(scheme)
	return scheme
}

// testReconciler creates a basic reconciler for testing with default configuration
func testReconciler(fakeClient client.Client, flowHealthProber FlowHealthProber) *Reconciler {
	return testReconcilerWithValidator(fakeClient, flowHealthProber)
}

// testReconcilerWithValidator creates a reconciler with custom validator options
func testReconcilerWithValidator(fakeClient client.Client, flowHealthProber FlowHealthProber, validatorOpts ...ValidatorOption) *Reconciler {
	validator := newTestValidator(validatorOpts...)
	return testReconcilerWithPipelineLock(fakeClient, flowHealthProber, stubs.NewPipelineLock(), validator)
}

// testReconcilerWithPipelineLock creates a reconciler with custom pipeline lock
func testReconcilerWithPipelineLock(fakeClient client.Client, flowHealthProber FlowHealthProber, pipelineLock PipelineLock, validator *Validator) *Reconciler {
	cfg := config.NewGlobal(config.WithTargetNamespace("default"))

	overridesHandler := &stubs.OverridesHandler{}
	pipelineSync := &stubs.PipelineSync{}
	errToMsgConverter := &conditions.ErrorToMessageConverter{}

	return New(
		WithClient(fakeClient),
		WithGlobals(cfg),
		WithFlowHealthProber(flowHealthProber),
		WithOverridesHandler(overridesHandler),
		WithPipelineLock(pipelineLock),
		WithPipelineSyncer(pipelineSync),
		WithPipelineValidator(validator),
		WithErrorToMessageConverter(errToMsgConverter),
	)
}

// newTestValidator creates a validator with the provided options or defaults
func newTestValidator(opts ...ValidatorOption) *Validator {
	v := &Validator{
		SecretRefValidator:     stubs.NewSecretRefValidator(nil),
		TLSCertValidator:       stubs.NewTLSCertValidator(nil),
		FilterSpecValidator:    stubs.NewFilterSpecValidator(nil),
		TransformSpecValidator: stubs.NewTransformSpecValidator(nil),
		EndpointValidator:      stubs.NewEndpointValidator(nil),
		PipelineLock:           stubs.NewPipelineLock(),
	}

	for _, opt := range opts {
		opt(v)
	}

	return v
}
