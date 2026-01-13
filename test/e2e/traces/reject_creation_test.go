package traces

import (
	"errors"
	"strconv"
	"testing"

	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestRejectTracePipelineCreation(t *testing.T) {
	const (
		backendHost = "example.com"
		backendPort = 4317
	)

	var backendEndpoint = backendHost + ":" + strconv.Itoa(backendPort)

	tests := []struct {
		name     string
		pipeline telemetryv1beta1.TracePipeline
		errorMsg string
		field    string
		causes   int
	}{
		// output general
		{
			name: "no-output",
			pipeline: telemetryv1beta1.TracePipeline{
				Spec: telemetryv1beta1.TracePipelineSpec{},
			},
			errorMsg: "must be of type object",
			field:    "spec.output.otlp",
			causes:   2,
		},
		{
			name: "valuefrom-accepts-only-one-option",
			pipeline: telemetryv1beta1.TracePipeline{
				Spec: telemetryv1beta1.TracePipelineSpec{
					Output: telemetryv1beta1.TracePipelineOutput{
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
			pipeline: telemetryv1beta1.TracePipeline{
				Spec: telemetryv1beta1.TracePipelineSpec{
					Output: telemetryv1beta1.TracePipelineOutput{
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
			pipeline: telemetryv1beta1.TracePipeline{
				Spec: telemetryv1beta1.TracePipelineSpec{
					Output: telemetryv1beta1.TracePipelineOutput{
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
			pipeline: telemetryv1beta1.TracePipeline{
				Spec: telemetryv1beta1.TracePipelineSpec{
					Output: telemetryv1beta1.TracePipelineOutput{
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
			pipeline: testutils.NewTracePipelineBuilder().
				WithOTLPOutput(
					testutils.OTLPEndpoint(backendEndpoint),
					testutils.OTLPEndpointPath("/v1/dummy"),
				).
				Build(),
			errorMsg: "Path is only available with HTTP protocol",
			field:    "spec.output.otlp",
		},
		{
			name: "otlp-output-with-grpc-proto-and-path",
			pipeline: testutils.NewTracePipelineBuilder().
				WithOTLPOutput(
					testutils.OTLPEndpoint(backendEndpoint),
					testutils.OTLPEndpointPath("/v1/dummy"),
					testutils.OTLPProtocol("grpc"),
				).
				Build(),
			errorMsg: "Path is only available with HTTP protocol",
			field:    "spec.output.otlp",
		},
		{
			name: "otlp-output-with-non-valid-proto",
			pipeline: testutils.NewTracePipelineBuilder().
				WithOTLPOutput(
					testutils.OTLPEndpoint(backendEndpoint),
					testutils.OTLPProtocol("icke"),
				).
				Build(),
			errorMsg: "Unsupported value",
			causes:   2,
			field:    "spec.output.otlp.protocol",
		},
		{
			name: "otlp-output-basic-auth-secretref-missing-password-key",
			pipeline: testutils.NewTracePipelineBuilder().
				WithOTLPOutput(
					testutils.OTLPEndpoint(backendEndpoint),
					testutils.OTLPBasicAuthFromSecret("name", "namespace", "user", ""),
				).
				Build(),
			errorMsg: "should be at least 1 chars long",
			field:    "spec.output.otlp.authentication.basic.password.valueFrom.secretKeyRef.key",
		},
		{
			name: "otlp-output-basic-auth-secretref-missing-user-key",
			pipeline: testutils.NewTracePipelineBuilder().
				WithOTLPOutput(
					testutils.OTLPEndpoint(backendEndpoint),
					testutils.OTLPBasicAuthFromSecret("name", "namespace", "", "password"),
				).
				Build(),
			errorMsg: "should be at least 1 chars long",
			field:    "spec.output.otlp.authentication.basic.user.valueFrom.secretKeyRef.key",
		},
		{
			name: "otlp-output-tls-missing-key",
			pipeline: testutils.NewTracePipelineBuilder().
				WithOTLPOutput(
					testutils.OTLPEndpoint(backendEndpoint),
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
			pipeline: testutils.NewTracePipelineBuilder().
				WithOTLPOutput(
					testutils.OTLPEndpoint(backendEndpoint),
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
			pipeline: testutils.NewTracePipelineBuilder().
				WithOTLPOutput(
					testutils.OTLPEndpoint(backendEndpoint),
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
			pipeline: testutils.NewTracePipelineBuilder().
				WithOTLPOutput(
					testutils.OTLPEndpoint(backendEndpoint),
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
			pipeline: testutils.NewTracePipelineBuilder().
				WithOTLPOutput(
					testutils.OTLPEndpoint(backendEndpoint),
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
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			suite.RegisterTestCase(t, suite.LabelMisc)

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
