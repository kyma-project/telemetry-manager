package nodesize_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/telemetry-manager/internal/nodesize"
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

func newTracker(nodes ...corev1.Node) *nodesize.Tracker {
	builder := fake.NewClientBuilder()
	for i := range nodes {
		builder = builder.WithObjects(&nodes[i])
	}

	return nodesize.NewTracker(builder.Build())
}

func TestUpdateSmallestMemory_EmptyNodeList(t *testing.T) {
	tracker := newTracker()

	changed, err := tracker.UpdateSmallestMemory(context.Background())

	require.NoError(t, err)
	assert.True(t, changed)

	mem := tracker.SmallestMemory()
	assert.True(t, mem.IsZero())
}

func TestUpdateSmallestMemory_SingleNode(t *testing.T) {
	tracker := newTracker(makeNode("node1", resource.MustParse("4Gi")))

	changed, err := tracker.UpdateSmallestMemory(context.Background())

	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, resource.MustParse("4Gi"), tracker.SmallestMemory())
}

func TestUpdateSmallestMemory_MultipleNodes_PicksSmallest(t *testing.T) {
	tracker := newTracker(
		makeNode("node1", resource.MustParse("8Gi")),
		makeNode("node2", resource.MustParse("2Gi")),
		makeNode("node3", resource.MustParse("4Gi")),
	)

	changed, err := tracker.UpdateSmallestMemory(context.Background())

	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, resource.MustParse("2Gi"), tracker.SmallestMemory())
}

func TestUpdateSmallestMemory_Unchanged_ReturnsFalse(t *testing.T) {
	tracker := newTracker(makeNode("node1", resource.MustParse("4Gi")))

	_, err := tracker.UpdateSmallestMemory(context.Background())
	require.NoError(t, err)

	changed, err := tracker.UpdateSmallestMemory(context.Background())

	require.NoError(t, err)
	assert.False(t, changed)
}

func TestUpdateSmallestMemory_Changed_ReturnsTrue(t *testing.T) {
	node1 := makeNode("node1", resource.MustParse("4Gi"))
	node2 := makeNode("node2", resource.MustParse("2Gi"))

	tracker := newTracker(node1)

	_, err := tracker.UpdateSmallestMemory(context.Background())
	require.NoError(t, err)

	// Recreate tracker with both nodes to simulate a new node appearing
	tracker = newTracker(node1, node2)

	changed, err := tracker.UpdateSmallestMemory(context.Background())

	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, resource.MustParse("2Gi"), tracker.SmallestMemory())
}

func TestUpdateSmallestMemory_NodeWithoutAllocatableMemory_IsSkipped(t *testing.T) {
	node1 := makeNode("node1", resource.MustParse("4Gi"))
	node2 := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node2"},
		Status:     corev1.NodeStatus{Allocatable: corev1.ResourceList{}},
	}
	tracker := newTracker(node1, node2)

	changed, err := tracker.UpdateSmallestMemory(context.Background())

	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, resource.MustParse("4Gi"), tracker.SmallestMemory())
}

func TestVPAMaxAllowedMemory(t *testing.T) {
	// 4Gi = 4294967296 bytes
	// 30% = 1288490188.8 → rounded to 1288490189
	// Floor to KiB: (1288490189 / 1024) * 1024 = 1288489984
	tracker := newTracker(makeNode("node1", resource.MustParse("4Gi")))

	_, err := tracker.UpdateSmallestMemory(context.Background())
	require.NoError(t, err)

	vpa := tracker.VPAMaxAllowedMemory()

	expected := *resource.NewQuantity(1288489984, resource.BinarySI)
	assert.Equal(t, expected, vpa)
}
