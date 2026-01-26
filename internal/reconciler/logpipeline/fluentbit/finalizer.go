package fluentbit

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

const (
	sectionsFinalizer = "FLUENT_BIT_SECTIONS_CONFIG_MAP"
	filesFinalizer    = "FLUENT_BIT_FILES"
)

// TODO: remove cleanup code after rollout telemetry 1.57.0
func cleanupFinalizers(ctx context.Context, client client.Client, pipeline *telemetryv1beta1.LogPipeline) error {
	var changed bool

	if controllerutil.ContainsFinalizer(pipeline, sectionsFinalizer) {
		controllerutil.RemoveFinalizer(pipeline, sectionsFinalizer)

		changed = true
	}

	if controllerutil.ContainsFinalizer(pipeline, filesFinalizer) {
		controllerutil.RemoveFinalizer(pipeline, filesFinalizer)

		changed = true
	}

	if !changed {
		return nil
	}

	return client.Update(ctx, pipeline)
}
