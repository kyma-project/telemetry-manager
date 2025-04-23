package otel

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	istionetworkingclientv1 "istio.io/client-go/pkg/apis/networking/v1"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))

	utilruntime.Must(operatorv1alpha1.AddToScheme(scheme))
	utilruntime.Must(telemetryv1alpha1.AddToScheme(scheme))
	utilruntime.Must(telemetryv1beta1.AddToScheme(scheme))
	utilruntime.Must(istiosecurityclientv1.AddToScheme(scheme))
	utilruntime.Must(istionetworkingclientv1.AddToScheme(scheme))
}

func TestOTelLogPipeline_ServiceNameEnrichment(t *testing.T) {
	RegisterTestingT(t)
	skipIfDoesNotMatchLabel(t, "logs")

	const (
		appLabelName  = "app"
		appLabelValue = "workload"
	)

	tests := []struct {
		name                 string
		logPipelineInputFunc func(includeNs []string, excludeNs []string) telemetryv1alpha1.LogPipelineInput
		logProducerFunc      func(deploymentName, namespace string) client.Object
	}{
		{
			name: "otlp",
			logPipelineInputFunc: func(includeNs []string, excludeNs []string) telemetryv1alpha1.LogPipelineInput {
				return telemetryv1alpha1.LogPipelineInput{
					Application: &telemetryv1alpha1.LogPipelineApplicationInput{Enabled: ptr.To(false)},
					OTLP:        &telemetryv1alpha1.OTLPInput{Disabled: false},
				}
			},
			logProducerFunc: func(deploymentName, namespace string) client.Object {
				podSpecWithUndefinedService := telemetrygen.PodSpec(telemetrygen.SignalTypeLogs, telemetrygen.WithServiceName(""))
				return kitk8s.NewDeployment(deploymentName, namespace).
					WithLabel(appLabelName, appLabelValue).
					WithPodSpec(podSpecWithUndefinedService).
					K8sObject()
			},
		},
		{
			name: "application",
			logPipelineInputFunc: func(includeNs []string, excludeNs []string) telemetryv1alpha1.LogPipelineInput {
				return telemetryv1alpha1.LogPipelineInput{
					Application: &telemetryv1alpha1.LogPipelineApplicationInput{Enabled: ptr.To(true)},
					OTLP:        &telemetryv1alpha1.OTLPInput{Disabled: true},
				}
			},
			logProducerFunc: func(deploymentName, namespace string) client.Object {
				return loggen.New(namespace).
					WithName(deploymentName).
					WithLabels(map[string]string{
						appLabelName: appLabelValue,
					}).
					K8sObject()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				mockNs       = suite.IDWithSuffix(tc.name + "-mocks")
				genName      = suite.IDWithSuffix(tc.name + "-gen")
				pipelineName = suite.IDWithSuffix(tc.name + "-pipeline")
			)

			backend := backend.New(mockNs, backend.SignalTypeLogsOtel)

			hostSecretRef := backend.HostSecretRefV1Alpha1()
			pipeline := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.logPipelineInputFunc(nil, nil)).
				WithOTLPOutput(
					testutils.OTLPEndpointFromSecret(
						hostSecretRef.Name,
						hostSecretRef.Namespace,
						hostSecretRef.Key,
					),
				).
				Build()

			var resources []client.Object
			resources = append(resources,
				kitk8s.NewNamespace(mockNs).K8sObject(),
				&pipeline,
				tc.logProducerFunc(genName, mockNs),
			)
			resources = append(resources, backend.K8sObjects()...)

			k8sClient, proxyClient := initializeClients(t)

			t.Cleanup(func() {
				// Cannot use t.Context() here because it is already canceled at this point
				err := kitk8s.DeleteObjects(context.Background(), k8sClient, resources...)
				require.NoError(t, err)
			})
			Expect(kitk8s.CreateObjects(t.Context(), k8sClient, resources...)).Should(Succeed())

			t.Log("Waiting for resources to be ready")

			assert.DeploymentReady(t.Context(), k8sClient, kitkyma.LogGatewayName)
			assert.DeploymentReady(t.Context(), k8sClient, types.NamespacedName{Name: backend.Name(), Namespace: mockNs})
			assert.LogPipelineOtelHealthy(t.Context(), k8sClient, pipelineName)
			assert.LogsFromNamespaceDelivered(proxyClient, backend.ExportURL(proxyClient), mockNs)

			verifyServiceNameAttr(proxyClient, backend.ExportURL(proxyClient), genName, appLabelValue)
		})
	}
}

func skipIfDoesNotMatchLabel(t *testing.T, label string) {
	args := os.Args
	idx := slices.IndexFunc(args, func(arg string) bool {
		return strings.HasPrefix(arg, "label")
	})

	if idx == -1 {
		return
	}

	labelArg := args[idx]
	if parts := strings.Split(labelArg, "="); len(parts) == 2 {
		labelVal := parts[1]
		if labelVal != label {
			t.Skipf("Skipping test: label mismatch. Expected: %s, Got: %s", label, labelVal)
		}
	}
}

func initializeClients(t *testing.T) (client.Client, *apiserverproxy.Client) {
	t.Helper()
	t.Log("Initializing clients")

	kubeconfig := clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())

	proxyClient, err := apiserverproxy.NewClient(cfg)
	Expect(err).NotTo(HaveOccurred())

	return k8sClient, proxyClient
}

func verifyServiceNameAttr(proxyClient *apiserverproxy.Client, backendExportURL, givenPodPrefix, expectedServiceName string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(backendExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		bodyContent, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(bodyContent).To(HaveFlatOtelLogs(
			ContainElement(SatisfyAll(
				HaveResourceAttributes(HaveKeyWithValue("service.name", expectedServiceName)),
				HaveResourceAttributes(HaveKeyWithValue("k8s.pod.name", ContainSubstring(givenPodPrefix))),
			)),
		))
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed(), fmt.Sprintf("could not find logs matching service.name: %s, k8s.pod.name: %s.*", expectedServiceName, givenPodPrefix))
}
