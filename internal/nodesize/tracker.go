package nodesize

import (
	"context"
	"fmt"
	"math"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/telemetry-manager/internal/metrics"
)

const vpaMaxAllowedMemoryFraction = 0.3

type Tracker struct {
	mu             sync.RWMutex
	smallestMemory *resource.Quantity
	reader         client.Reader
}

func NewTracker(r client.Reader) *Tracker {
	return &Tracker{reader: r}
}

// UpdateSmallestMemory lists all nodes and recalculates the smallest allocatable memory.
// Returns true if the value changed, false otherwise.
// It also updates the Prometheus metric when the value changes.
func (t *Tracker) UpdateSmallestMemory(ctx context.Context) (bool, error) {
	var nodeList corev1.NodeList
	if err := t.reader.List(ctx, &nodeList); err != nil {
		return false, fmt.Errorf("failed to list nodes: %w", err)
	}

	newSmallest := computeSmallestAllocatableMemory(nodeList.Items)

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.smallestMemory != nil && t.smallestMemory.Cmp(newSmallest) == 0 {
		return false, nil
	}

	logf.FromContext(ctx).Info("Smallest node allocatable memory changed",
		"previous", t.smallestMemory,
		"current", newSmallest.String(),
	)

	t.smallestMemory = &newSmallest

	metrics.NodeSmallestMemoryBytes.Set(float64(newSmallest.Value()))

	return true, nil
}

// SmallestMemory returns the current smallest allocatable memory.
func (t *Tracker) SmallestMemory() resource.Quantity {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.smallestMemory == nil {
		return resource.Quantity{}
	}

	return t.smallestMemory.DeepCopy()
}

// VpaMaxAllowedMemory returns 30% of the smallest allocatable memory,
// rounded down to the nearest KiB. This value is intended to be used as
// the maxAllowed memory in a VPA resource policy.
func (t *Tracker) VpaMaxAllowedMemory() resource.Quantity {
	smallest := t.SmallestMemory()

	thirtyPercent := int64(math.Round(float64(smallest.Value()) * vpaMaxAllowedMemoryFraction))

	const kib = 1024

	roundedToKiB := (thirtyPercent / kib) * kib

	return *resource.NewQuantity(roundedToKiB, resource.BinarySI)
}

func computeSmallestAllocatableMemory(nodes []corev1.Node) resource.Quantity {
	if len(nodes) == 0 {
		return resource.Quantity{}
	}

	var smallest *resource.Quantity

	for i := range nodes {
		// Allocatable represents the resources of a node that are available for scheduling (excluding resources reserved for system components).
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
