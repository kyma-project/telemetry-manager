//go:build e2e

package fluentbit

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(ID(), Label(LabelLogs, LabelExperimental), Ordered, func() {
	var (
		v1Alpha1PipelineName = IDWithSuffix("v1alpha1")
		v1Beta1PipelineName  = IDWithSuffix("v1beta1")
	)

	makeResources := func() []client.Object {
		v1Alpha1LogPipeline := telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: v1Alpha1PipelineName,
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Output: telemetryv1alpha1.LogPipelineOutput{
					HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{
						Host: telemetryv1alpha1.ValueType{
							Value: "localhost",
						},
						Port: "443",
						URI:  "/",
						TLS: telemetryv1alpha1.LogPipelineOutputTLS{
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
					HTTP: &telemetryv1beta1.LogPipelineHTTPOutput{
						Host: telemetryv1beta1.ValueType{
							Value: "localhost",
						},
						Port: "443",
						URI:  "/",
						TLSConfig: telemetryv1beta1.OutputTLS{
							Disabled: true,
						},
					},
				},
			},
		}

		return []client.Object{&v1Alpha1LogPipeline, &v1Beta1LogPipeline}
	}

	Context("When v1alpha1 and v1beta1 logpipelines exist", Ordered, func() {
		BeforeAll(func() {
			K8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(Ctx, K8sClient, K8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, K8sObjects...)).Should(Succeed())
		})

		It("Should convert v1alpha1 logpipeline to v1beta1 logpipeline", func() {
			var v1Alpha1AsV1Beta1 telemetryv1beta1.LogPipeline
			err := K8sClient.Get(Ctx, types.NamespacedName{Name: v1Alpha1PipelineName}, &v1Alpha1AsV1Beta1)
			Expect(err).ToNot(HaveOccurred())

			Expect(v1Alpha1AsV1Beta1.Name).To(Equal(v1Alpha1PipelineName))
		})

		It("Should convert v1beta1 logpipeline to v1alpha1 logpipeline", func() {
			var v1Beta1AsV1Alpha1 telemetryv1alpha1.LogPipeline
			err := K8sClient.Get(Ctx, types.NamespacedName{Name: v1Beta1PipelineName}, &v1Beta1AsV1Alpha1)
			Expect(err).ToNot(HaveOccurred())

			Expect(v1Beta1AsV1Alpha1.Name).To(Equal(v1Beta1PipelineName))
		})
	})
})
