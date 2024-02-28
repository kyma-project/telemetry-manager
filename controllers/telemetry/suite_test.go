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
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/k8sutils"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logparser"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/metricpipeline"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline"
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
	if testing.Short() {
		t.Skip("skipping envtest")
	}

	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(logzap.New(logzap.WriteTo(GinkgoWriter), logzap.UseDevMode(true)))
	atomicLogLevel := zap.NewAtomicLevel()

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases"), filepath.Join("..", "..", "config", "development")},
		ErrorIfCRDPathMissing: false,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = telemetryv1alpha1.AddToScheme(clientgoscheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = operatorv1alpha1.AddToScheme(clientgoscheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: clientgoscheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	syncPeriod := 1 * time.Second
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:  clientgoscheme.Scheme,
		Metrics: metricsserver.Options{BindAddress: "localhost:8080"},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: 19443,
			Host: "localhost",
		}),
		HealthProbeBindAddress: "localhost:8081",
		LeaderElection:         false,
		LeaderElectionID:       "cdd7ef0a.kyma-project.io",
		Cache: cache.Options{
			SyncPeriod: &syncPeriod,
		},
	})
	Expect(err).ToNot(HaveOccurred())

	client := mgr.GetClient()
	var handlerConfig overrides.HandlerConfig
	overridesHandler := overrides.New(client, atomicLogLevel, handlerConfig)

	kymaSystemNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "telemetry-system",
		},
	}
	Expect(k8sClient.Create(ctx, kymaSystemNamespace)).Should(Succeed())

	logpipelineController := NewLogPipelineReconciler(
		client,
		logpipeline.NewReconciler(client, testLogPipelineConfig, &k8sutils.DaemonSetProber{Client: client}, overridesHandler),
		testLogPipelineConfig)
	err = logpipelineController.SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	logparserReconciler := NewLogParserReconciler(
		client,
		logparser.NewReconciler(
			client,
			testLogParserConfig,
			&k8sutils.DaemonSetProber{Client: client},
			&k8sutils.DaemonSetAnnotator{Client: client},
			overridesHandler,
		),
		testLogParserConfig,
	)
	err = logparserReconciler.SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	tracepipelineReconciler := NewTracePipelineReconciler(
		client,
		tracepipeline.NewReconciler(client, testTracePipelineReconcilerConfig, &k8sutils.DeploymentProber{Client: client}, overridesHandler),
	)
	err = tracepipelineReconciler.SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	metricPipelineReconciler := NewMetricPipelineReconciler(
		client,
		metricpipeline.NewReconciler(client, testMetricPipelineReconcilerConfig, &k8sutils.DeploymentProber{Client: client},
			&k8sutils.DaemonSetProber{Client: client}, overridesHandler))
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
