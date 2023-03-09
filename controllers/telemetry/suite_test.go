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
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/collector"
	"github.com/kyma-project/telemetry-manager/internal/kubernetes"
	"github.com/kyma-project/telemetry-manager/internal/logger"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	zapLog "go.uber.org/zap"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logparser"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config

var (
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	dynamicLoglevel := zapLog.NewAtomicLevel()
	configureLogLevelOnFly := logger.NewLogReconfigurer(dynamicLoglevel)

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: false,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = telemetryv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 scheme.Scheme,
		MetricsBindAddress:     "localhost:8080",
		Port:                   19443,
		Host:                   "localhost",
		HealthProbeBindAddress: "localhost:8081",
		LeaderElection:         false,
		LeaderElectionID:       "cdd7ef0a.kyma-project.io",
	})
	Expect(err).ToNot(HaveOccurred())

	client := mgr.GetClient()
	overrides := overrides.New(configureLogLevelOnFly, &kubernetes.ConfigmapProber{Client: client})

	kymaSystemNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kyma-system",
		},
	}
	Expect(k8sClient.Create(ctx, kymaSystemNamespace)).Should(Succeed())

	logpipelineController := NewLogPipelineReconciler(
		client,
		logpipeline.NewReconciler(client, testLogPipelineConfig, &kubernetes.DaemonSetProber{Client: client}, overrides),
		testLogPipelineConfig)
	err = logpipelineController.SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	logparserReconciler := NewLogParserReconciler(
		client,
		logparser.NewReconciler(
			client,
			testLogParserConfig,
			&kubernetes.DaemonSetProber{Client: client},
			&kubernetes.DaemonSetAnnotator{Client: client},
			overrides,
		),
		testLogParserConfig,
	)
	err = logparserReconciler.SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	tracepipelineReconciler := NewTracePipelineReconciler(
		client,
		tracepipeline.NewReconciler(client, testTracePipelineConfig, &kubernetes.DeploymentProber{Client: client}, overrides),
	)
	err = tracepipelineReconciler.SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	metricPipelineReconciler := NewMetricPipelineReconciler(client, testMetricPipelineConfig, &kubernetes.DeploymentProber{Client: client}, overrides)
	err = metricPipelineReconciler.SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err := mgr.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()

})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func validatePodAnnotations(deployment appsv1.Deployment) error {
	if value, found := deployment.Spec.Template.ObjectMeta.Annotations["sidecar.istio.io/inject"]; !found || value != "false" {
		return fmt.Errorf("istio sidecar injection for otel collector not disabled")
	}

	if value, found := deployment.Spec.Template.ObjectMeta.Annotations["checksum/config"]; !found || value == "" {
		return fmt.Errorf("configuration hash not found in pod annotations")
	}

	return nil
}

func validateCollectorConfig(configData string) error {
	var config collector.OTELCollectorConfig
	if err := yaml.Unmarshal([]byte(configData), &config); err != nil {
		return err
	}

	if !config.Exporters.OTLP.TLS.Insecure {
		return fmt.Errorf("Insecure flag not set")
	}

	return nil
}
