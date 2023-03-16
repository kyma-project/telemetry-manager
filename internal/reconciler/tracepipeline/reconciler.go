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

package tracepipeline

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/kubernetes"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	collectorresources "github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
	"github.com/kyma-project/telemetry-manager/internal/utils/envvar"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

//go:generate mockery --name DeploymentProber --filename deployment_prober.go
type DeploymentProber interface {
	IsReady(ctx context.Context, name types.NamespacedName) (bool, error)
}

type Reconciler struct {
	client.Client
	config           collectorresources.Config
	prober           DeploymentProber
	overridesHandler overrides.GlobalConfigHandler
}

func NewReconciler(client client.Client, config collectorresources.Config, prober DeploymentProber, overridesHandler overrides.GlobalConfigHandler) *Reconciler {
	return &Reconciler{
		Client:           client,
		config:           config,
		prober:           prober,
		overridesHandler: overridesHandler,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	log.V(1).Info("Reconciliation triggered")

	overrideConfig, err := r.overridesHandler.UpdateOverrideConfig(ctx, r.config.OverrideConfigMap)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.overridesHandler.CheckGlobalConfig(overrideConfig.Global); err != nil {
		return ctrl.Result{}, err
	}
	if overrideConfig.Tracing.Paused {
		log.V(1).Info("Skipping reconciliation of tracepipeline as reconciliation is paused")
		return ctrl.Result{}, nil
	}

	var tracePipeline telemetryv1alpha1.TracePipeline
	if err := r.Get(ctx, req.NamespacedName, &tracePipeline); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return ctrl.Result{}, r.doReconcile(ctx, &tracePipeline)
}

func (r *Reconciler) doReconcile(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline) error {
	var err error
	lockAcquired := true

	defer func() {
		if statusErr := r.updateStatus(ctx, pipeline.Name, lockAcquired); statusErr != nil {
			if err != nil {
				err = fmt.Errorf("failed while updating status: %v: %v", statusErr, err)
			} else {
				err = fmt.Errorf("failed to update status: %v", statusErr)
			}
		}
	}()

	lockName := types.NamespacedName{
		Name:      "telemetry-tracepipeline-lock",
		Namespace: r.config.Namespace,
	}
	if err = kubernetes.TryAcquireLock(ctx, r.Client, lockName, pipeline); err != nil {
		lockAcquired = false
		return err
	}

	namespacedBaseName := types.NamespacedName{
		Name:      r.config.BaseName,
		Namespace: r.config.Namespace,
	}
	serviceAccount := commonresources.MakeServiceAccount(namespacedBaseName)
	if err = controllerutil.SetControllerReference(pipeline, serviceAccount, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateServiceAccount(ctx, r, serviceAccount); err != nil {
		return fmt.Errorf("failed to create otel collector service account: %w", err)
	}
	clusterRole := commonresources.MakeClusterRole(namespacedBaseName)
	if err = controllerutil.SetControllerReference(pipeline, clusterRole, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateClusterRole(ctx, r, clusterRole); err != nil {
		return fmt.Errorf("failed to create otel collector cluster role: %w", err)
	}
	clusterRoleBinding := commonresources.MakeClusterRoleBinding(namespacedBaseName)
	if err = controllerutil.SetControllerReference(pipeline, clusterRoleBinding, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateClusterRoleBinding(ctx, r, clusterRoleBinding); err != nil {
		return fmt.Errorf("failed to create otel collector cluster role Binding: %w", err)
	}

	var secretData map[string][]byte
	if secretData, err = secretref.FetchReferencedSecretData(ctx, r, pipeline); err != nil {
		return err
	}
	secretData = appendAuthHeaderIfNeeded(secretData, pipeline.Name, pipeline.Spec.Output.Otlp)

	secret := collectorresources.MakeSecret(r.config, secretData)
	if err = controllerutil.SetControllerReference(pipeline, secret, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateSecret(ctx, r.Client, secret); err != nil {
		return err
	}

	endpoint := getEndpoint(secretData, pipeline.Spec.Output.Otlp)
	isInsecure := isInsecureOutput(endpoint)

	collectorConfig := makeOtelCollectorConfig(pipeline.Spec.Output, isInsecure)
	configMap := collectorresources.MakeConfigMap(r.config, collectorConfig)
	if err = controllerutil.SetControllerReference(pipeline, configMap, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateConfigMap(ctx, r.Client, configMap); err != nil {
		return fmt.Errorf("failed to create otel collector configmap: %w", err)
	}

	configHash := configchecksum.Calculate([]corev1.ConfigMap{*configMap}, []corev1.Secret{*secret})
	deployment := collectorresources.MakeDeployment(r.config, configHash)
	if err = controllerutil.SetControllerReference(pipeline, deployment, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateDeployment(ctx, r.Client, deployment); err != nil {
		return fmt.Errorf("failed to create otel collector deployment: %w", err)
	}

	otlpService := collectorresources.MakeOTLPService(r.config)
	if err = controllerutil.SetControllerReference(pipeline, otlpService, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateService(ctx, r.Client, otlpService); err != nil {
		return fmt.Errorf("failed to create otel collector collector service: %w", err)
	}

	openCensusService := collectorresources.MakeOpenCensusService(r.config)
	if err = controllerutil.SetControllerReference(pipeline, openCensusService, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateService(ctx, r.Client, openCensusService); err != nil {
		return fmt.Errorf("failed to create otel collector open census service: %w", err)
	}

	metricsService := collectorresources.MakeMetricsService(r.config)
	if err = controllerutil.SetControllerReference(pipeline, metricsService, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateService(ctx, r.Client, metricsService); err != nil {
		return fmt.Errorf("failed to create otel collector metrics service: %w", err)
	}

	return nil
}

func isInsecureOutput(endpoint string) bool {
	return len(strings.TrimSpace(endpoint)) > 0 && strings.HasPrefix(endpoint, "http://")
}

// TODO: move common otlp logic to dedicated package
func appendAuthHeaderIfNeeded(secretData map[string][]byte, pipelineName string, output *telemetryv1alpha1.OtlpOutput) map[string][]byte {
	if output.Authentication != nil && output.Authentication.Basic.IsDefined() {
		basicAuth := output.Authentication.Basic
		var basicAuthUser string
		var basicAuthPassword string
		if basicAuth.User.Value != "" {
			basicAuthUser = basicAuth.User.Value
		} else {
			secretKeyRef := basicAuth.User.ValueFrom.SecretKeyRef
			basicAuthUser = string(secretData[envvar.FormatEnvVarName(pipelineName, secretKeyRef.Namespace, secretKeyRef.Name, secretKeyRef.Key)])
		}

		if basicAuth.Password.Value != "" {
			basicAuthPassword = basicAuth.Password.Value
		} else {
			secretKeyRef := basicAuth.Password.ValueFrom.SecretKeyRef
			basicAuthPassword = string(secretData[envvar.FormatEnvVarName(pipelineName, secretKeyRef.Namespace, secretKeyRef.Name, secretKeyRef.Key)])
		}
		secretData["BASIC_AUTH_HEADER"] = []byte(getBasicAuthHeader(basicAuthUser, basicAuthPassword))
	}

	return secretData
}

func getEndpoint(secretData map[string][]byte, output *telemetryv1alpha1.OtlpOutput) string {
	if output.Endpoint.Value != "" {
		return output.Endpoint.Value
	}

	return string(secretData["OTLP_ENDPOINT"])

}

func getBasicAuthHeader(username string, password string) string {
	return fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
}
