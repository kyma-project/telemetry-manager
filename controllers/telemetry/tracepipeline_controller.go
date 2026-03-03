package telemetry

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

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	"github.com/kyma-project/telemetry-manager/internal/secretwatch"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	"github.com/kyma-project/telemetry-manager/internal/validators/endpoint"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

// TracePipelineController reconciles a TracePipeline object
type TracePipelineController struct {
	client.Client

	reconcileTriggerChan <-chan event.GenericEvent
	reconciler           *tracepipeline.Reconciler
	secretWatchClient    *secretwatch.Client
}

type TracePipelineControllerConfig struct {
	config.Global

	RestConfig         *rest.Config
	OTelCollectorImage string
}

func NewTracePipelineController(config TracePipelineControllerConfig, client client.Client, reconcileTriggerChan <-chan event.GenericEvent, secretWatchClient *secretwatch.Client) (*TracePipelineController, error) {
	pipelineCount := resourcelock.MaxPipelineCount

	if config.UnlimitedPipelines() {
		pipelineCount = resourcelock.UnlimitedPipelineCount
	}

	pipelineLock := resourcelock.NewLocker(
		client,
		types.NamespacedName{
			Name:      names.TracePipelineLock,
			Namespace: config.TargetNamespace(),
		},
		pipelineCount,
	)

	pipelineSync := resourcelock.NewSyncer(
		client,
		types.NamespacedName{
			Name:      names.TracePipelineSync,
			Namespace: config.TargetNamespace(),
		},
	)

	flowHealthProber, err := prober.NewOTelTraceGatewayProber(types.NamespacedName{Name: names.SelfMonitor, Namespace: config.TargetNamespace()})
	if err != nil {
		return nil, err
	}

	transformSpecValidator, err := ottl.NewTransformSpecValidator(ottl.SignalTypeTrace)
	if err != nil {
		return nil, err
	}

	filterSpecValidator, err := ottl.NewFilterSpecValidator(ottl.SignalTypeTrace)
	if err != nil {
		return nil, err
	}

	pipelineValidator := tracepipeline.NewValidator(
		tracepipeline.WithEndpointValidator(&endpoint.Validator{Client: client}),
		tracepipeline.WithTLSCertValidator(tlscert.New(client)),
		tracepipeline.WithSecretRefValidator(&secretref.Validator{Client: client}),
		tracepipeline.WithValidatorPipelineLock(pipelineLock),
		tracepipeline.WithTransformSpecValidator(transformSpecValidator),
		tracepipeline.WithFilterSpecValidator(filterSpecValidator),
	)

	reconciler := tracepipeline.New(
		tracepipeline.WithClient(client),
		tracepipeline.WithGlobals(config.Global),

		tracepipeline.WithFlowHealthProber(flowHealthProber),
		tracepipeline.WithOverridesHandler(overrides.New(config.Global, client)),
		tracepipeline.WithErrorToMessageConverter(&conditions.ErrorToMessageConverter{}),

		tracepipeline.WithPipelineLock(pipelineLock),
		tracepipeline.WithPipelineSyncer(pipelineSync),
		tracepipeline.WithPipelineValidator(pipelineValidator),
		tracepipeline.WithSecretWatcher(secretWatchClient),
	)

	return &TracePipelineController{
		Client:               client,
		reconcileTriggerChan: reconcileTriggerChan,
		reconciler:           reconciler,
		secretWatchClient:    secretWatchClient,
	}, nil
}

func (r *TracePipelineController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.reconciler.Reconcile(ctx, req)
}

func (r *TracePipelineController) SetupWithManager(mgr ctrl.Manager) error {
	b := ctrl.NewControllerManagedBy(mgr).For(&telemetryv1beta1.TracePipeline{})

	b.WatchesRawSource(
		source.Channel(r.reconcileTriggerChan, &handler.EnqueueRequestForObject{}),
	)

	return b.Complete(r)
}
