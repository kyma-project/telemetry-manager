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

func TestRejectPipelineCreation(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetricsMisc)

	const (
		backendHost = "example.com"
		backendPort = 4317
	)

	var backenEndpoint = backendHost + ":" + strconv.Itoa(backendPort)

	tests := []struct {
		pipeline telemetryv1alpha1.MetricPipeline
		errorMsg string
		field    string
		causes   int
	}{
		// output general
		{
			pipeline: telemetryv1alpha1.MetricPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "no-output",
				},
				Spec: telemetryv1alpha1.MetricPipelineSpec{},
			},
			errorMsg: "must be of type object",
			field:    "spec.output.otlp",
			causes:   2,
		},
		{
			pipeline: telemetryv1alpha1.MetricPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "valuefrom-accepts-only-one-option",
				},
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Output: telemetryv1alpha1.MetricPipelineOutput{
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
			errorMsg: "Only one of 'value' or 'valueFrom' can be set",
			field:    "spec.output.otlp.endpoint",
		},
		{
			pipeline: telemetryv1alpha1.MetricPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "secretkeyref-requires-key",
				},
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Output: telemetryv1alpha1.MetricPipelineOutput{
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
			pipeline: telemetryv1alpha1.MetricPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "secretkeyref-requires-namespace",
				},
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Output: telemetryv1alpha1.MetricPipelineOutput{
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
			pipeline: telemetryv1alpha1.MetricPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "secretkeyref-requires-name",
				},
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Output: telemetryv1alpha1.MetricPipelineOutput{
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
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("otlp-output-with-default-proto-and-path").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPEndpointPath("/v1/dummy"),
				).
				Build(),
			errorMsg: "Path is only available with HTTP protocol",
			field:    "spec.output.otlp",
		},
		{
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("otlp-output-with-grpc-proto-and-path").
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
			pipeline: testutils.NewMetricPipelineBuilder().
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
			pipeline: testutils.NewMetricPipelineBuilder().
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
			pipeline: testutils.NewMetricPipelineBuilder().
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
			pipeline: testutils.NewMetricPipelineBuilder().
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
			pipeline: testutils.NewMetricPipelineBuilder().
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
		{
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("otlp-output-oauth2-insecure").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPOAuth2(
						testutils.OAuth2ClientID("clientid"),
						testutils.OAuth2ClientSecret("clientsecret"),
						testutils.OAuth2TokenURL("https://auth.example.com/token"),
					),
					testutils.OTLPClientTLS(&telemetryv1alpha1.OTLPTLS{
						Insecure: true,
					}),
				).
				Build(),
			errorMsg: "OAuth2 authentication requires TLS to be configured when using gRPC protocol",
			field:    "spec.output.otlp",
		},
		{
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("otlp-output-oauth2-no-tls").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPOAuth2(
						testutils.OAuth2ClientID("clientid"),
						testutils.OAuth2ClientSecret("clientsecret"),
						testutils.OAuth2TokenURL("https://auth.example.com/token"),
					),
				).
				Build(),
			errorMsg: "OAuth2 authentication requires TLS to be configured when using gRPC protocol",
			field:    "spec.output.otlp",
		},
		// otlp input
		{
			pipeline: testutils.NewMetricPipelineBuilder().
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
		{
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("otlp-input-namespaces-include-invalid").
				WithOTLPInput(true,
					testutils.IncludeNamespaces("aa!"),
				).
				WithOTLPOutput().
				Build(),
			errorMsg: "should match",
			field:    "spec.input.otlp.namespaces.include[0]",
		},
		{
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("otlp-input-namespaces-exclude-invalid").
				WithOTLPInput(true,
					testutils.ExcludeNamespaces("aa!"),
				).
				WithOTLPOutput().
				Build(),
			errorMsg: "should match",
			field:    "spec.input.otlp.namespaces.exclude[0]",
		},
		// prometheus input
		{
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("prometheus-input-namespaces-include-invalid").
				WithPrometheusInput(true,
					testutils.IncludeNamespaces("aa-"),
				).
				WithOTLPOutput().
				Build(),
			errorMsg: "should match",
			field:    "spec.input.prometheus.namespaces.include[0]",
		},
		{
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("prometheus-input-namespaces-exclude-invalid").
				WithPrometheusInput(true,
					testutils.ExcludeNamespaces("-aa"),
				).
				WithOTLPOutput().
				Build(),
			errorMsg: "should match",
			field:    "spec.input.prometheus.namespaces.exclude[0]",
		},
		// istio input
		{
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("istio-input-namespaces-include-invalid").
				WithIstioInput(true,
					testutils.IncludeNamespaces("#"),
				).
				WithOTLPOutput().
				Build(),
			errorMsg: "should match",
			field:    "spec.input.istio.namespaces.include[0]",
		},
		{
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("istio-input-namespaces-exclude-invalid").
				WithIstioInput(true,
					testutils.ExcludeNamespaces("/"),
				).
				WithOTLPOutput().
				Build(),
			errorMsg: "should match",
			field:    "spec.input.istio.namespaces.exclude[0]",
		},
		// runtime input
		{
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("istio-input-namespaces-include-invalid").
				WithRuntimeInput(true,
					testutils.IncludeNamespaces("aa", "bb", "??"),
				).
				WithOTLPOutput().
				Build(),
			errorMsg: "should match",
			field:    "spec.input.runtime.namespaces.include[2]",
		},
		{
			pipeline: testutils.NewMetricPipelineBuilder().
				WithName("istio-input-namespaces-exclude-invalid").
				WithRuntimeInput(true,
					testutils.ExcludeNamespaces("öö", "aa", "bb"),
				).
				WithOTLPOutput().
				Build(),
			errorMsg: "should match",
			field:    "spec.input.runtime.namespaces.exclude[0]",
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
