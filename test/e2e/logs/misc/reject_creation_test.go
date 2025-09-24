package misc

import (
	"log"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestRejectLogPipelineCreation(t *testing.T) {
	var (
		label     = suite.LabelMisc
		backendNs = "backend"
	)
	suite.RegisterTestCase(t, label)

	serverCerts, clientCerts, err := testutils.NewCertBuilder(kitbackend.DefaultName, backendNs).Build()
	Expect(err).ToNot(HaveOccurred())

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel, kitbackend.WithTLS(*serverCerts))

	tests := []struct {
		pipeline telemetryv1alpha1.LogPipeline
		errorMsg string
	}{
		// output general
		{
			pipeline: telemetryv1alpha1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "no-output",
				},
				Spec: telemetryv1alpha1.LogPipelineSpec{},
			},
			errorMsg: "spec.output in body should have at least 1 properties",
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("multiple-outputs").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backend.Endpoint()),
					testutils.OTLPEndpointPath("/v1/mock/metrics"),
				).
				WithHTTPOutput().
				Build(),
			errorMsg: "spec.output: Too many: 2: must have at most 1 items",
		},
		// otlp output
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("otlp-output-with-default-proto-and-path").
				WithApplicationInput(false).
				WithOTLPOutput(
					testutils.OTLPEndpoint(backend.Endpoint()),
					testutils.OTLPEndpointPath("/v1/mock/metrics"),
				).
				Build(),
			errorMsg: "Path is only available with HTTP protocol",
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("otlp-output-with-grpc-proto-and-path").
				WithApplicationInput(false).
				WithOTLPOutput(
					testutils.OTLPEndpoint(backend.Endpoint()),
					testutils.OTLPEndpointPath("/v1/mock/metrics"),
					testutils.OTLPProtocol("grpc"),
				).
				Build(),
			errorMsg: "Path is only available with HTTP protocol",
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("otlp-output-with-non-valid-proto").
				WithApplicationInput(false).
				WithOTLPOutput(
					testutils.OTLPEndpoint(backend.Endpoint()),
					testutils.OTLPProtocol("icke"),
				).
				Build(),
			errorMsg: "spec.output.otlp.protocol: Unsupported value",
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("otlp-output-without-endpoint").
				WithApplicationInput(false).
				WithOTLPOutput(
					testutils.OTLPEndpoint(""),
				).
				Build(),
			errorMsg: "spec.output.otlp.protocol: Unsupported value",
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("otlp-output-basic-auth-secretref-missing-password-key").
				WithApplicationInput(false).
				WithOTLPOutput(
					testutils.OTLPEndpoint(backend.Endpoint()),
					testutils.OTLPBasicAuthFromSecret("name", "namespace", "user", ""),
				).
				Build(),
			errorMsg: "spec.output.otlp.authentication.basic.password.valueFrom.secretKeyRef.key: Required value",
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("otlp-output-basic-auth-secretref-missing-user-key").
				WithApplicationInput(false).
				WithOTLPOutput(
					testutils.OTLPEndpoint(backend.Endpoint()),
					testutils.OTLPBasicAuthFromSecret("name", "namespace", "", "password"),
				).
				Build(),
			errorMsg: "spec.output.otlp.authentication.basic.user.valueFrom.secretKeyRef.key: Required value",
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("otlp-output-tls-missing-key").
				WithApplicationInput(false).
				WithOTLPOutput(
					testutils.OTLPEndpoint(backend.Endpoint()),
					testutils.OTLPClientTLS(&telemetryv1alpha1.OTLPTLS{
						CA:   &telemetryv1alpha1.ValueType{Value: clientCerts.CaCertPem.String()},
						Cert: &telemetryv1alpha1.ValueType{Value: clientCerts.ClientCertPem.String()},
					}),
				).
				Build(),
			errorMsg: "Can define either both 'cert' and 'key', or neither",
		},
		// http output
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("http-output-tls-missing-key").
				WithHTTPOutput(
					testutils.HTTPHost(backend.Host()),
					testutils.HTTPPort(backend.Port()),
					testutils.HTTPClientTLS(telemetryv1alpha1.LogPipelineOutputTLS{
						Cert: &telemetryv1alpha1.ValueType{Value: clientCerts.ClientCertPem.String()},
					}),
				).
				Build(),
			errorMsg: "Can define either both 'cert' and 'key', or neither",
		},
		{
			pipeline: telemetryv1alpha1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "http-output-uri-wrong-pattern",
				},
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.LogPipelineOutput{
						HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{
							URI: "without-leading-slash",
						},
					},
				},
			},
			errorMsg: "spec.output.http.uri in body should match '^/.*$'",
		},
		// application input
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("application-input-namespaces-not-exclusive").
				WithApplicationInput(true).
				WithIncludeNamespaces("ns1").
				WithExcludeNamespaces("ns2").
				WithOTLPOutput().
				Build(),
			errorMsg: "spec.input.application.namespaces: Too many: 2: must have at most 1 items",
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("application-input-containers-not-exclusive").
				WithApplicationInput(true).
				WithIncludeContainers("c1").
				WithExcludeContainers("c2").
				WithOTLPOutput().
				Build(),
			errorMsg: "spec.input.application.containers: Too many: 2: must have at most 1 items",
		},
		// files validation
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("files-name-required").
				WithFile("", "icke").
				WithHTTPOutput().
				Build(),
			errorMsg: "spec.files[0].name: Required value",
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("files-content-required").
				WithFile("file1", "").
				WithHTTPOutput().
				Build(),
			errorMsg: "spec.files[0].content: Required value",
		},
		// variables validation
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("variables-name-required").
				WithVariable("", "secName", "secNs", "secKey").
				WithHTTPOutput().
				Build(),
			errorMsg: "spec.variables[0].name: Required value",
		},
		{
			pipeline: telemetryv1alpha1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "variables-valuefrom-required",
				},
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Variables: []telemetryv1alpha1.LogPipelineVariableRef{
						{
							Name: "var1",
						},
					},
					Output: telemetryv1alpha1.LogPipelineOutput{
						HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{},
					},
				},
			},
			errorMsg: "spec.variables[0].valueFrom.secretKeyRef: Required value",
		},
		// legacy validations
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("legacy-http-output-using-otlp-input").
				WithHTTPOutput().
				WithOTLPInput(true).
				Build(),
			errorMsg: "otlp input is only supported with otlp output",
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("legacy-custom-output-using-otlp-input").
				WithCustomOutput("name icke").
				WithOTLPInput(true).
				Build(),
			errorMsg: "otlp input is only supported with otlp output",
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("legacy-drop-labels-with-otlp-output").
				WithApplicationInput(true).
				WithDropLabels(false).
				WithOTLPOutput(
					testutils.OTLPEndpoint(backend.Endpoint()),
				).
				Build(),
			errorMsg: "input.application.dropLabels is not supported with otlp output",
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("legacy-keep-annotations-with-otlp-output").
				WithApplicationInput(true).
				WithKeepAnnotations(false).
				WithOTLPOutput(
					testutils.OTLPEndpoint(backend.Endpoint()),
				).
				Build(),
			errorMsg: "input.application.keepAnnotations is not supported with otlp output",
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("legacy-files-with-otlp-output").
				WithApplicationInput(false).
				WithFile("file1.json", "icke").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backend.Endpoint()),
				).
				Build(),
			errorMsg: "files not supported with otlp output",
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("legacy-filters-with-otlp-output").
				WithApplicationInput(false).
				WithCustomFilter("name grep").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backend.Endpoint()),
				).
				Build(),
			errorMsg: "filters are not supported with otlp output",
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("legacy-variables-with-otlp-output").
				WithApplicationInput(false).
				WithVariable("var1", "secName", "secNs", "secKey").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backend.Endpoint()),
				).
				Build(),
			errorMsg: "variables not supported with otlp output",
		},
	}
	for _, tc := range tests {
		t.Run(label, func(t *testing.T) {
			suite.RegisterTestCase(t, label)

			resources := []client.Object{&tc.pipeline}

			t.Cleanup(func() {
				Expect(kitk8s.DeleteObjects(resources...)).Should(MatchError(ContainSubstring("not found")))
			})

			err := kitk8s.CreateObjects(t, resources...)

			log.Println("Icke", err)

			if len(tc.errorMsg) > 0 {
				Expect(err).Should(MatchError(ContainSubstring(tc.errorMsg)))
			} else {
				Expect(err).ShouldNot(Succeed(), "unexpected success, this test expects an error")
			}
		})
	}
}
