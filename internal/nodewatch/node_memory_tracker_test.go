package nodewatch_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/telemetry-manager/internal/nodewatch"
)

func makeNode(name string, memory resource.Quantity) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				corev1.ResourceMemory: memory,
			},
		},
	}
}

func setupTracker(nodes ...corev1.Node) {
	builder := fake.NewClientBuilder()
	for i := range nodes {
		builder = builder.WithObjects(&nodes[i])
	}

	nodewatch.SetClient(builder.Build())
}

func TestUpdateSmallestMemory_EmptyNodeList(t *testing.T) {
	setupTracker()

	changed, err := nodewatch.UpdateSmallestMemory(context.Background())

	require.NoError(t, err)
	assert.True(t, changed)

	mem := nodewatch.SmallestMemory()
	assert.True(t, mem.IsZero())
}

func TestUpdateSmallestMemory_SingleNode(t *testing.T) {
	setupTracker(makeNode("node1", resource.MustParse("4Gi")))

	changed, err := nodewatch.UpdateSmallestMemory(context.Background())

	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, resource.MustParse("4Gi"), nodewatch.SmallestMemory())
}

func TestUpdateSmallestMemory_MultipleNodes_PicksSmallest(t *testing.T) {
	setupTracker(
		makeNode("node1", resource.MustParse("8Gi")),
		makeNode("node2", resource.MustParse("2Gi")),
		makeNode("node3", resource.MustParse("4Gi")),
	)

	changed, err := nodewatch.UpdateSmallestMemory(context.Background())

	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, resource.MustParse("2Gi"), nodewatch.SmallestMemory())
}

func TestUpdateSmallestMemory_Unchanged_ReturnsFalse(t *testing.T) {
	setupTracker(makeNode("node1", resource.MustParse("4Gi")))

	_, err := nodewatch.UpdateSmallestMemory(context.Background())
	require.NoError(t, err)

	changed, err := nodewatch.UpdateSmallestMemory(context.Background())

	require.NoError(t, err)
	assert.False(t, changed)
}

func TestUpdateSmallestMemory_Changed_ReturnsTrue(t *testing.T) {
	node1 := makeNode("node1", resource.MustParse("4Gi"))
	setupTracker(node1)

	_, err := nodewatch.UpdateSmallestMemory(context.Background())
	require.NoError(t, err)

	node2 := makeNode("node2", resource.MustParse("2Gi"))
	setupTracker(node1, node2)

	changed, err := nodewatch.UpdateSmallestMemory(context.Background())

	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, resource.MustParse("2Gi"), nodewatch.SmallestMemory())
}

func TestUpdateSmallestMemory_NodeWithoutAllocatableMemory_IsSkipped(t *testing.T) {
	node1 := makeNode("node1", resource.MustParse("4Gi"))
	node2 := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node2"},
		Status:     corev1.NodeStatus{Allocatable: corev1.ResourceList{}},
	}
	setupTracker(node1, node2)

	changed, err := nodewatch.UpdateSmallestMemory(context.Background())

	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, resource.MustParse("4Gi"), nodewatch.SmallestMemory())
}
