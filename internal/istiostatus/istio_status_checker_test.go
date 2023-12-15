package istiostatus

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestIsIstioActive(t *testing.T) {
	scheme := clientgoscheme.Scheme
	_ = apiextensionsv1.AddToScheme(scheme)

	tests := []struct {
		name string
		crds []*apiextensionsv1.CustomResourceDefinition
		want bool
	}{
		{
			name: "should return true if peerauthentication crd found",
			crds: []*apiextensionsv1.CustomResourceDefinition{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "peerauthentications.security.istio.io",
					},
				},
			},
			want: true,
		},
		{
			name: "should return false if peerauthentication not crd found",
			crds: []*apiextensionsv1.CustomResourceDefinition{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClientBuilder := fake.NewClientBuilder().WithScheme(scheme)
			for _, crd := range tt.crds {
				fakeClientBuilder.WithObjects(crd)
			}
			fakeClient := fakeClientBuilder.Build()

			isc := &Checker{client: fakeClient}

			got := isc.IsIstioActive(context.Background())

			assert.Equal(t, tt.want, got)
		})
	}
}
