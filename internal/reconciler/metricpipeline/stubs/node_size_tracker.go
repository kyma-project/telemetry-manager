package stubs

import "k8s.io/apimachinery/pkg/api/resource"

type NodeSizeTracker struct {
	MaxMemory resource.Quantity
}

func (t *NodeSizeTracker) VpaMaxAllowedMemory() resource.Quantity {
	return t.MaxMemory
}
