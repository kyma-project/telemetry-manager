// Package secretwatch manages Kubernetes secret watches for telemetry pipelines.
//
// # Overview
//
// The secretwatch package helps manage watches on Kubernetes secrets across
// multiple pipelines. It automatically creates watchers for secrets as needed,
// tracks which pipelines reference each secret, and cleans up watchers when
// they're no longer needed.
//
// When a watched secret changes, the client routes a GenericEvent to the
// appropriate pipeline-type channel (trace, metric, or log) to trigger
// reconciliation via controller-runtime.
//
// # Usage
//
// Basic usage with controller-runtime:
//
//	// Create event channels for each pipeline type
//	traceEventChan := make(chan event.GenericEvent)
//	metricEventChan := make(chan event.GenericEvent)
//	logEventChan := make(chan event.GenericEvent)
//
//	// Create the client - events are routed to channels based on pipeline type
//	client, err := secretwatch.NewClient(cfg, traceEventChan, metricEventChan, logEventChan)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Configure which secrets a pipeline should watch.
//	// Watchers are started automatically for new secrets.
//	secrets := []types.NamespacedName{
//		{Namespace: "default", Name: "my-secret"},
//		{Namespace: "default", Name: "another-secret"},
//	}
//	if err := client.SyncWatchers(ctx, pipeline, secrets); err != nil {
//		log.Fatal(err)
//	}
//
//	// Update the watched secrets for a pipeline.
//	// New watchers start, removed watchers stop automatically.
//	newSecrets := []types.NamespacedName{
//		{Namespace: "default", Name: "my-secret"},
//		// "another-secret" watcher stops if no other pipeline uses it
//	}
//	if err := client.SyncWatchers(ctx, pipeline, newSecrets); err != nil {
//		log.Fatal(err)
//	}
//
//	// Remove all watches for a pipeline by passing an empty slice
//	if err := client.SyncWatchers(ctx, pipeline, nil); err != nil {
//		log.Fatal(err)
//	}
//
//	// Or remove a pipeline from all watchers by name and GVK (useful on deletion)
//	if err := client.RemoveFromWatchers(ctx, "pipeline-name", gvk); err != nil {
//		log.Fatal(err)
//	}
//
//	// Graceful shutdown (waits up to 30s for watchers to finish)
//	client.Stop(ctx)
//
// # Thread Safety
//
// All Client methods are safe for concurrent use. The client uses internal
// locking to ensure watcher state remains consistent.
//
// # Watcher Lifecycle
//
// Watchers are managed automatically by SyncWatchers:
//
//  1. When a new secret is added to a pipeline's watch list, a watcher is created and started immediately
//  2. When a secret is removed from all pipelines, its watcher is stopped and cleaned up immediately
//  3. When a pipeline is added to an existing watcher, no new watcher is created (reused)
//
// Watchers automatically reconnect on errors with a 5-second delay.
//
// # Graceful Shutdown
//
// After Stop() is called, the client cannot be reused. Any subsequent calls
// to SyncWatchers or RemoveFromWatchers will return ErrClientStopped.
package secretwatch
