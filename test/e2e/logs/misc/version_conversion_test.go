package misc

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestVersionConversion(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelExperimental)

	var (
		uniquePrefix         = unique.Prefix()
		v1Alpha1PipelineName = uniquePrefix("v1alpha1")
		v1Beta1PipelineName  = uniquePrefix("v1beta1")
	)

	v1Alpha1LogPipeline := telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1Alpha1PipelineName,
		},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.LogPipelineOutput{
				FluentBitHTTP: &telemetryv1alpha1.FluentBitHTTPOutput{
					Host: telemetryv1alpha1.ValueType{
						Value: "localhost",
					},
					Port: "443",
					URI:  "/",
					TLS: telemetryv1alpha1.FluentBitHTTPOutputTLS{
						Disabled: true,
					},
				},
			},
		},
	}

	v1Beta1LogPipeline := telemetryv1beta1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1Beta1PipelineName,
		},
		Spec: telemetryv1beta1.LogPipelineSpec{
			Output: telemetryv1beta1.LogPipelineOutput{
				FluentBitHTTP: &telemetryv1beta1.FluentBitHTTPOutput{
					Host: telemetryv1beta1.ValueType{
						Value: "localhost",
					},
					Port: "443",
					URI:  "/",
					TLS: telemetryv1beta1.OutputTLS{
						Insecure: true,
					},
				},
			},
		},
	}

	resources := []client.Object{
		&v1Alpha1LogPipeline,
		&v1Beta1LogPipeline,
	}

	kitk8s.CreateObjects(t, resources...)

	var v1Alpha1AsV1Beta1 telemetryv1beta1.LogPipeline

	Expect(suite.K8sClient.Get(t.Context(), types.NamespacedName{Name: v1Alpha1PipelineName}, &v1Alpha1AsV1Beta1)).To(Succeed())
	Expect(v1Alpha1AsV1Beta1.Name).To(Equal(v1Alpha1PipelineName))

	var v1Beta1AsV1Alpha1 telemetryv1alpha1.LogPipeline

	Expect(suite.K8sClient.Get(t.Context(), types.NamespacedName{Name: v1Beta1PipelineName}, &v1Beta1AsV1Alpha1)).To(Succeed())
	Expect(v1Beta1AsV1Alpha1.Name).To(Equal(v1Beta1PipelineName))
}
