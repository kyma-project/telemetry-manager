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
		backendHost     = "example.com"
		backendPort     = 4317
		validationError = "some validation rules were not checked"
	)

	var backenEndpoint = backendHost + ":" + strconv.Itoa(backendPort)

	tests := []struct {
		pipeline  telemetryv1alpha1.TracePipeline
		errorMsgs []string
	}{
		// output general
		{
			pipeline: telemetryv1alpha1.TracePipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "no-output",
				},
				Spec: telemetryv1alpha1.TracePipelineSpec{},
			},
			errorMsgs: []string{"spec.output.otlp in body must be of type object", validationError},
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
			errorMsgs: []string{"Exactly one of 'value' or 'valueFrom' must be set"},
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
			errorMsgs: []string{"spec.output.otlp.endpoint.valueFrom.secretKeyRef.key in body should be at least 1 chars long"},
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
			errorMsgs: []string{"spec.output.otlp.endpoint.valueFrom.secretKeyRef.namespace in body should be at least 1 chars long"},
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
			errorMsgs: []string{"spec.output.otlp.endpoint.valueFrom.secretKeyRef.name in body should be at least 1 chars long"},
		},
		// otlp output
		{
			pipeline: testutils.NewTracePipelineBuilder().
				WithName("otlp-output-with-default-proto-and-path").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPEndpointPath("/v1/mock/Traces"),
				).
				Build(),
			errorMsgs: []string{"Path is only available with HTTP protocol"},
		},
		{
			pipeline: testutils.NewTracePipelineBuilder().
				WithName("otlp-output-with-grpc-proto-and-path").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPEndpointPath("/v1/mock/Traces"),
					testutils.OTLPProtocol("grpc"),
				).
				Build(),
			errorMsgs: []string{"Path is only available with HTTP protocol"},
		},
		{
			pipeline: testutils.NewTracePipelineBuilder().
				WithName("otlp-output-with-non-valid-proto").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPProtocol("icke"),
				).
				Build(),
			errorMsgs: []string{"spec.output.otlp.protocol: Unsupported value", validationError},
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
			errorMsgs: []string{"Exactly one of 'value' or 'valueFrom' must be set"},
		},
		{
			pipeline: testutils.NewTracePipelineBuilder().
				WithName("otlp-output-basic-auth-secretref-missing-password-key").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPBasicAuthFromSecret("name", "namespace", "user", ""),
				).
				Build(),
			errorMsgs: []string{"spec.output.otlp.authentication.basic.password.valueFrom.secretKeyRef.key in body should be at least 1 chars long"},
		},
		{
			pipeline: testutils.NewTracePipelineBuilder().
				WithName("otlp-output-basic-auth-secretref-missing-user-key").
				WithOTLPOutput(
					testutils.OTLPEndpoint(backenEndpoint),
					testutils.OTLPBasicAuthFromSecret("name", "namespace", "", "password"),
				).
				Build(),
			errorMsgs: []string{"spec.output.otlp.authentication.basic.user.valueFrom.secretKeyRef.key in body should be at least 1 chars long"},
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
			errorMsgs: []string{"Can define either both 'cert' and 'key', or neither"},
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
			errorMsgs: []string{"Can define either both 'cert' and 'key', or neither"},
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
