package upgrade

import (
	"strconv"
	"testing"

	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	"github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

// TestStorageMigration verifies that after the manager starts, the CRD storedVersions
// only contains v1beta1 (i.e., v1alpha1 has been removed).
// This test validates the storage version migration functionality.
func TestStorageMigration(t *testing.T) {
	labels := []string{suite.LabelMisc, suite.LabelTelemetry, suite.LabelUpgrade, suite.LabelNoFIPS}
	suite.SetupTestWithOptions(t, labels,
		kubeprep.WithForceFreshInstall(),
		kubeprep.WithSkipDeployTestPrerequisites(),
		kubeprep.WithChartVersion("https://github.com/kyma-project/telemetry-manager/releases/download/1.55.0/telemetry-manager-1.55.0.tgz"),
	)

	var (
		uniquePrefix = unique.Prefix("migration")
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
	)

	telemetry := operatorv1alpha1.Telemetry{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
			Namespace: "kyma-system",
		},
	}

	Expect(kitk8s.CreateObjects(t, &telemetry)).To(Succeed())

	logbackend, logpipeline := createLogPipelineWithBackend(backendNs, pipelineName)
	metricbackend, metricpipeline := createMetricPipelineWithBackend(backendNs, pipelineName)
	tracebackend, tracepipeline := createTracePipelineWithBackend(backendNs, pipelineName)

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		&logpipeline,
		&metricpipeline,
		&tracepipeline,
	}
	resources = append(resources, logbackend.K8sObjects()...)
	resources = append(resources, metricbackend.K8sObjects()...)
	resources = append(resources, tracebackend.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	verifyStoredVersionsBeforeUpgrade()
	Expect(suite.UpgradeToTargetVersion(t, labels)).To(Succeed())
	verifyStoredVersionsAfterUpgrade()
}

// verifyStoredVersionEquals checks that the given CRD's storedVersions contains the specified version.
func verifyStoredVersionEquals(crdName, expectedVersion string) {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	Expect(suite.K8sClient.Get(suite.Ctx, types.NamespacedName{Name: crdName}, crd)).To(Succeed())

	var storedVersions []string
	for _, version := range crd.Status.StoredVersions {
		storedVersions = append(storedVersions, version)
	}

	Expect(storedVersions).To(ConsistOf(expectedVersion), "CRD %s should have %s in storedVersions", crdName, expectedVersion)
}

// createLogPipelineWithBackend creates a log backend and pipeline configured to send logs to it.
func createLogPipelineWithBackend(backendNs, pipelineName string) (*kitbackend.Backend, telemetryv1alpha1.LogPipeline) {
	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel, kitbackend.WithName("log-backend"))
	pipeline := telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: pipelineName,
		},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.LogPipelineInput{
				Application: &telemetryv1alpha1.LogPipelineApplicationInput{
					Enabled: new(true),
				},
			},
			Output: telemetryv1alpha1.LogPipelineOutput{
				OTLP: &telemetryv1alpha1.OTLPOutput{
					Endpoint: telemetryv1alpha1.ValueType{
						Value: backend.Host() + ":" + strconv.Itoa(int(backend.Port())),
					},
					Protocol: telemetryv1alpha1.OTLPProtocolGRPC,
					TLS: &telemetryv1alpha1.OTLPTLS{
						Insecure:           true,
						InsecureSkipVerify: true,
					},
				},
			},
		},
	}
	return backend, pipeline
}

// createMetricPipelineWithBackend creates a metric backend and pipeline configured to send metrics to it.
func createMetricPipelineWithBackend(backendNs, pipelineName string) (*kitbackend.Backend, telemetryv1alpha1.MetricPipeline) {
	backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName("metric-backend"))
	pipeline := telemetryv1alpha1.MetricPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: pipelineName,
		},
		Spec: telemetryv1alpha1.MetricPipelineSpec{
			Input: telemetryv1alpha1.MetricPipelineInput{
				Runtime: &telemetryv1alpha1.MetricPipelineRuntimeInput{
					Enabled: new(true),
				},
			},
			Output: telemetryv1alpha1.MetricPipelineOutput{
				OTLP: &telemetryv1alpha1.OTLPOutput{
					Endpoint: telemetryv1alpha1.ValueType{
						Value: backend.EndpointHTTP(),
					},
				},
			},
		},
	}
	return backend, pipeline
}

// createTracePipelineWithBackend creates a trace backend and pipeline configured to send traces to it.
func createTracePipelineWithBackend(backendNs, pipelineName string) (*kitbackend.Backend, telemetryv1alpha1.TracePipeline) {
	backend := kitbackend.New(backendNs, kitbackend.SignalTypeTraces, kitbackend.WithName("trace-backend"))
	pipeline := telemetryv1alpha1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: pipelineName,
		},
		Spec: telemetryv1alpha1.TracePipelineSpec{
			Output: telemetryv1alpha1.TracePipelineOutput{
				OTLP: &telemetryv1alpha1.OTLPOutput{
					Endpoint: telemetryv1alpha1.ValueType{
						Value: backend.EndpointHTTP(),
					},
				},
			},
		},
	}
	return backend, pipeline
}

// verifyStoredVersionsBeforeUpgrade verifies that CRD storedVersions contain v1alpha1.
func verifyStoredVersionsBeforeUpgrade() {
	verifyStoredVersionEquals("logpipelines.telemetry.kyma-project.io", "v1alpha1")
	verifyStoredVersionEquals("metricpipelines.telemetry.kyma-project.io", "v1alpha1")
	verifyStoredVersionEquals("tracepipelines.telemetry.kyma-project.io", "v1alpha1")
}

// verifyStoredVersionsAfterUpgrade verifies that CRD storedVersions contain v1beta1.
func verifyStoredVersionsAfterUpgrade() {
	verifyStoredVersionEquals("logpipelines.telemetry.kyma-project.io", "v1beta1")
	verifyStoredVersionEquals("metricpipelines.telemetry.kyma-project.io", "v1beta1")
	verifyStoredVersionEquals("tracepipelines.telemetry.kyma-project.io", "v1beta1")
}

