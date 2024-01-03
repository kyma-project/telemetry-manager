//go:build e2e

package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kittracepipeline "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/trace"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Traces with custom header and prefix", Label("tracing"), func() {
	const (
		customHeaderName       = "Token"
		customHeaderPrefix     = "Api-Token"
		customHeaderPlainValue = "foo_token"

		customHeaderNameForSecret   = "Authorization"
		customHeaderPrefixForSecret = "Bearer"
		customHeaderSecretKey       = "headerKey"
		customHeaderSecretData      = "bar_token"
	)

	var headerDataKey string
	var headerDataKeyFromSecret string
	var headerSecretName string

	makeResources := func() []client.Object {
		var objs []client.Object

		customHeaderSecret := kitk8s.NewOpaqueSecret("custom-header-secret-trace", kitkyma.DefaultNamespaceName,
			kitk8s.WithStringData(customHeaderSecretKey, customHeaderSecretData))

		headerSecretName = customHeaderSecret.Name()

		headers := []telemetryv1alpha1.Header{
			{
				Name:   customHeaderName,
				Prefix: customHeaderPrefix,
				ValueType: telemetryv1alpha1.ValueType{
					Value: customHeaderPlainValue,
				},
			},
			{
				Name:   customHeaderNameForSecret,
				Prefix: customHeaderPrefixForSecret,
				ValueType: telemetryv1alpha1.ValueType{
					ValueFrom: &telemetryv1alpha1.ValueFromSource{
						SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
							Key:       customHeaderSecretKey,
							Name:      headerSecretName,
							Namespace: kitkyma.DefaultNamespaceName,
						},
					},
				},
			},
		}

		tracePipeline := kittracepipeline.NewPipeline("mock-trace-custom-header-prefix").WithHeaders(headers)

		pipelineName := tracePipeline.Name()
		headerDataKey = fmt.Sprintf("%s_%s_%s", "HEADER", kitkyma.MakeEnvVarCompliant(pipelineName), kitkyma.MakeEnvVarCompliant(customHeaderName))
		headerDataKeyFromSecret = fmt.Sprintf("%s_%s_%s", "HEADER", kitkyma.MakeEnvVarCompliant(pipelineName), kitkyma.MakeEnvVarCompliant(customHeaderNameForSecret))
		objs = append(objs, tracePipeline.K8sObject(), customHeaderSecret.K8sObject())
		return objs
	}

	Context("When a TracePipeline with custom header and prefix exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a secret with custom header value and prefix", func() {
			verifiers.SecretShouldHaveValue(ctx, k8sClient, kitkyma.TraceGatewaySecretName, headerDataKey, fmt.Sprintf("%s %s", customHeaderPrefix, customHeaderPlainValue))
		})

		It("Should have a secret with custom header value and prefix from secret value", func() {
			verifiers.SecretShouldHaveValue(ctx, k8sClient, kitkyma.TraceGatewaySecretName, headerDataKeyFromSecret, fmt.Sprintf("%s %s", customHeaderPrefixForSecret, customHeaderSecretData))
		})
	})
})
