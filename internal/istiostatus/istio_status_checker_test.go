package istiostatus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clienttesting "k8s.io/client-go/testing"
)

func TestIsIstioActive(t *testing.T) {
	scheme := clientgoscheme.Scheme

	err := apiextensionsv1.AddToScheme(scheme)
	if err != nil {
		t.Fatalf("failed to add api extensions v1 scheme: %v", err)
	}

	tests := []struct {
		name      string
		resources []*metav1.APIResourceList
		want      bool
	}{
		{
			name: "should return true if peerauthentication crd found",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "peerauthentication.security.istio.io/v1beta1",
					APIResources: []metav1.APIResource{},
				},
			},
			want: true,
		},
		{
			name: "should return false if peerauthentication not crd found",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "operator.kyma-project.io/v1beta1",
					APIResources: []metav1.APIResource{},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			discovery := fake.FakeDiscovery{
				Fake: &clienttesting.Fake{
					Resources: tt.resources,
				},
			}
			checker := NewChecker(&discovery)
			got := checker.IsIstioActive(t.Context())

			assert.Equal(t, tt.want, got)
		})
	}
}
