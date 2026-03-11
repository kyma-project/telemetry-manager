package k8sclients

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewManagedResourceClient creates a client for writing managed resources.
// It composes two interceptors:
// 1. Labeler — ensures common telemetry labels on Create/Update/Patch
// 2. OwnerReferenceSetter — sets the owner reference on Create/Update
func NewManagedResourceClient(inner client.Client, owner metav1.Object) client.Client {
	return NewOwnerReferenceSetter(NewLabeler(inner), owner)
}
