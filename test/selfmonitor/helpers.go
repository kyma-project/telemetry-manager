package selfmonitor

import (
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/faultbackend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdoutloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

const (
	defaultRate = 1000

	faultPercentageAll         float64 = 100
	faultPercentageThirty      float64 = 30
	faultPercentageNinetyEight float64 = 98
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
// Fault backends use the fault-backend container (test/testkit/mocks/faultbackend) to return
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

// defaultGenerator returns the standard telemetry generator for a component at defaultRate.
// Each component type uses the generator matching its pipeline input:
//   - log-agent / fluent-bit: stdout log generator (runtime input reads container logs)
//   - log-gateway: telemetrygen sending OTLP logs
//   - metric-gateway: telemetrygen sending OTLP metrics
//   - metric-agent: Prometheus metric endpoint (scraped by the agent)
//   - traces: telemetrygen sending OTLP traces
//
// The defaultRate (100) is sufficient for outage/backpressure tests that are driven by fault
// percentage: even a moderate flow will trigger the self-monitor condition when the fault
// fraction is high enough. Tests that require higher throughput (e.g. fluent-bit buffer filling)
// override the generator explicitly in the test table via tc.generator.
func defaultGenerator(component string) func(ns string) []client.Object {
	switch component {
	case suite.LabelLogAgent, suite.LabelFluentBit:
		return stdoutLogGenerator(defaultRate)
	case suite.LabelLogGateway:
		return otelGenerator(telemetrygen.SignalTypeLogs)
	case suite.LabelMetricGateway:
		return otelGenerator(telemetrygen.SignalTypeMetrics)
	case suite.LabelMetricAgent:
		return promMetricGenerator()
	case suite.LabelTraces:
		return otelGenerator(telemetrygen.SignalTypeTraces)
	default:
		panic("unknown component: " + component)
	}
}

func stdoutLogGenerator(rate int) func(ns string) []client.Object {
	return func(ns string) []client.Object {
		return []client.Object{stdoutloggen.NewDeployment(ns, stdoutloggen.WithRate(rate)).K8sObject()}
	}
}

func otelGenerator(signalType telemetrygen.SignalType, opts ...telemetrygen.Option) func(ns string) []client.Object {
	return func(ns string) []client.Object {
		allOpts := []telemetrygen.Option{telemetrygen.WithRate(defaultRate), telemetrygen.WithWorkers(1)}
		allOpts = append(allOpts, opts...)

		return []client.Object{telemetrygen.NewDeployment(ns, signalType, allOpts...).K8sObject()}
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

// promMetricGeneratorHighLoad produces high-volume Prometheus metrics via the Avalanche load generator.
// Required for metric-agent fault-injection tests (backpressure/outage): the self-monitor rules fire
// only when both drop and sent rates are non-zero, which needs enough throughput to stay above zero
// in the Prometheus scrape window even when a large fraction of exports are faulted.
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
		assert.LogPipelineSelfMonitorIsHealthy(t, suite.K8sClient, pipelineName)
	case suite.LabelFluentBit:
		assert.FluentBitLogPipelineHealthy(t, pipelineName)
		assert.LogPipelineSelfMonitorIsHealthy(t, suite.K8sClient, pipelineName)
	case suite.LabelMetricGateway, suite.LabelMetricAgent:
		assert.MetricPipelineHealthy(t, pipelineName)
		assert.MetricPipelineSelfMonitorIsHealthy(t, suite.K8sClient, pipelineName)
	case suite.LabelTraces:
		assert.TracePipelineHealthy(t, pipelineName)
		assert.TracePipelineSelfMonitorIsHealthy(t, suite.K8sClient, pipelineName)
	default:
		panic("unknown component: " + component)
	}

	// Wait until Prometheus has enough scrape samples for rate() to return non-zero.
	// FlowHealthy=True confirms data is flowing at the CR level, but rate([5m]) queries
	// need at least 2 scrape samples in the window (scrape interval = 30s). If a pod
	// restarted recently, new series may have only 1 sample and rate() returns (no data).
	// Waiting here ensures the fault baseline is clean before faults are enabled.
	t.Log("Waiting for self-monitor rate metrics to be non-zero before enabling faults")
	assertSelfMonitorRateNonZero(t, component)
}

// assertSelfMonitorRateNonZero waits until at least one of the component's rate queries
// returns a non-zero value, confirming that Prometheus has sufficient scrape history.
func assertSelfMonitorRateNonZero(t *testing.T, component string) {
	
	t.Helper()

	Eventually(func() bool {
		for _, query := range metricsForComponent(component) {
			val, err := queryPrometheus(t.Context(), query)
			if err != nil || val == "(no data)" {
				t.Logf("rate baseline not ready [%s]: %v", query, err)
				continue
			}

			if val != "" {
				t.Logf("rate baseline ready [%s]: %s", query, val)
				return true
			}
		}

		return false
	}, periodic.EventuallyTimeout, periodic.SelfmonitorQueryInterval).Should(
		BeTrue(),
		"self-monitor rate metrics never became non-zero for component %s", component,
	)
}

func assertPipelineConditionTransition(t *testing.T, component, pipelineName string, expectedReasons []assert.ReasonStatus) {
	t.Helper()

	condType := conditions.TypeFlowHealthy
	key := types.NamespacedName{Name: pipelineName}

	var currCond *metav1.Condition

	for _, exp := range expectedReasons {
		t.Logf("Waiting for condition %s[%s] (%s) — watching self-monitor metrics for component %s",
			exp.Reason, exp.Status, alertConditionDescription(exp.Reason, component), component)

		Eventually(func(g Gomega) assert.ReasonStatus {
			logSelfMonitorMetrics(t, component)

			switch component {
			case suite.LabelLogAgent, suite.LabelLogGateway, suite.LabelFluentBit:
				var pipeline telemetryv1beta1.LogPipeline
				g.Expect(suite.K8sClient.Get(t.Context(), key, &pipeline)).To(Succeed())
				currCond = meta.FindStatusCondition(pipeline.Status.Conditions, condType)
			case suite.LabelMetricGateway, suite.LabelMetricAgent:
				var pipeline telemetryv1beta1.MetricPipeline
				g.Expect(suite.K8sClient.Get(t.Context(), key, &pipeline)).To(Succeed())
				currCond = meta.FindStatusCondition(pipeline.Status.Conditions, condType)
			case suite.LabelTraces:
				var pipeline telemetryv1beta1.TracePipeline
				g.Expect(suite.K8sClient.Get(t.Context(), key, &pipeline)).To(Succeed())
				currCond = meta.FindStatusCondition(pipeline.Status.Conditions, condType)
			default:
				panic("unknown component: " + component)
			}

			if currCond == nil {
				return assert.ReasonStatus{}
			}

			return assert.ReasonStatus{Reason: currCond.Reason, Status: currCond.Status}
		}, periodic.FlowHealthConditionTransitionTimeout, periodic.SelfmonitorQueryInterval).Should(
			Equal(exp),
			"expected reason %s[%s] of type %s not reached", exp.Reason, exp.Status, condType,
		)

		t.Logf("Transitioned to [%s]%s\n", currCond.Status, currCond.Reason)
	}
}

// assertFlowDegraded runs the standard assertion sequence for backpressure/outage tests:
// condition transition and telemetry warning state. Call assertComponentReady and
// assertPipelineHealthy before enabling faults, then call this function.
func assertFlowDegraded(t *testing.T, component, pipelineName string, expectedReasons []assert.ReasonStatus) {
	t.Helper()

	assertPipelineConditionTransition(t, component, pipelineName, expectedReasons)

	finalReason := expectedReasons[len(expectedReasons)-1].Reason

	assert.TelemetryHasState(t, operatorv1beta1.StateWarning)
	assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
		Type:   componentConditionType(component),
		Status: metav1.ConditionFalse,
		Reason: finalReason,
	})
}

// degradedReasons builds the expected condition transition sequence: each reason with status False.
func degradedReasons(reasons ...string) []assert.ReasonStatus {
	result := []assert.ReasonStatus{}
	for _, r := range reasons {
		result = append(result, assert.ReasonStatus{Reason: r, Status: metav1.ConditionFalse})
	}

	return result
}
