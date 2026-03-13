package k8sclients

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNoopWatchClient(t *testing.T) {
	inner := fake.NewClientBuilder().Build()
	noop := &noopWatchClient{Client: inner}

	w, err := noop.Watch(t.Context(), &corev1.ConfigMapList{})
	require.Nil(t, w)
	require.ErrorIs(t, err, ErrNotImplemented)
}
