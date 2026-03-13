package k8sclients

import (
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrNotImplemented = errors.New("not implemented")

// noopWatchClient wraps a client.Client to satisfy the client.WithWatch interface
// required by interceptor.NewClient. Watch is not used by the interceptors.
type noopWatchClient struct {
	client.Client
}

func (n *noopWatchClient) Watch(_ context.Context, _ client.ObjectList, _ ...client.ListOption) (watch.Interface, error) {
	return nil, ErrNotImplemented
}
