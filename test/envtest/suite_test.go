package envtest

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	commonStatusStubs "github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus/stubs"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline/stubs"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	"github.com/kyma-project/telemetry-manager/internal/validators/endpoint"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
	tracepipelinewebhookv1beta1 "github.com/kyma-project/telemetry-manager/webhook/tracepipeline/v1beta1"
)

// testEnvFixture holds the envtest resources for a single test.
type testEnvFixture struct {
	client client.Client
	ctx    context.Context //nolint:containedctx // test fixture intentionally carries the test-scoped context
}

func testScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()

	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1beta1.AddToScheme(scheme)
	_ = operatorv1beta1.AddToScheme(scheme)

	return scheme
}

func projectRoot() string {
	root, err := filepath.Abs("../..")
	if err != nil {
		panic(err)
	}

	return root
}

func crdPaths() []string {
	return []string{
		filepath.Join(projectRoot(), "helm", "charts", "default", "templates"),
	}
}

func webhookPaths() []string {
	return []string{
		filepath.Join(projectRoot(), "test", "envtest", "testdata"),
	}
}

// setupWebhookOnly starts envtest with CRDs and webhook server, but no controller.
// Use this for Tier 1 tests that only validate admission webhook behavior.
func setupWebhookOnly(t *testing.T) testEnvFixture {
	t.Helper()

	logf.SetLogger(logzap.New(logzap.UseDevMode(true)))

	scheme := testScheme()

	env := newEnvtestEnv(scheme)

	cfg, err := env.Start()
	require.NoError(t, err)

	testCtx, testCancel := context.WithCancel(context.Background())

	mgr, err := newManager(cfg, scheme, env)
	require.NoError(t, err)

	require.NoError(t, tracepipelinewebhookv1beta1.SetupWithManager(mgr))

	go func() {
		if err := mgr.Start(testCtx); err != nil {
			t.Logf("manager exited: %v", err)
		}
	}()

	waitForWebhookServer(t, env)

	t.Cleanup(func() {
		testCancel()
		require.NoError(t, env.Stop())
	})

	return testEnvFixture{
		client: mgr.GetClient(),
		ctx:    testCtx,
	}
}

// setupWithController starts envtest with CRDs, webhook server, AND the TracePipeline controller.
// Use this for Tier 2 tests that need the reconciliation loop running.
func setupWithController(t *testing.T) testEnvFixture {
	t.Helper()

	logf.SetLogger(logzap.New(logzap.UseDevMode(true)))

	scheme := testScheme()
	env := newEnvtestEnv(scheme)

	cfg, err := env.Start()
	require.NoError(t, err)

	testCtx, testCancel := context.WithCancel(context.Background())

	mgr, err := newManager(cfg, scheme, env)
	require.NoError(t, err)

	require.NoError(t, tracepipelinewebhookv1beta1.SetupWithManager(mgr))

	reconciler := newMockedReconciler(mgr.GetClient())
	require.NoError(t, ctrl.NewControllerManagedBy(mgr).
		For(&telemetryv1beta1.TracePipeline{}).
		Complete(reconciler))

	go func() {
		if err := mgr.Start(testCtx); err != nil {
			t.Logf("manager exited: %v", err)
		}
	}()

	waitForWebhookServer(t, env)

	t.Cleanup(func() {
		testCancel()
		require.NoError(t, env.Stop())
	})

	return testEnvFixture{
		client: mgr.GetClient(),
		ctx:    testCtx,
	}
}

func newEnvtestEnv(scheme *runtime.Scheme) *envtest.Environment {
	return &envtest.Environment{
		CRDDirectoryPaths: crdPaths(),
		Scheme:            scheme,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: webhookPaths(),
		},
	}
}

func newManager(cfg *rest.Config, scheme *runtime.Scheme, env *envtest.Environment) (ctrl.Manager, error) {
	webhookOpts := env.WebhookInstallOptions

	return ctrl.NewManager(cfg, ctrl.Options{
		Scheme:  scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    webhookOpts.LocalServingHost,
			Port:    webhookOpts.LocalServingPort,
			CertDir: webhookOpts.LocalServingCertDir,
		}),
	})
}

