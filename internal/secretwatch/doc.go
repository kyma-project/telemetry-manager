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
//   - Event callbacks for secret changes
//   - Declarative secret synchronization
//   - Automatic reconnection on errors
//   - Graceful shutdown with timeout support
//
// # Usage
//
// Basic usage:
//
//	import (
//		"context"
//		"fmt"
//		"time"
//
//		"k8s.io/apimachinery/pkg/types"
//		"github.com/kyma-project/telemetry-manager/internal/secretwatch"
//	)
//
//	// Create a client with an event handler
//	:= func(secretName types.NamespacedName, eventType secretwatch.EventType, pipelines []string)
//		fmt.Printf("Secret %s changed (type: %s), affects pipelines: %v\n",
//			secretName, eventType, pipelines)
//	}
//
//	client, err := secretwatch.NewClient(cfg, handler)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	ctx := context.Background()
//
//	// Configure which secrets each pipeline should watch
//	// Watchers are started automatically
//	secrets := []types.NamespacedName{
//		{Namespace: "default", Name: "my-secret"},
//		{Namespace: "default", Name: "another-secret"},
//	}
//	client.SyncWatchedSecrets(ctx, "pipeline-1", secrets)
//
//	// Later: update the watched secrets for a pipeline
//	// New watchers are started, removed watchers are stopped automatically
//	newSecrets := []types.NamespacedName{
//		{Namespace: "default", Name: "my-secret"},
//		// "another-secret" watcher is stopped automatically
//	}
//	client.SyncWatchedSecrets(ctx, "pipeline-1", newSecrets)
//
//	// Remove all watches for a pipeline (stops all its watchers)
//	client.RemovePipeline(ctx, "pipeline-1")
//
//	// Graceful shutdown (waits for watchers to finish with 30s timeout)
//	client.Stop()
//
//	// Or use custom timeout
//	client.StopWithTimeout(10 * time.Second)
//
// # Event Handler
//
// The EventHandler callback receives three parameters:
//   - secretName: The namespaced name of the secret that changed
//   - eventType: The type of change (Added, Modified, Deleted, etc.)
//   - linkedPipelines: List of pipeline names that reference this secret
//
// If no event handler is provided (nil), events will only be logged using
// controller-runtime's structured logging.
//
// # Thread Safety
//
// All Client methods are safe for concurrent use. The client uses internal
// locking to ensure that watcher state remains consistent even when
// SyncWatchedSecrets is called from multiple goroutines.
//
// The watcher's linkedPipelines list is also protected by a mutex, and the
// client uses thread-safe methods (addPipeline, removePipeline, hasPipeline)
// to modify it.
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
// The Stop() and StopWithTimeout() methods implement graceful shutdown:
//
//  1. All watcher contexts are canceled (signals them to stop)
//  2. The client waits for all watcher goroutines to finish
//  3. If the timeout is exceeded, a warning is logged but the method returns
//
// This ensures that no goroutines are leaked and that watchers have a chance
// to clean up resources. The default timeout is 30 seconds.
//
// # Lifecycle
//
// The typical lifecycle is:
//
//  1. Create the client with NewClient()
//  2. Configure watches with SyncWatchedSecrets() - watchers start automatically
//  3. Update watches as needed with SyncWatchedSecrets() - watchers start/stop automatically
//  4. Stop all watchers with Stop() or StopWithTimeout()
package secretwatch
