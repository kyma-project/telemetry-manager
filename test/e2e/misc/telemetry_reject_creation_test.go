package misc

import (
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestRejectTelemetryCRCreation(t *testing.T) {
	suite.SetupTest(t, suite.LabelTelemetry, suite.LabelMisc)

	tests := []struct {
		name      string
		telemetry operatorv1beta1.Telemetry
		errorMsg  string
		field     string
		causes    int
	}{
		{
			name: "global metric collection interval zero",
			telemetry: operatorv1beta1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: kitkyma.SystemNamespaceName,
				},
				Spec: operatorv1beta1.TelemetrySpec{
					Metric: &operatorv1beta1.MetricSpec{
						CollectionInterval: &metav1.Duration{Duration: 0},
					},
				},
			},
			errorMsg: "'collectionInterval' must be greater than 0",
			field:    "spec.metric.collectionInterval",
		},
		{
			name: "global metric collection interval negative",
			telemetry: operatorv1beta1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: kitkyma.SystemNamespaceName,
				},
				Spec: operatorv1beta1.TelemetrySpec{
					Metric: &operatorv1beta1.MetricSpec{
						CollectionInterval: &metav1.Duration{Duration: -1 * time.Second},
					},
				},
			},
			errorMsg: "'collectionInterval' must be greater than 0",
			field:    "spec.metric.collectionInterval",
		},
		{
			name: "runtime collection interval zero",
			telemetry: operatorv1beta1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: kitkyma.SystemNamespaceName,
				},
				Spec: operatorv1beta1.TelemetrySpec{
					Metric: &operatorv1beta1.MetricSpec{
						Runtime: &operatorv1beta1.MetricInputSpec{
							CollectionInterval: &metav1.Duration{Duration: 0},
						},
					},
				},
			},
			errorMsg: "'collectionInterval' must be greater than 0",
			field:    "spec.metric.runtime.collectionInterval",
		},
		{
			name: "prometheus collection interval zero",
			telemetry: operatorv1beta1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: kitkyma.SystemNamespaceName,
				},
				Spec: operatorv1beta1.TelemetrySpec{
					Metric: &operatorv1beta1.MetricSpec{
						Prometheus: &operatorv1beta1.MetricInputSpec{
							CollectionInterval: &metav1.Duration{Duration: 0},
						},
					},
				},
			},
			errorMsg: "'collectionInterval' must be greater than 0",
			field:    "spec.metric.prometheus.collectionInterval",
		},
		{
			name: "istio collection interval zero",
			telemetry: operatorv1beta1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: kitkyma.SystemNamespaceName,
				},
				Spec: operatorv1beta1.TelemetrySpec{
					Metric: &operatorv1beta1.MetricSpec{
						Istio: &operatorv1beta1.MetricInputSpec{
							CollectionInterval: &metav1.Duration{Duration: 0},
						},
					},
				},
			},
			errorMsg: "'collectionInterval' must be greater than 0",
			field:    "spec.metric.istio.collectionInterval",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			suite.SetupTest(t, suite.LabelMisc)

			tc.telemetry.Name = "custom"

			resources := []client.Object{&tc.telemetry}

			err := kitk8s.CreateObjects(t, resources...)

			Expect(err).ShouldNot(Succeed(), "unexpected success for telemetry '%s', this test expects an error", tc.telemetry.Name)

			errStatus := &apierrors.StatusError{}

			ok := errors.As(err, &errStatus)
			Expect(ok).To(BeTrue(), "telemetry '%s' has wrong error type %s", tc.telemetry.Name, err.Error())
			Expect(errStatus.Status().Details).ToNot(BeNil(), "error of telemetry '%s' has no details %w", tc.telemetry.Name, err.Error())

			if tc.causes == 0 {
				Expect(errStatus.Status().Details.Causes).
					To(HaveLen(1),
						"telemetry '%s' has more or less than 1 cause: %+v", tc.telemetry.Name, errStatus.Status().Details.Causes)
			} else {
				Expect(errStatus.Status().Details.Causes).
					To(HaveLen(tc.causes),
						"telemetry '%s' has more or less than %d causes: %+v", tc.telemetry.Name, tc.causes, errStatus.Status().Details.Causes)
			}

			Expect(errStatus.Status().Details.Causes[0].Field).To(Equal(tc.field), "the first error cause for telemetry '%s' does not contain expected field %s", tc.telemetry.Name, tc.field)

			Expect(errStatus.Status().Details.Causes[0].Message).Should(ContainSubstring(tc.errorMsg), "the error for telemetry '%s' does not contain expected message %s", tc.telemetry.Name, tc.errorMsg)
		})
	}
}
