/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package logpipeline

import (
	"context"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
)

var (
	ErrUnsupportedOutputType = fmt.Errorf("unsupported output type")
)

type FlowHealthProber interface {
	Probe(ctx context.Context, pipelineName string) (prober.LogPipelineProbeResult, error)
}

type LogPipelineReconciler interface {
	Reconcile(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error
	SupportedOutput() logpipelineutils.Mode
}

type OverridesHandler interface {
	LoadOverrides(ctx context.Context) (*overrides.Config, error)
}

type PipelineSyncer interface {
	TryAcquireLock(ctx context.Context, owner metav1.Object) error
}

type Reconciler struct {
	client.Client

	overridesHandler OverridesHandler
	reconcilers      map[logpipelineutils.Mode]LogPipelineReconciler

	pipelineSyncer PipelineSyncer
}

func New(
	client client.Client,

	overridesHandler OverridesHandler,
	pipelineSyncer PipelineSyncer,
	reconcilers ...LogPipelineReconciler,
) *Reconciler {
	reconcilersMap := make(map[logpipelineutils.Mode]LogPipelineReconciler)
	for _, r := range reconcilers {
		reconcilersMap[r.SupportedOutput()] = r
	}

	return &Reconciler{
		Client:           client,
		overridesHandler: overridesHandler,
		reconcilers:      reconcilersMap,
		pipelineSyncer:   pipelineSyncer,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logf.FromContext(ctx).V(1).Info("Reconciling")

	overrideConfig, err := r.overridesHandler.LoadOverrides(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	if overrideConfig.Logging.Paused {
		logf.FromContext(ctx).V(1).Info("Skipping reconciliation: paused using override config")
		return ctrl.Result{}, nil
	}

	var pipeline telemetryv1alpha1.LogPipeline
	if err := r.Get(ctx, req.NamespacedName, &pipeline); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.pipelineSyncer.TryAcquireLock(ctx, &pipeline); err != nil {
		if errors.Is(err, resourcelock.ErrMaxPipelinesExceeded) {
			logf.FromContext(ctx).V(1).Error(err, "Skipping reconciliation: max pipelines exceeded")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	outputType := logpipelineutils.GetOutputType(&pipeline)
	reconciler, ok := r.reconcilers[outputType]

	if !ok {
		return ctrl.Result{}, fmt.Errorf("%w: %v", ErrUnsupportedOutputType, outputType)
	}

	err = reconciler.Reconcile(ctx, &pipeline)

	return ctrl.Result{}, err
}
