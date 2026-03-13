package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCalculateVpaMaxAllowedMemory(t *testing.T) {
	tests := []struct {
		name           string
		nodes          []corev1.Node
		expectedResult resource.Quantity
	}{
		{
			name: "single node",
			nodes: []corev1.Node{
				newNodeWithMemory("node-1", 8*1024*1024*1024), // 8Gi
			},
			// 30% of 8Gi = 2,576,980,377.6 bytes, math.Round = 2,576,980,378, floor to KiB = 2,576,979,968
			expectedResult: *resource.NewQuantity(2576979968, resource.BinarySI),
		},
		{
			name: "multiple nodes returns 30 percent of lowest",
			nodes: []corev1.Node{
				newNodeWithMemory("node-1", 16*1024*1024*1024), // 16Gi
				newNodeWithMemory("node-2", 8*1024*1024*1024),  // 8Gi
				newNodeWithMemory("node-3", 32*1024*1024*1024), // 32Gi
			},
			// lowest is 8Gi, 30% rounded and floored to KiB = 2,576,979,968
			expectedResult: *resource.NewQuantity(2576979968, resource.BinarySI),
		},
		{
			name: "small node memory",
			nodes: []corev1.Node{
				newNodeWithMemory("node-1", 1024*1024), // 1Mi
			},
			// 30% of 1Mi = 314,572.8 bytes, math.Round = 314,573, floor to KiB = 314,368 (307Ki)
			expectedResult: *resource.NewQuantity(314368, resource.BinarySI),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objs []corev1.Node
			objs = append(objs, tt.nodes...)

			clientBuilder := fake.NewClientBuilder()
			for i := range objs {
				clientBuilder = clientBuilder.WithObjects(&objs[i])
			}

			fakeClient := clientBuilder.Build()

			result, err := CalculateVpaMaxAllowedMemory(t.Context(), fakeClient)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult.Value(), result.Value())
		})
	}
}

func newNodeWithMemory(name string, memoryBytes int64) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				corev1.ResourceMemory: *resource.NewQuantity(memoryBytes, resource.BinarySI),
			},
		},
	}
}
