package assert

import (
	"context"
	"fmt"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func SecretHasKeyValue(ctx context.Context, name types.NamespacedName, dataKey, dataValue string) {
	Eventually(func(g Gomega) {
		secret, err := secretExists(ctx, suite.K8sClient, name)
		g.Expect(err).NotTo(HaveOccurred())

		secretValue, found := secret.Data[dataKey]
		g.Expect(found).Should(BeTrueBecause("Secret does not contain key %s", dataKey))

		g.Expect(string(secretValue)).Should(Equal(dataValue))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func secretExists(ctx context.Context, k8sClient client.Client, name types.NamespacedName) (*corev1.Secret, error) {
	var secret corev1.Secret

	err := k8sClient.Get(ctx, name, &secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s: %w", name.String(), err)
	}

	return &secret, nil
}
