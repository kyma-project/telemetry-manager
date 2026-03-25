package telemetry

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	istionetworkingclientv1 "istio.io/client-go/pkg/apis/networking/v1"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	autoscalingvpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// typeName returns the Go type name of a client.Object for comparison purposes.
func typeName(obj client.Object) string {
	t := reflect.TypeOf(obj)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	return fmt.Sprintf("%s/%s", t.PkgPath(), t.Name())
}

// typeNames returns a sorted set of type names for a list of client.Objects.
func typeNames(objs []client.Object) []string {
	names := make([]string, 0, len(objs))
	for _, obj := range objs {
		names = append(names, typeName(obj))
	}

	return names
}

// The following tests ensure that the resource types watched by each controller match the resource
// types that are actually managed (created/updated/deleted) by the corresponding reconciler.
//
// If a new resource type is added to an applier/deleter, it must also be added to the corresponding
// controller's owned resource types function, and vice versa. Failing to do so will cause these
// tests to fail, preventing unwatched resources from slipping through.

func TestLogPipelineOwnedResourceTypes(t *testing.T) {
	// Resources always managed by the LogPipeline reconciler:
	// - FluentBitApplierDeleter: DaemonSet, ConfigMap, Secret, Service, ServiceAccount, ClusterRole, ClusterRoleBinding, NetworkPolicy
	// - GatewayApplierDeleter (log gateway): Deployment, ConfigMap, Secret, Service, ServiceAccount, ClusterRole, ClusterRoleBinding, NetworkPolicy
	// - AgentApplierDeleter (log agent): DaemonSet, ConfigMap, Secret, Service, ServiceAccount, ClusterRole, ClusterRoleBinding, NetworkPolicy
	// - OTLPGatewayApplierDeleter: DaemonSet, ConfigMap, Secret, Service, ServiceAccount, ClusterRole, ClusterRoleBinding, NetworkPolicy
	//
	// Union of always-managed types:
	expectedAlways := []client.Object{
		&appsv1.DaemonSet{},
		&appsv1.Deployment{},
		&corev1.ConfigMap{},
		&corev1.Secret{},
		&corev1.Service{},
		&corev1.ServiceAccount{},
		&rbacv1.ClusterRole{},
		&rbacv1.ClusterRoleBinding{},
		&networkingv1.NetworkPolicy{},
	}

	actual := logPipelineOwnedResourceTypes()
	assert.ElementsMatch(t, typeNames(expectedAlways), typeNames(actual),
		"logPipelineOwnedResourceTypes() must match the resource types managed by the log pipeline applier/deleters")

	// Conditionally managed types (watched separately in SetupWithManager):
	// - PeerAuthentication: created by GatewayApplierDeleter when Istio is active
	// - DestinationRule: created by OTLPGatewayApplierDeleter when Istio is active
	// - VerticalPodAutoscaler: created by AgentApplierDeleter when VPA CRD exists
	expectedConditional := []client.Object{
		&istiosecurityclientv1.PeerAuthentication{},
		&istionetworkingclientv1.DestinationRule{},
		&autoscalingvpav1.VerticalPodAutoscaler{},
	}

	// Verify that conditional types are NOT included in the always-watched list
	alwaysNames := typeNames(actual)
	for _, obj := range expectedConditional {
		assert.NotContains(t, alwaysNames, typeName(obj),
			"conditional resource %s should not be in the always-watched list", typeName(obj))
	}
}

func TestMetricPipelineOwnedResourceTypes(t *testing.T) {
	// Resources always managed by the MetricPipeline reconciler:
	// - GatewayApplierDeleter (metric gateway): Deployment, ConfigMap, Secret, Service, ServiceAccount, ClusterRole, ClusterRoleBinding, Role, RoleBinding, NetworkPolicy
	// - AgentApplierDeleter (metric agent): DaemonSet, ConfigMap, Secret, Service, ServiceAccount, ClusterRole, ClusterRoleBinding, Role, RoleBinding, NetworkPolicy
	//
	// Union of always-managed types:
	expectedAlways := []client.Object{
		&appsv1.DaemonSet{},
		&appsv1.Deployment{},
		&corev1.ConfigMap{},
		&corev1.Secret{},
		&corev1.Service{},
		&corev1.ServiceAccount{},
		&rbacv1.ClusterRole{},
		&rbacv1.ClusterRoleBinding{},
		&rbacv1.Role{},
		&rbacv1.RoleBinding{},
		&networkingv1.NetworkPolicy{},
	}

	actual := metricPipelineOwnedResourceTypes()
	assert.ElementsMatch(t, typeNames(expectedAlways), typeNames(actual),
		"metricPipelineOwnedResourceTypes() must match the resource types managed by the metric pipeline applier/deleters")

	// Conditionally managed types (watched separately in SetupWithManager):
	// - PeerAuthentication: created by GatewayApplierDeleter when Istio is active
	// - VerticalPodAutoscaler: created by AgentApplierDeleter when VPA CRD exists
	expectedConditional := []client.Object{
		&istiosecurityclientv1.PeerAuthentication{},
		&autoscalingvpav1.VerticalPodAutoscaler{},
	}

	alwaysNames := typeNames(actual)
	for _, obj := range expectedConditional {
		assert.NotContains(t, alwaysNames, typeName(obj),
			"conditional resource %s should not be in the always-watched list", typeName(obj))
	}
}

func TestTracePipelineOwnedResourceTypes(t *testing.T) {
	// Resources always managed by the TracePipeline reconciler:
	// - GatewayApplierDeleter (trace gateway): Deployment, ConfigMap, Secret, Service, ServiceAccount, ClusterRole, ClusterRoleBinding, NetworkPolicy
	//
	// The trace gateway does not use DaemonSet, Role, RoleBinding, or VPA.
	expectedAlways := []client.Object{
		&appsv1.Deployment{},
		&corev1.ConfigMap{},
		&corev1.Secret{},
		&corev1.Service{},
		&corev1.ServiceAccount{},
		&rbacv1.ClusterRole{},
		&rbacv1.ClusterRoleBinding{},
		&networkingv1.NetworkPolicy{},
	}

	actual := tracePipelineOwnedResourceTypes()
	assert.ElementsMatch(t, typeNames(expectedAlways), typeNames(actual),
		"tracePipelineOwnedResourceTypes() must match the resource types managed by the trace pipeline applier/deleters")

	// Conditionally managed types (watched separately in SetupWithManager):
	// - PeerAuthentication: created by GatewayApplierDeleter when Istio is active
	expectedConditional := []client.Object{
		&istiosecurityclientv1.PeerAuthentication{},
	}

	alwaysNames := typeNames(actual)
	for _, obj := range expectedConditional {
		assert.NotContains(t, alwaysNames, typeName(obj),
			"conditional resource %s should not be in the always-watched list", typeName(obj))
	}
}
