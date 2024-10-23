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

package logparser

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus"
)

const checksumAnnotationKey = "checksum/logparser-config"

type Config struct {
	ParsersConfigMap  types.NamespacedName
	DaemonSet         types.NamespacedName
	OverrideConfigMap types.NamespacedName
	Overrides         overrides.Config
}

type DaemonSetAnnotator interface {
	SetAnnotation(ctx context.Context, name types.NamespacedName, key, value string) error
}

type Reconciler struct {
	client.Client

	config           Config
	prober           commonstatus.DaemonSetProber
	annotator        DaemonSetAnnotator
	syncer           syncer
	overridesHandler *overrides.Handler
	errorConverter   commonstatus.ErrorToMessageConverter
}

func New(client client.Client, config Config, prober commonstatus.DaemonSetProber, annotator DaemonSetAnnotator, overridesHandler *overrides.Handler, errToMsgConverter commonstatus.ErrorToMessageConverter) *Reconciler {
	return &Reconciler{
		Client:           client,
		config:           config,
		prober:           prober,
		annotator:        annotator,
		syncer:           syncer{client, config},
		overridesHandler: overridesHandler,
		errorConverter:   errToMsgConverter,
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

	var parser telemetryv1alpha1.LogParser
	if err := r.Get(ctx, req.NamespacedName, &parser); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	err = r.doReconcile(ctx, &parser)
	if statusErr := r.updateStatus(ctx, parser.Name); statusErr != nil {
		if err != nil {
			err = fmt.Errorf("failed while updating status: %w: %w", statusErr, err)
		} else {
			err = fmt.Errorf("failed to update status: %w", statusErr)
		}
	}

	return ctrl.Result{}, err
}

func (r *Reconciler) doReconcile(ctx context.Context, parser *telemetryv1alpha1.LogParser) error {
	var allParsers telemetryv1alpha1.LogParserList
	if err := r.List(ctx, &allParsers); err != nil {
		return fmt.Errorf("failed to list log parsers: %w", err)
	}

	if err := ensureFinalizer(ctx, r.Client, parser); err != nil {
		return err
	}

	if err := r.syncer.syncFluentBitConfig(ctx); err != nil {
		return err
	}

	if err := cleanupFinalizerIfNeeded(ctx, r.Client, parser); err != nil {
		return err
	}

	var checksum string

	var err error
	if checksum, err = r.calculateConfigChecksum(ctx); err != nil {
		return err
	}

	if err = r.annotator.SetAnnotation(ctx, r.config.DaemonSet, checksumAnnotationKey, checksum); err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) calculateConfigChecksum(ctx context.Context) (string, error) {
	var cm corev1.ConfigMap
	if err := r.Get(ctx, r.config.ParsersConfigMap, &cm); err != nil {
		return "", fmt.Errorf("failed to get %s/%s ConfigMap: %w", r.config.ParsersConfigMap.Namespace, r.config.ParsersConfigMap.Name, err)
	}

	return configchecksum.Calculate([]corev1.ConfigMap{cm}, nil), nil
}
