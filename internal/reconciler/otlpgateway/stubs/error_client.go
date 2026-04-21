package stubs

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/resources/names"
)

type ErrorClient struct {
	client.Client

	Err error
}

func (c *ErrorClient) Get(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
	return c.Err
}

type OverrideConfigErrorClient struct {
	client.Client

	Err error
}

func (c *OverrideConfigErrorClient) Get(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	if _, ok := obj.(*corev1.ConfigMap); ok && key.Name == names.OverrideConfigMap {
		return c.Err
	}

	return apierrors.NewNotFound(schema.GroupResource{}, key.Name)
}
