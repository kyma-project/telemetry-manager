// Package secretwatch provides a client for watching Kubernetes secrets and
// tracking which pipelines depend on them.
//
// # Overview
//
// The secretwatch package helps manage watches on Kubernetes secrets across
// multiple pipelines. It automatically creates watchers for secrets as needed,
// tracks which pipelines reference each secret, and cleans up watchers when
// they're no longer needed.
//
// # Features
//
//   - Thread-safe concurrent access
//   - Automatic watcher lifecycle management
//   - Generic events for controller-runtime integration
//   - Declarative secret synchronization
//   - Automatic reconnection on errors
//   - Graceful shutdown with timeout support
//
// # Usage
//
// Basic usage with controller-runtime:
//
//	import (
//		"context"
//
//		"k8s.io/apimachinery/pkg/types"
//		"sigs.k8s.io/controller-runtime/pkg/event"
//		"github.com/kyma-project/telemetry-manager/internal/secretwatch"
//	)
//
//	// Create an event channel for triggering reconciliation
//	eventChan := make(chan event.GenericEvent)
//
//	// Create a client that sends events to the channel
//	client, err := secretwatch.NewClient(cfg, eventChan)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	ctx := context.Background()
//
//	// Configure which secrets a pipeline should watch
//	// Watchers are started automatically
//	secrets := []types.NamespacedName{
//		{Namespace: "default", Name: "my-secret"},
//		{Namespace: "default", Name: "another-secret"},
//	}
//	if err := client.SyncWatchedSecrets(ctx, pipeline, secrets); err != nil {
//		log.Fatal(err)
//	}
//
//	// Later: update the watched secrets for a pipeline
//	// New watchers are started, removed watchers are stopped automatically
//	newSecrets := []types.NamespacedName{
//		{Namespace: "default", Name: "my-secret"},
//		// "another-secret" watcher is stopped automatically if no other pipeline uses it
//	}
//	if err := client.SyncWatchedSecrets(ctx, pipeline, newSecrets); err != nil {
//		log.Fatal(err)
//	}
//
//	// Remove all watches for a pipeline by passing an empty slice
//	if err := client.SyncWatchedSecrets(ctx, pipeline, nil); err != nil {
//		log.Fatal(err)
//	}
//
//	// Graceful shutdown (waits for watchers to finish with 30s timeout)
//	client.Stop()
//
// # Event Channel
//
// When a watched secret changes, the client sends a GenericEvent to the provided
// channel with the pipeline object that references the secret. This integrates
// with controller-runtime's event handling to trigger reconciliation.
//
// # Thread Safety
//
// All Client methods are safe for concurrent use. The client uses internal
// locking to ensure that watcher state remains consistent even when
// SyncWatchedSecrets is called from multiple goroutines.
//
// The watcher's linked pipelines list is also protected by a mutex, ensuring
// thread-safe access when pipelines are added or removed.
//
// # Watcher Lifecycle
//
// Watchers are managed automatically by SyncWatchedSecrets:
//
//  1. When a new secret is added to a pipeline's watch list, a watcher is created and started immediately
//  2. When a secret is removed from all pipelines, its watcher is stopped and cleaned up immediately
//  3. When a pipeline is added to an existing watcher, no new watcher is created (reused)
//
// This automatic lifecycle management means you don't need to explicitly start or stop
// individual watchers - just declare the desired state and the client handles the rest.
//
// # Graceful Shutdown
//
// The Stop() method implements graceful shutdown:
//
//  1. All watcher contexts are canceled (signals them to stop)
//  2. The client waits for all watcher goroutines to finish
//  3. If the timeout is exceeded, a warning is logged but the method returns
//
// This ensures that no goroutines are leaked and that watchers have a chance
// to clean up resources. The default timeout is 30 seconds.
//
// After Stop() is called, the client cannot be reused. Any subsequent calls to
// SyncWatchedSecrets will return ErrClientStopped.
//
// # Lifecycle
//
// The typical lifecycle is:
//
//  1. Create the client with NewClient()
//  2. Configure watches with SyncWatchedSecrets() - watchers start automatically
//  3. Update watches as needed with SyncWatchedSecrets() - watchers start/stop automatically
//  4. Stop all watchers with Stop()
package secretwatch
