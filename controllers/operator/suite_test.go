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

package operator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/telemetry"
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
	ctx, cancel = context.WithCancel(context.TODO())
	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = operatorv1alpha1.AddToScheme(clientgoscheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = telemetryv1alpha1.AddToScheme(clientgoscheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: clientgoscheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:  clientgoscheme.Scheme,
		Metrics: metricsserver.Options{BindAddress: "localhost:8085"},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: 19443,
			Host: "localhost",
		}),
		HealthProbeBindAddress: "localhost:8088",
		LeaderElection:         false,
		LeaderElectionID:       "cdd7ef0a.kyma-project.io",
	})
	Expect(err).ToNot(HaveOccurred())

	certDir, err := os.MkdirTemp("", "certificate")
	Expect(err).ToNot(HaveOccurred())
	defer func(path string) {
		deleteErr := os.RemoveAll(path)
		Expect(deleteErr).ToNot(HaveOccurred())
	}(certDir)

	selfMonitorConfig := telemetry.SelfMonitorConfig{
		Enabled: false,
	}
	webhookConfig := telemetry.WebhookConfig{
		Enabled: false,
	}
	config := telemetry.Config{
		Traces: telemetry.TracesConfig{
			OTLPServiceName: "traceFoo",
			Namespace:       "kyma-system",
		},
		Metrics: telemetry.MetricsConfig{
			OTLPServiceName: "metricFoo",
			Namespace:       "kyma-system",
		},
		Webhook:                webhookConfig,
		OverridesConfigMapName: types.NamespacedName{Name: "telemetry-override-config", Namespace: "kyma-system"},
		SelfMonitor:            selfMonitorConfig,
	}
	client := mgr.GetClient()

	atomicLogLevel := zap.NewAtomicLevel()
	var handlerConfig overrides.HandlerConfig
	overridesHandler := overrides.New(client, atomicLogLevel, handlerConfig)

	telemetryReconciler := NewTelemetryReconciler(client,
		telemetry.NewReconciler(client, mgr.GetScheme(), config, overridesHandler),
		config)
	err = telemetryReconciler.SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

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
