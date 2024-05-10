//go:build e2e

package e2e

import (
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe(suite.ID(), Ordered, Label(suite.LabelTraces), func() {
	var (
		pipelineName    = suite.ID()
		basicAuthSecret *kitk8s.Secret
	)

	const (
		basicAuthSecretName                 = "traces-basic-auth-credentials" // #nosec G101
		basicAuthSecretUsernameKey          = "user"
		basicAuthSecretUsernameValue        = "secret-username"
		basicAuthSecretUpdatedUsernameValue = "new-secret-username"
		basicAuthSecretPasswordKey          = "password"
		basicAuthSecretPasswordValue        = "secret-password"
		basicAuthSecretUpdatedPasswordValue = "new-secret-password"

		customHeaderName       = "Token"
		customHeaderPrefix     = "Api-Token"
		customHeaderPlainValue = "foo_token"

		customHeaderSecretName         = "traces-custom-header"
		customHeaderSecretKey          = "headerKey"
		customHeaderSecretValue        = "bar_token"
		customHeaderNameForSecretRef   = "Authorization"
		customHeaderPrefixForSecretRef = "Bearer"
	)

	makeResources := func() []client.Object {
		basicAuthSecret = kitk8s.NewOpaqueSecret(basicAuthSecretName, kitkyma.DefaultNamespaceName,
			kitk8s.WithStringData(basicAuthSecretUsernameKey, basicAuthSecretUsernameValue),
			kitk8s.WithStringData(basicAuthSecretPasswordKey, basicAuthSecretPasswordValue),
		)

		customHeaderSecret := kitk8s.NewOpaqueSecret(customHeaderSecretName, kitkyma.DefaultNamespaceName,
			kitk8s.WithStringData(customHeaderSecretKey, customHeaderSecretValue),
		)

		pipeline := kitk8s.NewTracePipelineV1Alpha1(pipelineName).
			WithBasicAuthUserFromSecret(basicAuthSecret.SecretKeyRefV1Alpha1(basicAuthSecretUsernameKey)).
			WithBasicAuthPasswordFromSecret(basicAuthSecret.SecretKeyRefV1Alpha1(basicAuthSecretPasswordKey)).
			WithHeader(customHeaderName, customHeaderPrefix, customHeaderPlainValue).
			WithHeaderFromSecret(customHeaderNameForSecretRef, customHeaderPrefixForSecretRef, customHeaderSecret.SecretKeyRefV1Alpha1(customHeaderSecretKey))

		objs := []client.Object{
			basicAuthSecret.K8sObject(),
			customHeaderSecret.K8sObject(),
			pipeline.K8sObject(),
		}

		return objs
	}

	BeforeAll(func() {
		k8sObjects := makeResources()

		DeferCleanup(func() {
			Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})
		Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
	})

	Context("When a TracePipeline with basic authentication exists", Ordered, func() {
		It("Should have a trace gateway secret with correct authentication credentials", func() {
			encodedCredentials := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", basicAuthSecretUsernameValue, basicAuthSecretPasswordValue)))

			verifiers.SecretShouldHaveValue(ctx, k8sClient, kitkyma.TraceGatewaySecretName,
				fmt.Sprintf("BASIC_AUTH_HEADER_%s", kitkyma.MakeEnvVarCompliant(pipelineName)),
				fmt.Sprintf("Basic %s", encodedCredentials),
			)
		})

		It("Should update the trace gateway secret when referenced secret changes", func() {
			By("Updating the referenced secret", func() {
				newData := map[string][]byte{
					basicAuthSecretUsernameKey: []byte(basicAuthSecretUpdatedUsernameValue),
					basicAuthSecretPasswordKey: []byte(basicAuthSecretUpdatedPasswordValue),
				}

				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      basicAuthSecret.Name(),
						Namespace: kitkyma.DefaultNamespaceName,
					},
					Data: newData,
				}

				Expect(k8sClient.Update(ctx, secret)).Should(Succeed())
			})

			encodedCredentials := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", basicAuthSecretUpdatedUsernameValue, basicAuthSecretUpdatedPasswordValue)))

			verifiers.SecretShouldHaveValue(ctx, k8sClient, kitkyma.TraceGatewaySecretName,
				fmt.Sprintf("BASIC_AUTH_HEADER_%s", kitkyma.MakeEnvVarCompliant(pipelineName)),
				fmt.Sprintf("Basic %s", encodedCredentials),
			)
		})

	})

	Context("When a TracePipeline with custom header prefix and plain value exists", Ordered, func() {
		It("Should have a trace gateway secret with custom header prefix and plain value", func() {
			verifiers.SecretShouldHaveValue(ctx, k8sClient, kitkyma.TraceGatewaySecretName,
				fmt.Sprintf("HEADER_%s_%s", kitkyma.MakeEnvVarCompliant(pipelineName), kitkyma.MakeEnvVarCompliant(customHeaderName)),
				fmt.Sprintf("%s %s", customHeaderPrefix, customHeaderPlainValue),
			)
		})
	})

	Context("When a TracePipeline with custom header prefix and value from secret exists", Ordered, func() {
		It("Should have a trace gateway secret with custom header prefix and value from secret", func() {
			verifiers.SecretShouldHaveValue(ctx, k8sClient, kitkyma.TraceGatewaySecretName,
				fmt.Sprintf("HEADER_%s_%s", kitkyma.MakeEnvVarCompliant(pipelineName), kitkyma.MakeEnvVarCompliant(customHeaderNameForSecretRef)),
				fmt.Sprintf("%s %s", customHeaderPrefixForSecretRef, customHeaderSecretValue),
			)
		})
	})
})
