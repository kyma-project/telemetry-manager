package nodewatch

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func resetState() {
	mu.Lock()
	defer mu.Unlock()

	smallestMemoryBytes = 0
	initialized = false
}

func makeNode(memoryBytes int64) corev1.Node {
	return corev1.Node{
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				corev1.ResourceMemory: *resource.NewQuantity(memoryBytes, resource.BinarySI),
			},
		},
	}
}

func TestUpdateSmallestMemory_EmptyNodeList(t *testing.T) {
	resetState()

	changed := UpdateSmallestMemory(context.Background(), nil)

	assert.True(t, changed)
	assert.Equal(t, int64(0), SmallestMemoryBytes())
}

func TestUpdateSmallestMemory_SingleNode(t *testing.T) {
	resetState()

	nodes := []corev1.Node{makeNode(4 * 1024 * 1024 * 1024)}

	changed := UpdateSmallestMemory(context.Background(), nodes)

	assert.True(t, changed)
	assert.Equal(t, int64(4*1024*1024*1024), SmallestMemoryBytes())
}

func TestUpdateSmallestMemory_MultipleNodes_PicksSmallest(t *testing.T) {
	resetState()

	nodes := []corev1.Node{
		makeNode(8 * 1024 * 1024 * 1024),
		makeNode(2 * 1024 * 1024 * 1024),
		makeNode(4 * 1024 * 1024 * 1024),
	}

	changed := UpdateSmallestMemory(context.Background(), nodes)

	assert.True(t, changed)
	assert.Equal(t, int64(2*1024*1024*1024), SmallestMemoryBytes())
}

func TestUpdateSmallestMemory_Unchanged_ReturnsFalse(t *testing.T) {
	resetState()

	nodes := []corev1.Node{makeNode(4 * 1024 * 1024 * 1024)}

	UpdateSmallestMemory(context.Background(), nodes)
	changed := UpdateSmallestMemory(context.Background(), nodes)

	assert.False(t, changed)
}

func TestUpdateSmallestMemory_Changed_ReturnsTrue(t *testing.T) {
	resetState()

	nodes := []corev1.Node{makeNode(4 * 1024 * 1024 * 1024)}
	UpdateSmallestMemory(context.Background(), nodes)

	nodes = []corev1.Node{
		makeNode(4 * 1024 * 1024 * 1024),
		makeNode(2 * 1024 * 1024 * 1024),
	}

	changed := UpdateSmallestMemory(context.Background(), nodes)

	assert.True(t, changed)
	assert.Equal(t, int64(2*1024*1024*1024), SmallestMemoryBytes())
}

func TestUpdateSmallestMemory_NodeWithoutAllocatableMemory_IsSkipped(t *testing.T) {
	resetState()

	nodes := []corev1.Node{
		makeNode(4 * 1024 * 1024 * 1024),
		{Status: corev1.NodeStatus{Allocatable: corev1.ResourceList{}}},
	}

	changed := UpdateSmallestMemory(context.Background(), nodes)

	assert.True(t, changed)
	assert.Equal(t, int64(4*1024*1024*1024), SmallestMemoryBytes())
}
