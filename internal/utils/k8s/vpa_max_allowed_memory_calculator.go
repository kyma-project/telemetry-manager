package k8s

import (
	"context"
	"fmt"
	"math"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CalculateVpaMaxAllowedMemory returns 30% of the lowest allocatable memory across all nodes in the cluster,
// rounded down to the nearest KiB. This value is intended to be used as the maxAllowed memory in a VPA resource policy.
func CalculateVpaMaxAllowedMemory(ctx context.Context, c client.Client) (resource.Quantity, error) {
	var nodeList corev1.NodeList
	if err := c.List(ctx, &nodeList); err != nil {
		return resource.Quantity{}, fmt.Errorf("failed to list nodes: %w", err)
	}

	minMemory := int64(math.MaxInt64)
	for i := range nodeList.Items {
		allocatable := nodeList.Items[i].Status.Allocatable[corev1.ResourceMemory]
		if val := allocatable.Value(); val < minMemory {
			minMemory = val
		}
	}

	thirtyPercent := int64(math.Round(float64(minMemory) * 0.3))
	const kib = 1024
	roundedToKiB := (thirtyPercent / kib) * kib

	return *resource.NewQuantity(roundedToKiB, resource.BinarySI), nil
}
