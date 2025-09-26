package misc

import (
	"errors"
	"strconv"
	"testing"

	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestRejectLogPipelineCreation(t *testing.T) {
	const (
		backendHost     = "example.com"
		backendPort     = 4317
		validationError = "some validation rules were not checked"
	)

	var backenEndpoint = backendHost + ":" + strconv.Itoa(backendPort)

	tests := []struct {
		pipeline  telemetryv1alpha1.LogPipeline
		errorMsgs []string
	}{
		// output general
		{
			pipeline: telemetryv1alpha1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "no-output",
				},
				Spec: telemetryv1alpha1.LogPipelineSpec{},
			},
			errorMsgs: []string{"spec.output in body should have at least 1 properties"},
		},
		{
			pipeline: telemetryv1alpha1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "valuefrom-accepts-only-one-option",
				},
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.LogPipelineOutput{
						OTLP: &telemetryv1alpha1.OTLPOutput{
							Endpoint: telemetryv1alpha1.ValueType{
								Value: "example.com",
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name:      "name",
										Namespace: "namespace",
										Key:       "key",
									},
								},
							},
						},
					},
				},
			},
			errorMsgs: []string{"Exactly one of 'value' or 'valueFrom' must be set"},
		},
		{
			pipeline: telemetryv1alpha1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "secretkeyref-requires-key",
				},
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.LogPipelineOutput{
						OTLP: &telemetryv1alpha1.OTLPOutput{
							Endpoint: telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name:      "name",
										Namespace: "namespace",
									},
								},
							},
						},
					},
				},
			},
			errorMsgs: []string{"spec.output.otlp.endpoint.valueFrom.secretKeyRef.key in body should be at least 1 chars long"},
		},
		{
			pipeline: telemetryv1alpha1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "secretkeyref-requires-namespace",
				},
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.LogPipelineOutput{
						OTLP: &telemetryv1alpha1.OTLPOutput{
							Endpoint: telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name: "name",
										Key:  "key",
									},
								},
							},
						},
					},
				},
			},
			errorMsgs: []string{"spec.output.otlp.endpoint.valueFrom.secretKeyRef.namespace in body should be at least 1 chars long"},
		},
		{
			pipeline: telemetryv1alpha1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "secretkeyref-requires-name",
				},
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.LogPipelineOutput{
						OTLP: &telemetryv1alpha1.OTLPOutput{
							Endpoint: telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Namespace: "namespace",
										Key:       "key",
									},
								},
							},
						},
					},
				},
			},
			errorMsgs: []string{"spec.output.otlp.endpoint.valueFrom.secretKeyRef.name in body should be at least 1 chars long"},
		},
		// otlp output
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("otlp-output-with-default-proto-and-path").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPEndpointPath("/v1/mock/metrics"),
				).
				Build(),
			errorMsgs: []string{"Path is only available with HTTP protocol"},
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("otlp-output-with-grpc-proto-and-path").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPEndpointPath("/v1/mock/metrics"),
					testutils.OTLPProtocol("grpc"),
				).
				Build(),
			errorMsgs: []string{"Path is only available with HTTP protocol"},
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("otlp-output-with-non-valid-proto").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPProtocol("icke"),
				).
				Build(),
			errorMsgs: []string{"spec.output.otlp.protocol: Unsupported value", validationError},
		},
		{
			pipeline: telemetryv1alpha1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "otlp-output-without-endpoint",
				},
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.LogPipelineOutput{
						OTLP: &telemetryv1alpha1.OTLPOutput{},
					},
				},
			},
			errorMsgs: []string{"Exactly one of 'value' or 'valueFrom' must be set"},
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("otlp-output-basic-auth-secretref-missing-password-key").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPBasicAuthFromSecret("name", "namespace", "user", ""),
				).
				Build(),
			errorMsgs: []string{"spec.output.otlp.authentication.basic.password.valueFrom.secretKeyRef.key in body should be at least 1 chars long"},
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("otlp-output-basic-auth-secretref-missing-user-key").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPBasicAuthFromSecret("name", "namespace", "", "password"),
				).
				Build(),
			errorMsgs: []string{"spec.output.otlp.authentication.basic.user.valueFrom.secretKeyRef.key in body should be at least 1 chars long"},
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("otlp-output-tls-missing-key").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPClientTLS(&telemetryv1alpha1.OTLPTLS{
						CA:   &telemetryv1alpha1.ValueType{Value: "myCACert"},
						Cert: &telemetryv1alpha1.ValueType{Value: "myClientCert"},
					}),
				).
				Build(),
			errorMsgs: []string{"Can define either both 'cert' and 'key', or neither"},
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("otlp-output-tls-missing-cert").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPClientTLS(&telemetryv1alpha1.OTLPTLS{
						CA:  &telemetryv1alpha1.ValueType{Value: "myCACert"},
						Key: &telemetryv1alpha1.ValueType{Value: "myKey"},
					}),
				).
				Build(),
			errorMsgs: []string{"Can define either both 'cert' and 'key', or neither"},
		},
		// otlp input
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("otlp-input-namespaces-not-exclusive").
				WithOTLPInput(true,
					testutils.ExcludeNamespaces("ns1"),
					testutils.IncludeNamespaces("ns2"),
				).
				WithOTLPOutput().
				Build(),
			errorMsgs: []string{"Too many: 2: must have at most 1 items", validationError},
		},
		// http output
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("http-output-tls-missing-key").
				WithHTTPOutput(
					testutils.HTTPHost(backendHost),
					testutils.HTTPPort(backendPort),
					testutils.HTTPClientTLS(telemetryv1alpha1.LogPipelineOutputTLS{
						Cert: &telemetryv1alpha1.ValueType{Value: "myClientCert"},
					}),
				).
				Build(),
			errorMsgs: []string{"Can define either both 'cert' and 'key', or neither"},
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("http-output-tls-missing-cert").
				WithHTTPOutput(
					testutils.HTTPHost(backendHost),
					testutils.HTTPPort(backendPort),
					testutils.HTTPClientTLS(telemetryv1alpha1.LogPipelineOutputTLS{
						Key: &telemetryv1alpha1.ValueType{Value: "key"},
					}),
				).
				Build(),
			errorMsgs: []string{"Can define either both 'cert' and 'key', or neither"},
		},
		{
			pipeline: telemetryv1alpha1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "http-output-uri-wrong-pattern",
				},
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.LogPipelineOutput{
						HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{
							Host: telemetryv1alpha1.ValueType{Value: "example.com"},
							URI:  "without-leading-slash",
						},
					},
				},
			},
			errorMsgs: []string{"spec.output.http.uri in body should match '^/.*$'"},
		},
		{
			pipeline: telemetryv1alpha1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "http-output-host-required",
				},
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.LogPipelineOutput{
						HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{},
					},
				},
			},
			errorMsgs: []string{"Exactly one of 'value' or 'valueFrom' must be set"},
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
			errorMsgs: []string{"spec.input.application.namespaces: Too many: 2: must have at most 1 items", validationError},
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("application-input-containers-not-exclusive").
				WithApplicationInput(true).
				WithIncludeContainers("c1").
				WithExcludeContainers("c2").
				WithOTLPOutput().
				Build(),
			errorMsgs: []string{"spec.input.application.containers: Too many: 2: must have at most 1 items", validationError},
		},
		// files validation
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("files-name-required").
				WithFile("", "icke").
				WithHTTPOutput().
				Build(),
			errorMsgs: []string{"spec.files[0].name in body should be at least 1 chars long"},
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("files-content-required").
				WithFile("file1", "").
				WithHTTPOutput().
				Build(),
			errorMsgs: []string{"spec.files[0].content in body should be at least 1 chars long"},
		},
		// variables validation
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("variables-name-required").
				WithVariable("", "secName", "secNs", "secKey").
				WithHTTPOutput().
				Build(),
			errorMsgs: []string{"spec.variables[0].name in body should be at least 1 chars long"},
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
			errorMsgs: []string{"spec.variables[0].valueFrom.secretKeyRef in body must be of type object", validationError},
		},
		// legacy validations
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("multiple-outputs").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPEndpointPath("/v1/mock/metrics"),
				).
				WithHTTPOutput().
				Build(),
			errorMsgs: []string{"spec.output: Too many: 2: must have at most 1 items", validationError},
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("legacy-http-output-using-otlp-input").
				WithHTTPOutput().
				WithOTLPInput(true).
				Build(),
			errorMsgs: []string{"otlp input is only supported with otlp output"},
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("legacy-custom-output-using-otlp-input").
				WithCustomOutput("name icke").
				WithOTLPInput(true).
				Build(),
			errorMsgs: []string{"otlp input is only supported with otlp output"},
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("legacy-drop-labels-with-otlp-output").
				WithApplicationInput(true).
				WithDropLabels(false).
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
				).
				Build(),
			errorMsgs: []string{"input.application.dropLabels is not supported with otlp output"},
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("legacy-keep-annotations-with-otlp-output").
				WithApplicationInput(true).
				WithKeepAnnotations(false).
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
				).
				Build(),
			errorMsgs: []string{"input.application.keepAnnotations is not supported with otlp output"},
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("legacy-files-with-otlp-output").
				WithApplicationInput(false).
				WithFile("file1.json", "icke").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
				).
				Build(),
			errorMsgs: []string{"files not supported with otlp output"},
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("legacy-filters-with-otlp-output").
				WithApplicationInput(false).
				WithCustomFilter("name grep").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
				).
				Build(),
			errorMsgs: []string{"filters are not supported with otlp output"},
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("legacy-variables-with-otlp-output").
				WithApplicationInput(false).
				WithVariable("var1", "secName", "secNs", "secKey").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
				).
				Build(),
			errorMsgs: []string{"variables not supported with otlp output"},
		},
	}
	for _, tc := range tests {
		t.Run(suite.LabelMisc, func(t *testing.T) {
			suite.RegisterTestCase(t, suite.LabelMisc)

			resources := []client.Object{&tc.pipeline}

			t.Cleanup(func() {
				Expect(kitk8s.DeleteObjects(resources...)).Should(MatchError(ContainSubstring("not found")))
			})

			err := kitk8s.CreateObjects(t, resources...)

			Expect(err).ShouldNot(Succeed(), "unexpected success for pipeline %s, this test expects an error", tc.pipeline.Name)

			errStatus := &apierrors.StatusError{}
			ok := errors.As(err, &errStatus)
			if ok && errStatus.Status().Details != nil {
				Expect(errStatus.Status().Details.Causes).
					To(HaveLen(len(tc.errorMsgs)),
						"status error for pipeline %s has more than %d cause: %+v",
						tc.pipeline.Name, len(tc.errorMsgs), errStatus.Status().Details.Causes)
			}

			for _, msg := range tc.errorMsgs {
				Expect(err).Should(MatchError(ContainSubstring(msg)), "Error for pipeline %s does not contain expected message %s", tc.pipeline.Name, msg)
			}
		})
	}
}
