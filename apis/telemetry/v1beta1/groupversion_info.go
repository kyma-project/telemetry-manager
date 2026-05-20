// Package v1beta1 contains API Schema definitions for the telemetry v1beta1 API group
// +kubebuilder:object:generate=true
// +groupName=telemetry.kyma-project.io
package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "telemetry.kyma-project.io", Version: "v1beta1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &runtime.SchemeBuilder{}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion)
	metav1.AddToGroupVersion(scheme, GroupVersion)

	return nil
}

//nolint:gochecknoinits // SchemeBuilder's registration is required.
func init() {
	SchemeBuilder.Register(addKnownTypes)
}
