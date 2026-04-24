package stubs

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ErrorClient struct {
	client.Client

	Err error
}

func (c *ErrorClient) Get(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
	return c.Err
}