func waitForWebhookServer(t *testing.T, env *envtest.Environment) {
	t.Helper()

	opts := env.WebhookInstallOptions
	addr := fmt.Sprintf("%s:%d", opts.LocalServingHost, opts.LocalServingPort)
	dialer := &net.Dialer{Timeout: time.Second}

	tlsDialer := &tls.Dialer{
		NetDialer: dialer,
		Config:    &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // test-only
	}

	require.Eventually(t, func() bool {
		conn, err := tlsDialer.DialContext(context.Background(), "tcp", addr)
		if err != nil {
			return false
		}

		_ = conn.Close()

		return true
	}, 10*time.Second, 200*time.Millisecond, "webhook server did not become ready")
}

// newMockedReconciler creates a TracePipeline reconciler with all data-plane dependencies mocked.
// The real validation logic (endpoint, TLS, secret ref) runs against the envtest API server.
func newMockedReconciler(c client.Client) *reconcilerAdapter {
	gatewayConfigBuilder := &mocks.GatewayConfigBuilder{}
	gatewayConfigBuilder.On("Build", mock.Anything, mock.Anything, mock.Anything).Return(&common.Config{}, nil, nil).Maybe()

	gatewayApplierDeleter := &mocks.GatewayApplierDeleter{}
	gatewayApplierDeleter.On("ApplyResources", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	gatewayApplierDeleter.On("DeleteResources", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	flowHealthProber := &mocks.FlowHealthProber{}
	flowHealthProber.On("Probe", mock.Anything, mock.Anything).Return(prober.OTelGatewayProbeResult{}, nil).Maybe()

	overridesHandler := &mocks.OverridesHandler{}
	overridesHandler.On("LoadOverrides", mock.Anything).Return(&overrides.Config{}, nil).Maybe()

	pipelineLock := &mocks.PipelineLock{}
	pipelineLock.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil).Maybe()
	pipelineLock.On("IsLockHolder", mock.Anything, mock.Anything).Return(nil).Maybe()

	pipelineSyncer := &mocks.PipelineSyncer{}
	pipelineSyncer.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil).Maybe()

	transformSpecValidator, _ := ottl.NewTransformSpecValidator(ottl.SignalTypeTrace)
	filterSpecValidator, _ := ottl.NewFilterSpecValidator(ottl.SignalTypeTrace)

	pipelineValidator := tracepipeline.NewValidator(
		tracepipeline.WithEndpointValidator(&endpoint.Validator{Client: c}),
		tracepipeline.WithTLSCertValidator(tlscert.New(c)),
		tracepipeline.WithSecretRefValidator(&secretref.Validator{Client: c}),
		tracepipeline.WithValidatorPipelineLock(pipelineLock),
		tracepipeline.WithTransformSpecValidator(transformSpecValidator),
		tracepipeline.WithFilterSpecValidator(filterSpecValidator),
	)

	r := tracepipeline.New(
		tracepipeline.WithClient(c),
		tracepipeline.WithGlobals(config.NewGlobal(config.WithTargetNamespace("default"))),
		tracepipeline.WithGatewayConfigBuilder(gatewayConfigBuilder),
		tracepipeline.WithGatewayApplierDeleter(gatewayApplierDeleter),
		tracepipeline.WithGatewayProber(commonStatusStubs.NewDeploymentSetProber(nil)),
		tracepipeline.WithFlowHealthProber(flowHealthProber),
		tracepipeline.WithIstioStatusChecker(&stubs.IstioStatusChecker{IsActive: false}),
		tracepipeline.WithOverridesHandler(overridesHandler),
		tracepipeline.WithPipelineLock(pipelineLock),
		tracepipeline.WithPipelineSyncer(pipelineSyncer),
		tracepipeline.WithPipelineValidator(pipelineValidator),
		tracepipeline.WithErrorToMessageConverter(&conditions.ErrorToMessageConverter{}),
		tracepipeline.WithSecretWatcher(stubs.NewSecretWatcher(nil)),
	)

	return &reconcilerAdapter{inner: r}
}

type reconcilerAdapter struct {
	inner *tracepipeline.Reconciler
}

func (a *reconcilerAdapter) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return a.inner.Reconcile(ctx, req)
}
