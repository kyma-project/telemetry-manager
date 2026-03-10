package nodewatch

import (
	"context"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/telemetry-manager/internal/metrics"
)

type nodeTracker struct {
	mu             sync.RWMutex
	smallestMemory resource.Quantity
	initialized    bool
	reader         client.Reader
}

var defaultTracker = &nodeTracker{}

// SetClient sets the client.Reader used to list nodes.
// Must be called once during startup before UpdateSmallestMemory is used.
func SetClient(r client.Reader) {
	defaultTracker.reader = r
}

// UpdateSmallestMemory lists all nodes and recalculates the smallest allocatable memory.
// Returns true if the value changed, false otherwise.
// It also updates the Prometheus metric when the value changes.
func UpdateSmallestMemory(ctx context.Context) (bool, error) {
	return defaultTracker.update(ctx)
}

// SmallestMemory returns the current smallest allocatable memory.
func SmallestMemory() resource.Quantity {
	return defaultTracker.getSmallestMemory()
}

func (t *nodeTracker) update(ctx context.Context) (bool, error) {
	var nodeList corev1.NodeList
	if err := t.reader.List(ctx, &nodeList); err != nil {
		return false, fmt.Errorf("failed to list nodes: %w", err)
	}

	newSmallest := computeSmallestAllocatableMemory(nodeList.Items)

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.initialized && t.smallestMemory.Cmp(newSmallest) == 0 {
		return false, nil
	}

	logf.FromContext(ctx).Info("Smallest node allocatable memory changed",
		"previous", t.smallestMemory.String(),
		"current", newSmallest.String(),
	)

	t.smallestMemory = newSmallest
	t.initialized = true

	metrics.NodeSmallestMemoryBytes.Set(float64(newSmallest.Value()))

	return true, nil
}

func (t *nodeTracker) getSmallestMemory() resource.Quantity {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.smallestMemory.DeepCopy()
}

func computeSmallestAllocatableMemory(nodes []corev1.Node) resource.Quantity {
	if len(nodes) == 0 {
		return resource.Quantity{}
	}

	var smallest *resource.Quantity

	for i := range nodes {
		allocatable, ok := nodes[i].Status.Allocatable[corev1.ResourceMemory]
		if !ok {
			continue
		}

		if smallest == nil || allocatable.Cmp(*smallest) < 0 {
			smallest = &allocatable
		}
	}

	if smallest == nil {
		return resource.Quantity{}
	}

	return smallest.DeepCopy()
}
