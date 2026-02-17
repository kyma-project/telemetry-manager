package kubeprep

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DetectIstioInstalled checks if Istio is installed by looking for the default Istio CR.
// This is the only detection needed because Istio changes require special handling
// (manager must be removed before Istio changes).
func DetectIstioInstalled(ctx context.Context, k8sClient client.Client) bool {
	if k8sClient == nil {
		return false
	}

	// Check if the default Istio CR exists in kyma-system namespace
	// This is the CR created by our Istio installation: istios.operator.kyma-project.io/default
	istioCR := &unstructured.Unstructured{}
	istioCR.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.kyma-project.io",
		Version: "v1alpha2",
		Kind:    "Istio",
	})

	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      "default",
		Namespace: "kyma-system",
	}, istioCR)

	return err == nil
}
