package common

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestMakeNetworkPolicy(t *testing.T) {
	name := types.NamespacedName{Name: "test-component", Namespace: "test-namespace"}
	labels := map[string]string{"app": "test"}
	selectorLabels := map[string]string{"app.kubernetes.io/name": "test-component"}

	tests := []struct {
		name           string
		opts           []NetworkPolicyOption
		goldenFilePath string
	}{
		{
			name:           "basic",
			opts:           nil,
			goldenFilePath: "testdata/networkpolicy-basic.yaml",
		},
		{
			name: "ingress options",
			opts: []NetworkPolicyOption{
				WithIngressFromAny([]int32{8080}),
				WithIngressFromPods(map[string]string{"app": "source"}, []int32{9090}),
				WithIngressFromPodsInAllNamespaces(map[string]string{"app": "global"}, []int32{9091}),
				WithIngressFromPodsInNamespace("other-ns", map[string]string{"app": "external"}, []int32{9092}),
				WithIngressRule(networkingv1.NetworkPolicyIngressRule{
					From: []networkingv1.NetworkPolicyPeer{
						{IPBlock: &networkingv1.IPBlock{CIDR: "10.0.0.0/8"}},
					},
				}),
			},
			goldenFilePath: "testdata/networkpolicy-ingress.yaml",
		},
		{
			name: "egress options",
			opts: []NetworkPolicyOption{
				WithEgressToAny(),
				WithEgressToPods(map[string]string{"app": "target"}, []int32{3000}),
				WithEgressToPodsInAllNamespaces(map[string]string{"app": "global-target"}, []int32{3001}),
				WithEgressToPodsInNamespace("other-ns", map[string]string{"app": "external-target"}, 3002),
				WithEgressRule(networkingv1.NetworkPolicyEgressRule{
					To: []networkingv1.NetworkPolicyPeer{
						{IPBlock: &networkingv1.IPBlock{CIDR: "10.0.0.0/8"}},
					},
				}),
			},
			goldenFilePath: "testdata/networkpolicy-egress.yaml",
		},
		{
			name: "with name suffix",
			opts: []NetworkPolicyOption{
				WithNameSuffix("custom"),
				WithIngressFromAny([]int32{8080}),
				WithEgressToAny(),
			},
			goldenFilePath: "testdata/networkpolicy-with-name-suffix.yaml",
		},
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			np := MakeNetworkPolicy(name, labels, selectorLabels, tt.opts...)

			objects := []client.Object{np}
			bytes, err := testutils.MarshalYAML(scheme, objects)
			require.NoError(t, err)

			if testutils.ShouldUpdateGoldenFiles() {
				testutils.UpdateGoldenFileYAML(t, tt.goldenFilePath, bytes)
				return
			}

			goldenFileBytes, err := os.ReadFile(tt.goldenFilePath)
			require.NoError(t, err)

			require.Equal(t, string(goldenFileBytes), string(bytes))
		})
	}
}
