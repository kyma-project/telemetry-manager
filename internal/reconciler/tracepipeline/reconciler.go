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
	"errors"
	"fmt"
	"strings"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	utils "github.com/kyma-project/telemetry-manager/internal/kubernetes"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	corev1 "k8s.io/api/core/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Config struct {
	BaseName          string
	Namespace         string
	OverrideConfigMap types.NamespacedName

	Deployment DeploymentConfig
	Service    ServiceConfig
	Overrides  overrides.Config
}

type DeploymentConfig struct {
	Image             string
	PriorityClassName string
	CPULimit          resource.Quantity
	MemoryLimit       resource.Quantity
	CPURequest        resource.Quantity
	MemoryRequest     resource.Quantity
}

type ServiceConfig struct {
	OTLPServiceName string
}

//go:generate mockery --name DeploymentProber --filename deployment_prober.go
type DeploymentProber interface {
	IsReady(ctx context.Context, name types.NamespacedName) (bool, error)
}

type Reconciler struct {
	client.Client
	config           Config
	prober           DeploymentProber
	overridesHandler overrides.GlobalConfigHandler
}

func NewReconciler(client client.Client, config Config, prober DeploymentProber) *Reconciler {
	var r Reconciler
	r.Client = client
	r.config = config
	r.prober = prober
	return &r
}

func (r *Reconciler) DoReconcile(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline) error {
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

	if err = r.tryAcquireLock(ctx, pipeline); err != nil {
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
	if err = utils.CreateOrUpdateServiceAccount(ctx, r, serviceAccount); err != nil {
		return fmt.Errorf("failed to create otel collector service account: %w", err)
	}
	clusterRole := commonresources.MakeClusterRole(namespacedBaseName)
	if err = controllerutil.SetControllerReference(pipeline, clusterRole, r.Scheme()); err != nil {
		return err
	}
	if err = utils.CreateOrUpdateClusterRole(ctx, r, clusterRole); err != nil {
		return fmt.Errorf("failed to create otel collector cluster role: %w", err)
	}
	clusterRoleBinding := commonresources.MakeClusterRoleBinding(namespacedBaseName)
	if err = controllerutil.SetControllerReference(pipeline, clusterRoleBinding, r.Scheme()); err != nil {
		return err
	}
	if err = utils.CreateOrUpdateClusterRoleBinding(ctx, r, clusterRoleBinding); err != nil {
		return fmt.Errorf("failed to create otel collector cluster role Binding: %w", err)
	}

	var secretData map[string][]byte
	if secretData, err = fetchSecretData(ctx, r, pipeline.Spec.Output.Otlp); err != nil {
		return err
	}
	secret := makeSecret(r.config, secretData)
	if err = controllerutil.SetControllerReference(pipeline, secret, r.Scheme()); err != nil {
		return err
	}
	if err = utils.CreateOrUpdateSecret(ctx, r.Client, secret); err != nil {
		return err
	}
	endpoint := string(secretData[otlpEndpointVariable])
	isInsecure := isInsecureOutput(endpoint)

	collectorConfig := makeOtelCollectorConfig(pipeline.Spec.Output, isInsecure)
	configMap := makeConfigMap(r.config, collectorConfig)
	if err = controllerutil.SetControllerReference(pipeline, configMap, r.Scheme()); err != nil {
		return err
	}
	if err = utils.CreateOrUpdateConfigMap(ctx, r.Client, configMap); err != nil {
		return fmt.Errorf("failed to create otel collector configmap: %w", err)
	}

	configHash := configchecksum.Calculate([]corev1.ConfigMap{*configMap}, []corev1.Secret{*secret})
	deployment := makeDeployment(r.config, configHash)
	if err = controllerutil.SetControllerReference(pipeline, deployment, r.Scheme()); err != nil {
		return err
	}
	if err = utils.CreateOrUpdateDeployment(ctx, r.Client, deployment); err != nil {
		return fmt.Errorf("failed to create otel collector deployment: %w", err)
	}

	otlpService := makeOTLPService(r.config)
	if err = controllerutil.SetControllerReference(pipeline, otlpService, r.Scheme()); err != nil {
		return err
	}
	if err = utils.CreateOrUpdateService(ctx, r.Client, otlpService); err != nil {
		return fmt.Errorf("failed to create otel collector otlp service: %w", err)
	}

	openCensusService := makeOpenCensusService(r.config)
	if err = controllerutil.SetControllerReference(pipeline, openCensusService, r.Scheme()); err != nil {
		return err
	}
	if err = utils.CreateOrUpdateService(ctx, r.Client, openCensusService); err != nil {
		return fmt.Errorf("failed to create otel collector open census service: %w", err)
	}

	metricsService := makeMetricsService(r.config)
	if err = controllerutil.SetControllerReference(pipeline, metricsService, r.Scheme()); err != nil {
		return err
	}
	if err = utils.CreateOrUpdateService(ctx, r.Client, metricsService); err != nil {
		return fmt.Errorf("failed to create otel collector metrics service: %w", err)
	}

	return nil
}

func isInsecureOutput(endpoint string) bool {
	return len(strings.TrimSpace(endpoint)) > 0 && strings.HasPrefix(endpoint, "http://")
}

func (r *Reconciler) tryAcquireLock(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline) error {
	lockName := types.NamespacedName{Name: "telemetry-tracepipeline-lock", Namespace: r.config.Namespace}
	var lock corev1.ConfigMap
	if err := r.Get(ctx, lockName, &lock); err != nil {
		if apierrors.IsNotFound(err) {
			return r.createLock(ctx, lockName, pipeline)
		}
		return fmt.Errorf("failed to get lock: %v", err)
	}

	for _, ref := range lock.GetOwnerReferences() {
		if ref.Name == pipeline.Name && ref.UID == pipeline.UID {
			return nil
		}
	}

	return errors.New("lock is already acquired by another TracePipeline")
}

func (r *Reconciler) createLock(ctx context.Context, name types.NamespacedName, owner *telemetryv1alpha1.TracePipeline) error {
	lock := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
	}
	if err := controllerutil.SetControllerReference(owner, &lock, r.Scheme()); err != nil {
		return fmt.Errorf("failed to set owner reference: %v", err)
	}
	if err := r.Create(ctx, &lock); err != nil {
		return fmt.Errorf("failed to create lock: %v", err)
	}
	return nil
}
