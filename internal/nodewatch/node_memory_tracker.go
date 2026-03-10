package nodewatch

import (
	"context"
	"sync"

	corev1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/telemetry-manager/internal/metrics"
)

var (
	mu                  sync.RWMutex
	smallestMemoryBytes int64
	initialized         bool
)

// UpdateSmallestMemory recalculates the smallest allocatable memory from the given node list.
// Returns true if the value changed, false otherwise.
// It also updates the Prometheus metric when the value changes.
func UpdateSmallestMemory(ctx context.Context, nodes []corev1.Node) bool {
	newSmallest := computeSmallestAllocatableMemory(nodes)

	mu.Lock()
	defer mu.Unlock()

	if initialized && smallestMemoryBytes == newSmallest {
		return false
	}

	logf.FromContext(ctx).Info("Smallest node allocatable memory changed",
		"previous", smallestMemoryBytes,
		"current", newSmallest,
	)

	smallestMemoryBytes = newSmallest
	initialized = true

	metrics.NodeSmallestMemoryBytes.Set(float64(newSmallest))

	return true
}

// SmallestMemoryBytes returns the current smallest allocatable memory in bytes.
func SmallestMemoryBytes() int64 {
	mu.RLock()
	defer mu.RUnlock()

	return smallestMemoryBytes
}

func computeSmallestAllocatableMemory(nodes []corev1.Node) int64 {
	if len(nodes) == 0 {
		return 0
	}

	var smallest int64

	for i, node := range nodes {
		allocatable, ok := node.Status.Allocatable[corev1.ResourceMemory]
		if !ok {
			continue
		}

		memBytes := allocatable.Value()

		if i == 0 || memBytes < smallest {
			smallest = memBytes
		}
	}

	return smallest
}
