package operator

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func typeName(obj client.Object) string {
	t := reflect.TypeOf(obj)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	return fmt.Sprintf("%s/%s", t.PkgPath(), t.Name())
}

func typeNames(objs []client.Object) []string {
	names := make([]string, 0, len(objs))
	for _, obj := range objs {
		names = append(names, typeName(obj))
	}

	return names
}

func TestTelemetryOwnedResourceTypes(t *testing.T) {
	// Resources always managed by the Telemetry reconciler (selfmonitor ApplierDeleter):
	// - Deployment, ConfigMap, Service, ServiceAccount, Role, RoleBinding, NetworkPolicy
	// Additionally, the Telemetry reconciler manages webhook cert Secrets.
	expectedAlways := []client.Object{
		&appsv1.Deployment{},
		&corev1.ConfigMap{},
		&corev1.Secret{},
		&corev1.Service{},
		&corev1.ServiceAccount{},
		&rbacv1.Role{},
		&rbacv1.RoleBinding{},
		&networkingv1.NetworkPolicy{},
	}

	actual := telemetryOwnedResourceTypes()
	assert.ElementsMatch(t, typeNames(expectedAlways), typeNames(actual),
		"telemetryOwnedResourceTypes() must match the resource types managed by the Telemetry reconciler")
}
