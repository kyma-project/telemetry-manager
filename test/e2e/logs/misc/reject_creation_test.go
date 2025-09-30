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
		backendHost = "example.com"
		backendPort = 4317
	)

	var backenEndpoint = backendHost + ":" + strconv.Itoa(backendPort)

	tests := []struct {
		pipeline telemetryv1alpha1.LogPipeline
		errorMsg string
		field    string
		causes   int
		label    string
	}{
		// output general
		{
			pipeline: telemetryv1alpha1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "no-output",
				},
				Spec: telemetryv1alpha1.LogPipelineSpec{},
			},
			errorMsg: "Exactly one output out of 'custom', 'http' or 'otlp' must be defined",
			field:    "spec.output",
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
			errorMsg: "Exactly one of 'value' or 'valueFrom' must be set",
			field:    "spec.output.otlp.endpoint",
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
			errorMsg: "should be at least 1 chars long",
			field:    "spec.output.otlp.endpoint.valueFrom.secretKeyRef.key",
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
			errorMsg: "should be at least 1 chars long",
			field:    "spec.output.otlp.endpoint.valueFrom.secretKeyRef.namespace",
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
			errorMsg: "should be at least 1 chars long",
			field:    "spec.output.otlp.endpoint.valueFrom.secretKeyRef.name",
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
			errorMsg: "Path is only available with HTTP protocol",
			field:    "spec.output.otlp",
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
			errorMsg: "Path is only available with HTTP protocol",
			field:    "spec.output.otlp",
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("otlp-output-with-non-valid-proto").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPProtocol("icke"),
				).
				Build(),
			errorMsg: "Unsupported value",
			causes:   2,
			field:    "spec.output.otlp.protocol",
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
			errorMsg: "Exactly one of 'value' or 'valueFrom' must be set",
			field:    "spec.output.otlp.endpoint",
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("otlp-output-basic-auth-secretref-missing-password-key").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPBasicAuthFromSecret("name", "namespace", "user", ""),
				).
				Build(),
			errorMsg: "should be at least 1 chars long",
			field:    "spec.output.otlp.authentication.basic.password.valueFrom.secretKeyRef.key",
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("otlp-output-basic-auth-secretref-missing-user-key").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPBasicAuthFromSecret("name", "namespace", "", "password"),
				).
				Build(),
			errorMsg: "should be at least 1 chars long",
			field:    "spec.output.otlp.authentication.basic.user.valueFrom.secretKeyRef.key",
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
			errorMsg: "Can define either both 'cert' and 'key', or neither",
			field:    "spec.output.otlp.tls",
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
			errorMsg: "Can define either both 'cert' and 'key', or neither",
			field:    "spec.output.otlp.tls",
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
			errorMsg: "Only one of 'include' or 'exclude' can be defined",
			field:    "spec.input.otlp.namespaces",
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
			errorMsg: "Can define either both 'cert' and 'key', or neither",
			field:    "spec.output.http.tls",
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
			errorMsg: "Can define either both 'cert' and 'key', or neither",
			field:    "spec.output.http.tls",
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
			errorMsg: "should match '^/.*$'",
			field:    "spec.output.http.uri",
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
			errorMsg: "Exactly one of 'value' or 'valueFrom' must be set",
			field:    "spec.output.http.host",
		},
		// application input
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("application-input-namespaces-include-exclude-not-exclusive").
				WithApplicationInput(true).
				WithIncludeNamespaces("ns1").
				WithExcludeNamespaces("ns2").
				WithOTLPOutput().
				Build(),
			errorMsg: "Only one of 'include', 'exclude' or 'system' can be defined",
			field:    "spec.input.application.namespaces",
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("application-input-namespaces-include-system-not-exclusive").
				WithApplicationInput(true).
				WithIncludeNamespaces("ns1").
				WithSystemNamespaces(true).
				WithOTLPOutput().
				Build(),
			errorMsg: "Only one of 'include', 'exclude' or 'system' can be defined",
			field:    "spec.input.application.namespaces",
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("application-input-containers-not-exclusive").
				WithApplicationInput(true).
				WithIncludeContainers("c1").
				WithExcludeContainers("c2").
				WithOTLPOutput().
				Build(),
			errorMsg: "Only one of 'include' or 'exclude' can be defined",
			field:    "spec.input.application.containers",
		},
		// files validation
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("files-name-required").
				WithFile("", "icke").
				WithHTTPOutput().
				Build(),
			errorMsg: "should be at least 1 chars long",
			field:    "spec.files[0].name",
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("files-content-required").
				WithFile("file1", "").
				WithHTTPOutput().
				Build(),
			errorMsg: "should be at least 1 chars long",
			field:    "spec.files[0].content",
		},
		// variables validation
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("variables-name-required").
				WithVariable("", "secName", "secNs", "secKey").
				WithHTTPOutput().
				Build(),
			errorMsg: "should be at least 1 chars long",
			field:    "spec.variables[0].name",
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
			errorMsg: "must be of type object",
			causes:   2,
			field:    "spec.variables[0].valueFrom.secretKeyRef",
		},
		// legacy validations
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("multiple-outputs").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
				).
				WithHTTPOutput().
				Build(),
			errorMsg: "Exactly one output out of 'custom', 'http' or 'otlp' must be defined",
			field:    "spec.output",
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("legacy-http-output-using-otlp-input").
				WithHTTPOutput().
				WithOTLPInput(true).
				Build(),
			errorMsg: "otlp input is only supported with otlp output",
			field:    "spec",
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("legacy-custom-output-using-otlp-input").
				WithCustomOutput("name icke").
				WithOTLPInput(true).
				Build(),
			errorMsg: "otlp input is only supported with otlp output",
			field:    "spec",
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
			errorMsg: "input.application.dropLabels is not supported with otlp output",
			field:    "spec",
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
			errorMsg: "input.application.keepAnnotations is not supported with otlp output",
			field:    "spec",
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
			errorMsg: "files not supported with otlp output",
			field:    "spec",
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
			errorMsg: "filters are not supported with otlp output",
			field:    "spec",
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
			errorMsg: "variables not supported with otlp output",
			field:    "spec",
		},
		{
			pipeline: testutils.NewLogPipelineBuilder().
				WithName("legacy-transform-with-http-output").
				WithApplicationInput(false).
				WithTransform(telemetryv1alpha1.TransformSpec{
					Statements: []string{"set(attributes[\"log.level\"], \"error\")", "set(body, \"transformed1\")"},
				}).
				WithHTTPOutput().
				Build(),
			errorMsg: "transform is only supported with otlp output",
			field:    "spec",
			label:    suite.LabelExperimental,
		},
	}
	for _, tc := range tests {
		if tc.label == "" {
			tc.label = suite.LabelMisc
		}

		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.label)

			resources := []client.Object{&tc.pipeline}

			t.Cleanup(func() {
				Expect(kitk8s.DeleteObjects(resources...)).Should(MatchError(ContainSubstring("not found")))
			})

			err := kitk8s.CreateObjects(t, resources...)

			Expect(err).ShouldNot(Succeed(), "unexpected success for pipeline '%s', this test expects an error", tc.pipeline.Name)

			errStatus := &apierrors.StatusError{}

			ok := errors.As(err, &errStatus)
			Expect(ok).To(BeTrue(), "pipeline '%s' has wrong error type %s", tc.pipeline.Name, err.Error())
			Expect(errStatus.Status().Details).ToNot(BeNil(), "error of pipeline '%s' has no details %w", tc.pipeline.Name, err.Error())

			if tc.causes == 0 {
				Expect(errStatus.Status().Details.Causes).
					To(HaveLen(1),
						"pipeline '%s' has more or less than 1 cause: %+v", tc.pipeline.Name, errStatus.Status().Details.Causes)
			} else {
				Expect(errStatus.Status().Details.Causes).
					To(HaveLen(tc.causes),
						"pipeline '%s' has more or less than %d causes: %+v", tc.pipeline.Name, tc.causes, errStatus.Status().Details.Causes)
			}

			Expect(errStatus.Status().Details.Causes[0].Field).To(Equal(tc.field), "the first error cause for pipeline '%s' does not contain expected field %s", tc.pipeline.Name, tc.field)

			Expect(errStatus.Status().Details.Causes[0].Message).Should(ContainSubstring(tc.errorMsg), "the error for pipeline '%s' does not contain expected message %s", tc.pipeline.Name, tc.errorMsg)
		})
	}
}
