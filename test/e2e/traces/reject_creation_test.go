package traces

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

func TestRejectTracePipelineCreation(t *testing.T) {
	const (
		backendHost = "example.com"
		backendPort = 4317
	)

	var backenEndpoint = backendHost + ":" + strconv.Itoa(backendPort)

	tests := []struct {
		pipeline telemetryv1alpha1.TracePipeline
		errorMsg string
		field    string
		causes   int
		label    string
	}{
		// output general
		{
			pipeline: telemetryv1alpha1.TracePipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "no-output",
				},
				Spec: telemetryv1alpha1.TracePipelineSpec{},
			},
			errorMsg: "must be of type object",
			field:    "spec.output.otlp",
			causes:   2,
		},
		{
			pipeline: telemetryv1alpha1.TracePipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "valuefrom-accepts-only-one-option",
				},
				Spec: telemetryv1alpha1.TracePipelineSpec{
					Output: telemetryv1alpha1.TracePipelineOutput{
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
			pipeline: telemetryv1alpha1.TracePipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "secretkeyref-requires-key",
				},
				Spec: telemetryv1alpha1.TracePipelineSpec{
					Output: telemetryv1alpha1.TracePipelineOutput{
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
			pipeline: telemetryv1alpha1.TracePipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "secretkeyref-requires-namespace",
				},
				Spec: telemetryv1alpha1.TracePipelineSpec{
					Output: telemetryv1alpha1.TracePipelineOutput{
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
			pipeline: telemetryv1alpha1.TracePipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "secretkeyref-requires-name",
				},
				Spec: telemetryv1alpha1.TracePipelineSpec{
					Output: telemetryv1alpha1.TracePipelineOutput{
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
			pipeline: testutils.NewTracePipelineBuilder().
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
			pipeline: testutils.NewTracePipelineBuilder().
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
			pipeline: testutils.NewTracePipelineBuilder().
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
			pipeline: telemetryv1alpha1.TracePipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "otlp-output-without-endpoint",
				},
				Spec: telemetryv1alpha1.TracePipelineSpec{
					Output: telemetryv1alpha1.TracePipelineOutput{
						OTLP: &telemetryv1alpha1.OTLPOutput{},
					},
				},
			},
			errorMsg: "Exactly one of 'value' or 'valueFrom' must be set",
			field:    "spec.output.otlp.endpoint",
		},
		{
			pipeline: testutils.NewTracePipelineBuilder().
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
			pipeline: testutils.NewTracePipelineBuilder().
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
			pipeline: testutils.NewTracePipelineBuilder().
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
			pipeline: testutils.NewTracePipelineBuilder().
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
