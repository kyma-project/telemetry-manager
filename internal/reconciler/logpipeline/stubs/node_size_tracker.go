package stubs

import "k8s.io/apimachinery/pkg/api/resource"

type NodeSizeTracker struct {
	MaxMemory resource.Quantity
}

func (t *NodeSizeTracker) VPAMaxAllowedMemory() resource.Quantity {
	return t.MaxMemory
}
