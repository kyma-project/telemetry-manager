package misc

import (
	"errors"
	"strconv"
	"testing"

	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestRejectLogPipelineCreation(t *testing.T) {
	const (
		backendHost = "example.com"
		backendPort = 4317
		// Example string longer than 63 characters
		veryLongString = "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz123"
	)

	var backenEndpoint = backendHost + ":" + strconv.Itoa(backendPort)

	tests := []struct {
		name     string
		pipeline telemetryv1beta1.LogPipeline
		errorMsg string
		field    string
		causes   int
		label    string
	}{
		// output general
		{
			name: "no-output",
			pipeline: telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{},
			},
			errorMsg: "Exactly one output out of 'custom', 'http' or 'otlp' must be defined",
			field:    "spec.output",
		},
		{
			name: "valuefrom-accepts-only-one-option",
			pipeline: telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Output: telemetryv1beta1.LogPipelineOutput{
						OTLP: &telemetryv1beta1.OTLPOutput{
							Endpoint: telemetryv1beta1.ValueType{
								Value: "example.com",
								ValueFrom: &telemetryv1beta1.ValueFromSource{
									SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
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
			name: "secretkeyref-requires-key",
			pipeline: telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Output: telemetryv1beta1.LogPipelineOutput{
						OTLP: &telemetryv1beta1.OTLPOutput{
							Endpoint: telemetryv1beta1.ValueType{
								ValueFrom: &telemetryv1beta1.ValueFromSource{
									SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
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
			name: "secretkeyref-requires-namespace",
			pipeline: telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Output: telemetryv1beta1.LogPipelineOutput{
						OTLP: &telemetryv1beta1.OTLPOutput{
							Endpoint: telemetryv1beta1.ValueType{
								ValueFrom: &telemetryv1beta1.ValueFromSource{
									SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
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
			name: "secretkeyref-requires-name",
			pipeline: telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Output: telemetryv1beta1.LogPipelineOutput{
						OTLP: &telemetryv1beta1.OTLPOutput{
							Endpoint: telemetryv1beta1.ValueType{
								ValueFrom: &telemetryv1beta1.ValueFromSource{
									SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
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
			name: "otlp-output-with-default-proto-and-path",
			pipeline: testutils.NewLogPipelineBuilder().
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPEndpointPath("/v1/dummy"),
				).
				Build(),
			errorMsg: "Path is only available with HTTP protocol",
			field:    "spec.output.otlp",
		},
		{
			name: "otlp-output-with-grpc-proto-and-path",
			pipeline: testutils.NewLogPipelineBuilder().
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPEndpointPath("/v1/dummy"),
					testutils.OTLPProtocol("grpc"),
				).
				Build(),
			errorMsg: "Path is only available with HTTP protocol",
			field:    "spec.output.otlp",
		},
		{
			name: "otlp-output-with-non-valid-proto",
			pipeline: testutils.NewLogPipelineBuilder().
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
			name: "otlp-output-basic-auth-secretref-missing-password-key",
			pipeline: testutils.NewLogPipelineBuilder().
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPBasicAuthFromSecret("name", "namespace", "user", ""),
				).
				Build(),
			errorMsg: "should be at least 1 chars long",
			field:    "spec.output.otlp.authentication.basic.password.valueFrom.secretKeyRef.key",
		},
		{
			name: "otlp-output-basic-auth-secretref-missing-user-key",
			pipeline: testutils.NewLogPipelineBuilder().
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPBasicAuthFromSecret("name", "namespace", "", "password"),
				).
				Build(),
			errorMsg: "should be at least 1 chars long",
			field:    "spec.output.otlp.authentication.basic.user.valueFrom.secretKeyRef.key",
		},
		{
			name: "otlp-output-tls-missing-key",
			pipeline: testutils.NewLogPipelineBuilder().
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPClientTLS(&telemetryv1beta1.OutputTLS{
						CA:   &telemetryv1beta1.ValueType{Value: "myCACert"},
						Cert: &telemetryv1beta1.ValueType{Value: "myClientCert"},
					}),
				).
				Build(),
			errorMsg: "Can define either both 'cert' and 'key', or neither",
			field:    "spec.output.otlp.tls",
		},
		{
			name: "otlp-output-tls-missing-cert",
			pipeline: testutils.NewLogPipelineBuilder().
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPClientTLS(&telemetryv1beta1.OutputTLS{
						CA:  &telemetryv1beta1.ValueType{Value: "myCACert"},
						Key: &telemetryv1beta1.ValueType{Value: "myKey"},
					}),
				).
				Build(),
			errorMsg: "Can define either both 'cert' and 'key', or neither",
			field:    "spec.output.otlp.tls",
		},
		{
			name: "otlp-output-oauth2-invalid-token-url",
			pipeline: testutils.NewLogPipelineBuilder().
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPOAuth2(
						testutils.OAuth2ClientSecret("clientsecret"),
						testutils.OAuth2ClientID("clientid"),
						testutils.OAuth2TokenURL("../not-a-url"),
					),
				).
				Build(),
			errorMsg: "Invalid value: \"object\": 'tokenURL' must be a valid URL",
			field:    "spec.output.otlp.authentication.oauth2.tokenURL",
		},
		{
			name: "otlp-output-oauth2-missing-client-id",
			pipeline: testutils.NewLogPipelineBuilder().
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPOAuth2(
						testutils.OAuth2ClientSecret("clientsecret"),
						testutils.OAuth2TokenURL("https://auth.example.com/token"),
					),
				).
				Build(),
			errorMsg: "Exactly one of 'value' or 'valueFrom' must be set",
			field:    "spec.output.otlp.authentication.oauth2.clientID",
			causes:   1,
		},
		{
			name: "otlp-output-oauth2-insecure",
			pipeline: testutils.NewLogPipelineBuilder().
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPOAuth2(
						testutils.OAuth2ClientID("clientid"),
						testutils.OAuth2ClientSecret("clientsecret"),
						testutils.OAuth2TokenURL("https://auth.example.com/token"),
					),
					testutils.OTLPClientTLS(&telemetryv1beta1.OutputTLS{
						Insecure: true,
					}),
				).
				Build(),
			errorMsg: "OAuth2 authentication requires TLS when using gRPC protocol",
			field:    "spec.output.otlp",
		},
		{
			name: "otlp-input-namespaces-not-exclusive",
			pipeline: testutils.NewLogPipelineBuilder().
				WithOTLPInput(true,
					testutils.ExcludeNamespaces("ns1"),
					testutils.IncludeNamespaces("ns2"),
				).
				WithOTLPOutput().
				Build(),
			errorMsg: "Only one of 'include' or 'exclude' can be defined",
			field:    "spec.input.otlp.namespaces",
		},
		{
			name: "otlp-input-namespaces-include-invalid",
			pipeline: testutils.NewLogPipelineBuilder().
				WithOTLPInput(true,
					testutils.IncludeNamespaces("Test"),
				).
				WithOTLPOutput().
				Build(),
			errorMsg: "should match",
			field:    "spec.input.otlp.namespaces.include[0]",
		},
		{
			name: "otlp-input-namespaces-include-too-long",
			pipeline: testutils.NewLogPipelineBuilder().
				WithOTLPInput(true,
					testutils.IncludeNamespaces(veryLongString),
				).
				WithOTLPOutput().
				Build(),
			errorMsg: "Too long:",
			field:    "spec.input.otlp.namespaces.include[0]",
			causes:   2,
		},
		{
			name: "otlp-input-namespaces-exclude-invalid",
			pipeline: testutils.NewLogPipelineBuilder().
				WithOTLPInput(true,
					testutils.ExcludeNamespaces("Test"),
				).
				WithOTLPOutput().
				Build(),
			errorMsg: "should match",
			field:    "spec.input.otlp.namespaces.exclude[0]",
		},
		{
			name: "otlp-input-namespaces-exclude-too-long",
			pipeline: testutils.NewLogPipelineBuilder().
				WithOTLPInput(true,
					testutils.ExcludeNamespaces(veryLongString),
				).
				WithOTLPOutput().
				Build(),
			errorMsg: "Too long:",
			field:    "spec.input.otlp.namespaces.exclude[0]",
			causes:   2,
		},
		// http output
		{
			name: "http-output-tls-missing-key",
			pipeline: testutils.NewLogPipelineBuilder().
				WithHTTPOutput(
					testutils.HTTPHost(backendHost),
					testutils.HTTPPort(backendPort),
					testutils.HTTPClientTLS(telemetryv1beta1.OutputTLS{
						Cert: &telemetryv1beta1.ValueType{Value: "myClientCert"},
					}),
				).
				Build(),
			errorMsg: "Can define either both 'cert' and 'key', or neither",
			field:    "spec.output.http.tls",
		},
		{
			name: "http-output-tls-missing-cert",
			pipeline: testutils.NewLogPipelineBuilder().
				WithHTTPOutput(
					testutils.HTTPHost(backendHost),
					testutils.HTTPPort(backendPort),
					testutils.HTTPClientTLS(telemetryv1beta1.OutputTLS{
						Key: &telemetryv1beta1.ValueType{Value: "key"},
					}),
				).
				Build(),
			errorMsg: "Can define either both 'cert' and 'key', or neither",
			field:    "spec.output.http.tls",
		},
		{
			name: "http-output-uri-wrong-pattern",
			pipeline: telemetryv1beta1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "http-output-uri-wrong-pattern",
				},
				Spec: telemetryv1beta1.LogPipelineSpec{
					Output: telemetryv1beta1.LogPipelineOutput{
						FluentBitHTTP: &telemetryv1beta1.FluentBitHTTPOutput{
							Host: telemetryv1beta1.ValueType{Value: "example.com"},
							URI:  "without-leading-slash",
						},
					},
				},
			},
			errorMsg: "should match '^/.*$'",
			field:    "spec.output.http.uri",
		},
		// runtime input
		{
			name: "runtime-input-namespaces-exclude-system-not-exclusive",
			pipeline: testutils.NewLogPipelineBuilder().
				WithRuntimeInput(true).
				WithIncludeNamespaces("ns1").
				WithExcludeNamespaces("ns2").
				WithOTLPOutput().
				Build(),
			errorMsg: "Only one of 'include' or 'exclude' can be defined",
			field:    "spec.input.runtime.namespaces",
		},
		{
			name: "runtime-input-containers-not-exclusive",
			pipeline: testutils.NewLogPipelineBuilder().
				WithRuntimeInput(true).
				WithIncludeContainers("c1").
				WithExcludeContainers("c2").
				WithOTLPOutput().
				Build(),
			errorMsg: "Only one of 'include' or 'exclude' can be defined",
			field:    "spec.input.runtime.containers",
		},
		{
			name: "runtime-input-namespaces-include-invalid",
			pipeline: testutils.NewLogPipelineBuilder().
				WithRuntimeInput(true).
				WithIncludeNamespaces("*").
				WithOTLPOutput().
				Build(),
			errorMsg: "should match",
			field:    "spec.input.runtime.namespaces.include[0]",
		},
		{
			name: "runtime-input-namespaces-include-too-long",
			pipeline: testutils.NewLogPipelineBuilder().
				WithRuntimeInput(true).
				WithIncludeNamespaces(veryLongString).
				WithOTLPOutput().
				Build(),
			errorMsg: "Too long:",
			field:    "spec.input.runtime.namespaces.include[0]",
			causes:   2,
		},
		{
			name: "runtime-input-namespaces-exclude-invalid",
			pipeline: testutils.NewLogPipelineBuilder().
				WithRuntimeInput(true).
				WithExcludeNamespaces("a*a").
				WithOTLPOutput().
				Build(),
			errorMsg: "should match",
			field:    "spec.input.runtime.namespaces.exclude[0]",
		},
		{
			name: "runtime-input-namespaces-exclude-too-long",
			pipeline: testutils.NewLogPipelineBuilder().
				WithRuntimeInput(true).
				WithExcludeNamespaces(veryLongString).
				WithOTLPOutput().
				Build(),
			errorMsg: "Too long:",
			field:    "spec.input.runtime.namespaces.exclude[0]",
			causes:   2,
		},
		// files validation
		{
			name: "files-name-required",
			pipeline: testutils.NewLogPipelineBuilder().
				WithFile("", "icke").
				WithHTTPOutput().
				Build(),
			errorMsg: "should be at least 1 chars long",
			field:    "spec.files[0].name",
		},
		{
			name: "files-content-required",
			pipeline: testutils.NewLogPipelineBuilder().
				WithFile("file1", "").
				WithHTTPOutput().
				Build(),
			errorMsg: "should be at least 1 chars long",
			field:    "spec.files[0].content",
		},
		// variables validation
		{
			name: "variables-name-required",
			pipeline: testutils.NewLogPipelineBuilder().
				WithVariable("", "secName", "secNs", "secKey").
				WithHTTPOutput().
				Build(),
			errorMsg: "should be at least 1 chars long",
			field:    "spec.variables[0].name",
		},
		{
			name: "variables-valuefrom-required",
			pipeline: telemetryv1beta1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "variables-valuefrom-required",
				},
				Spec: telemetryv1beta1.LogPipelineSpec{
					FluentBitVariables: []telemetryv1beta1.FluentBitVariable{
						{
							Name: "var1",
						},
					},
					Output: telemetryv1beta1.LogPipelineOutput{
						FluentBitHTTP: &telemetryv1beta1.FluentBitHTTPOutput{},
					},
				},
			},
			errorMsg: "must be of type object",
			causes:   2,
			field:    "spec.variables[0].valueFrom.secretKeyRef",
		},
		// legacy validations
		{
			name: "multiple-outputs",
			pipeline: testutils.NewLogPipelineBuilder().
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
				).
				WithHTTPOutput().
				Build(),
			errorMsg: "Exactly one output out of 'custom', 'http' or 'otlp' must be defined",
			field:    "spec.output",
		},
		{
			name: "legacy-http-output-using-otlp-input",
			pipeline: testutils.NewLogPipelineBuilder().
				WithHTTPOutput().
				WithOTLPInput(true).
				Build(),
			errorMsg: "otlp input is only supported with otlp output",
			field:    "spec",
		},
		{
			name: "legacy-custom-output-using-otlp-input",
			pipeline: testutils.NewLogPipelineBuilder().
				WithCustomOutput("name icke").
				WithOTLPInput(true).
				Build(),
			errorMsg: "otlp input is only supported with otlp output",
			field:    "spec",
		},
		{
			name: "legacy-drop-labels-with-otlp-output",
			pipeline: testutils.NewLogPipelineBuilder().
				WithRuntimeInput(true).
				WithDropLabels(false).
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
				).
				Build(),
			errorMsg: "input.runtime.dropLabels is not supported with otlp output",
			field:    "spec",
		},
		{
			name: "legacy-keep-annotations-with-otlp-output",
			pipeline: testutils.NewLogPipelineBuilder().
				WithRuntimeInput(true).
				WithKeepAnnotations(false).
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
				).
				Build(),
			errorMsg: "input.runtime.keepAnnotations is not supported with otlp output",
			field:    "spec",
		},
		{
			name: "legacy-files-with-otlp-output",
			pipeline: testutils.NewLogPipelineBuilder().
				WithRuntimeInput(false).
				WithFile("file1.json", "icke").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
				).
				Build(),
			errorMsg: "files not supported with otlp output",
			field:    "spec",
		},
		{
			name: "legacy-filters-with-otlp-output",
			pipeline: testutils.NewLogPipelineBuilder().
				WithRuntimeInput(false).
				WithCustomFilter("name grep").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
				).
				Build(),
			errorMsg: "filters are not supported with otlp output",
			field:    "spec",
		},
		{
			name: "legacy-variables-with-otlp-output",
			pipeline: testutils.NewLogPipelineBuilder().
				WithRuntimeInput(false).
				WithVariable("var1", "secName", "secNs", "secKey").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
				).
				Build(),
			errorMsg: "variables not supported with otlp output",
			field:    "spec",
		},
		{
			name: "legacy-transform-with-http-output",
			pipeline: testutils.NewLogPipelineBuilder().
				WithRuntimeInput(false).
				WithTransform(telemetryv1beta1.TransformSpec{
					Statements: []string{"set(attributes[\"log.level\"], \"error\")", "set(body, \"transformed1\")"},
				}).
				WithHTTPOutput().
				Build(),
			errorMsg: "transform is only supported with otlp output",
			field:    "spec",
		},
		{
			name: "legacy-filter-with-http-output",
			pipeline: testutils.NewLogPipelineBuilder().
				WithRuntimeInput(false).
				WithFilter(telemetryv1beta1.FilterSpec{
					Conditions: []string{"isMatch(log.attributes[\"log.level\"], \"error\"))"},
				}).
				WithHTTPOutput().
				Build(),
			errorMsg: "filter is only supported with otlp output",
			field:    "spec",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.label, suite.LabelLogsMisc)

			tc.pipeline.Name = tc.name

			resources := []client.Object{&tc.pipeline}

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
