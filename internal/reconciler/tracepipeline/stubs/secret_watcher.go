package stubs

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SecretWatcher struct {
	err error
}

func NewSecretWatcher(err error) *SecretWatcher {
	return &SecretWatcher{
		err: err,
	}
}

func (s *SecretWatcher) SyncWatchedSecrets(ctx context.Context, pipeline client.Object, secrets []types.NamespacedName) error {
	return s.err
}
