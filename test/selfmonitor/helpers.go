package selfmonitor

import (
	"net/http"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/faultbackend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdoutloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

const (
	defaultRate = 100

	faultPercentageAll        float64 = 100
	faultPercentageHalf       float64 = 50
	faultPercentageNinetyFive float64 = 95
)

// HTTP status codes used for fault injection.
//
// Retryability differs between OTel Collector and Fluent Bit (summarized for tests; exact
// sets follow the collector/Fluent Bit versions wired into Kyma — see exporter and output plugin docs if in doubt):
//
//	OTel Collector retryable codes: 429, 502, 503, 504
//	OTel Collector non-retryable codes: everything else, including 400 and 500
//
//	Fluent Bit retryable codes: 408, 429, 5xx
//	Fluent Bit non-retryable codes: 4xx (except 408 and 429)
//
// HTTP 400 is non-retryable for both OTel Collector and Fluent Bit, so it is used
// as the universal non-retryable status code.
// HTTP 429 is retryable for both, so it is used as the universal retryable status code.
//
// Fault backends use the mock-backend container (test/testkit/mocks/faultbackend) to return
// these codes at configurable percentages: faultNonRetryableErr → 400, faultRetryableErr → 429.
const (
	retryableErrCode    = http.StatusTooManyRequests
	nonRetryableErrCode = http.StatusBadRequest
)

// faultNonRetryableErr returns FaultBackend options that reject requests with HTTP 400 at the given percentage.
// HTTP 400 is non-retryable for both OTel Collector and Fluent Bit, so rejected data is
// dropped immediately without retry.
func faultNonRetryableErr(percentage float64) []faultbackend.Option {
	return []faultbackend.Option{faultbackend.WithStatusCodeAndPercentage(nonRetryableErrCode, percentage)}
}

func faultRetryableErr(percentage float64) []faultbackend.Option {
	return []faultbackend.Option{faultbackend.WithStatusCodeAndPercentage(retryableErrCode, percentage)}
}

// backendNoEndpoints runs the mock backend Deployment with zero replicas so the Service has no ready endpoints.
// Fluent Bit keeps reading logs while output cannot complete (no HTTP response), which tends to surface
// ReasonSelfMonAgentNoLogsDelivered rather than immediate non-retryable drops (ReasonSelfMonAgentAllDataDropped).
func backendNoEndpoints() []kitbackend.Option {
	return []kitbackend.Option{kitbackend.WithReplicas(0)}
}

// pipelineBackend is satisfied by both *backend.Backend and *faultbackend.FaultBackend,
// allowing buildPipeline to work with either.
type pipelineBackend interface {
	EndpointHTTP() string
	Host() string
	Port() int32
}

func buildPipeline(component, pipelineName, includeNs string, backend pipelineBackend) client.Object {
	switch component {
	case suite.LabelLogAgent:
		p := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithInput(testutils.BuildLogPipelineRuntimeInput(testutils.IncludeNamespaces(includeNs))).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
			Build()

		return &p

	case suite.LabelLogGateway:
		p := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithInput(testutils.BuildLogPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
			Build()

		return &p

	case suite.LabelFluentBit:
		p := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithRuntimeInput(true, testutils.IncludeNamespaces(includeNs)).
			WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
			Build()

		return &p

	case suite.LabelMetricGateway:
		p := testutils.NewMetricPipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
			Build()

		return &p

	case suite.LabelMetricAgent:
		p := testutils.NewMetricPipelineBuilder().
			WithName(pipelineName).
			WithPrometheusInput(true, testutils.IncludeNamespaces(includeNs)).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
			Build()

		return &p

	case suite.LabelTraces:
		p := testutils.NewTracePipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
			Build()

		return &p

	default:
		panic("unknown component: " + component)
	}
}

// Generator factories

func stdoutLogGenerator(rate int) func(ns string) []client.Object {
	return func(ns string) []client.Object {
		return []client.Object{stdoutloggen.NewDeployment(ns, stdoutloggen.WithRate(rate)).K8sObject()}
	}
}

func stdoutLogGeneratorDefault() func(ns string) []client.Object {
	return func(ns string) []client.Object {
		return []client.Object{stdoutloggen.NewDeployment(ns).K8sObject()}
	}
}

func otelGenerator(signalType telemetrygen.SignalType, opts ...telemetrygen.Option) func(ns string) []client.Object {
	return func(ns string) []client.Object {
		return []client.Object{telemetrygen.NewDeployment(ns, signalType, opts...).K8sObject()}
	}
}

func promMetricGenerator() func(ns string) []client.Object {
	return func(ns string) []client.Object {
		metricProducer := prommetricgen.New(ns)

		return []client.Object{
			metricProducer.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
			metricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
		}
	}
}

func promMetricGeneratorHighLoad() func(ns string) []client.Object {
	return func(ns string) []client.Object {
		metricProducer := prommetricgen.New(ns)

		return []client.Object{
			metricProducer.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).WithAvalancheHighLoad().K8sObject(),
			metricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
		}
	}
}

// Assertion helpers

func componentConditionType(component string) string {
	switch component {
	case suite.LabelLogAgent, suite.LabelLogGateway, suite.LabelFluentBit:
		return conditions.TypeLogComponentsHealthy
	case suite.LabelMetricAgent, suite.LabelMetricGateway:
		return conditions.TypeMetricComponentsHealthy
	case suite.LabelTraces:
		return conditions.TypeTraceComponentsHealthy
	default:
		panic("unknown component: " + component)
	}
}

func assertComponentReady(t *testing.T, component string) {
	t.Helper()

	switch component {
	case suite.LabelLogAgent:
		assert.DeploymentReady(t, kitkyma.LogGatewayName)
		assert.DaemonSetReady(t, kitkyma.LogAgentName)
	case suite.LabelLogGateway:
		assert.DeploymentReady(t, kitkyma.LogGatewayName)
	case suite.LabelFluentBit:
		assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
	case suite.LabelMetricGateway:
		assert.DeploymentReady(t, kitkyma.MetricGatewayName)
	case suite.LabelMetricAgent:
		assert.DeploymentReady(t, kitkyma.MetricGatewayName)
		assert.DaemonSetReady(t, kitkyma.MetricAgentName)
	case suite.LabelTraces:
		assert.DeploymentReady(t, kitkyma.TraceGatewayName)
	default:
		panic("unknown component: " + component)
	}
}

func assertPipelineHealthy(t *testing.T, component, pipelineName string) {
	t.Helper()

	switch component {
	case suite.LabelLogAgent, suite.LabelLogGateway:
		assert.OTelLogPipelineHealthy(t, pipelineName)
	case suite.LabelFluentBit:
		assert.FluentBitLogPipelineHealthy(t, pipelineName)
	case suite.LabelMetricGateway, suite.LabelMetricAgent:
		assert.MetricPipelineHealthy(t, pipelineName)
	case suite.LabelTraces:
		assert.TracePipelineHealthy(t, pipelineName)
	default:
		panic("unknown component: " + component)
	}
}

func assertPipelineConditionTransition(t *testing.T, component, pipelineName string, expectedReasons []assert.ReasonStatus) {
	t.Helper()

	switch component {
	case suite.LabelLogAgent, suite.LabelLogGateway, suite.LabelFluentBit:
		assert.LogPipelineConditionReasonsTransition(t, pipelineName, conditions.TypeFlowHealthy, expectedReasons)
	case suite.LabelMetricGateway, suite.LabelMetricAgent:
		assert.MetricPipelineConditionReasonsTransition(t, pipelineName, conditions.TypeFlowHealthy, expectedReasons)
	case suite.LabelTraces:
		assert.TracePipelineConditionReasonsTransition(t, pipelineName, conditions.TypeFlowHealthy, expectedReasons)
	default:
		panic("unknown component: " + component)
	}
}

// assertFlowDegraded runs the standard assertion sequence for backpressure/outage tests:
// component readiness, pipeline health, condition transition, and telemetry warning state.
func assertFlowDegraded(t *testing.T, component, pipelineName string, expectedReasons []assert.ReasonStatus) {
	t.Helper()

	assertComponentReady(t, component)
	assertPipelineHealthy(t, component, pipelineName)
	assertPipelineConditionTransition(t, component, pipelineName, expectedReasons)

	finalReason := expectedReasons[len(expectedReasons)-1].Reason

	assert.TelemetryHasState(t, operatorv1beta1.StateWarning)
	assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
		Type:   componentConditionType(component),
		Status: metav1.ConditionFalse,
		Reason: finalReason,
	})
}

// flowHealthyThenDegraded builds the standard transition: FlowHealthy(true) → reason(false) [→ reason(false) ...]
func flowHealthyThenDegraded(reasons ...string) []assert.ReasonStatus {
	result := []assert.ReasonStatus{
		{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
	}
	for _, r := range reasons {
		result = append(result, assert.ReasonStatus{Reason: r, Status: metav1.ConditionFalse})
	}

	return result
}
