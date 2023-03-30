// /*
// Copyright 2021.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */
package logpipeline

import (
	"context"
	"crypto/tls"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/webhook/logpipeline/mocks"
	validationmocks "github.com/kyma-project/telemetry-manager/webhook/logpipeline/validation/mocks"

	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	k8sWebhook "sigs.k8s.io/controller-runtime/pkg/webhook"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	fluentBitConfigMapName     = "telemetry-fluent-bit"
	fluentBitFileConfigMapName = "telemetry-fluent-bit-files"
	controllerNamespace        = "default"
)

var (
	k8sClient                 client.Client
	testEnv                   *envtest.Environment
	ctx                       context.Context
	cancel                    context.CancelFunc
	variableValidatorMock     *validationmocks.VariablesValidator
	maxPipelinesValidatorMock *validationmocks.MaxPipelinesValidator
	fileValidatorMock         *validationmocks.FilesValidator
	dryRunnerMock             *mocks.DryRunner
	mutex                     sync.Mutex
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "LogPipeline Webhook Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "config", "webhook")},
		},
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = telemetryv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// start logPipeline webhook server using Manager
	webhookInstallOptions := &testEnv.WebhookInstallOptions
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 scheme.Scheme,
		Host:                   webhookInstallOptions.LocalServingHost,
		Port:                   webhookInstallOptions.LocalServingPort,
		CertDir:                webhookInstallOptions.LocalServingCertDir,
		LeaderElection:         false,
		MetricsBindAddress:     "localhost:8082",
		HealthProbeBindAddress: "localhost:8083",
	})
	Expect(err).NotTo(HaveOccurred())

	variableValidatorMock = &validationmocks.VariablesValidator{}
	dryRunnerMock = &mocks.DryRunner{}
	maxPipelinesValidatorMock = &validationmocks.MaxPipelinesValidator{}
	fileValidatorMock = &validationmocks.FilesValidator{}
	validationConfig := &telemetryv1alpha1.LogPipelineValidationConfig{DeniedOutPutPlugins: []string{"lua", "stdout"}, DeniedFilterPlugins: []string{"stdout"}}

	logPipelineValidator := NewValidatingWebhookHandler(mgr.GetClient(), variableValidatorMock, maxPipelinesValidatorMock, fileValidatorMock, dryRunnerMock, validationConfig)

	By("registering LogPipeline webhook")
	mgr.GetWebhookServer().Register(
		"/validate-logpipeline",
		&k8sWebhook.Admission{Handler: logPipelineValidator})

	//+kubebuilder:scaffold:webhook

	go func() {
		defer GinkgoRecover()
		mutex.Lock()
		err := mgr.Start(ctx)
		mutex.Unlock()
		Expect(err).NotTo(HaveOccurred())
	}()

	// wait for the webhook server to get ready
	dialer := &net.Dialer{Timeout: time.Second}
	addrPort := fmt.Sprintf("%s:%d", webhookInstallOptions.LocalServingHost, webhookInstallOptions.LocalServingPort)
	Eventually(func() error {
		conn, err := tls.DialWithDialer(dialer, "tcp", addrPort, &tls.Config{InsecureSkipVerify: true}) /* #nosec */
		if err != nil {
			return err
		}
		if err := conn.Close(); err != nil {
			return err
		}
		return nil
	}).Should(Succeed())

	By("creating the necessary resources")
	err = createResources()
	Expect(err).NotTo(HaveOccurred())

})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func createResources() error {
	cmFluentBit := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fluentBitConfigMapName,
			Namespace: controllerNamespace,
		},
		Data: map[string]string{
			"fluent-bit.conf": `@INCLUDE dynamic/*.conf
[SERVICE]
    Daemon Off
    Flush 1
    Parsers_File custom_parsers.conf
    Parsers_File dynamic-parsers/parsers.conf

[INPUT]
    Name tail
    Path /var/log/containers/*.log
    multiline.parser docker, cri
    Tag kube.*
    Mem_Buf_Limit 5MB
    Skip_Long_Lines On
    storage.type  filesystem
`,
		},
	}
	err := k8sClient.Create(ctx, &cmFluentBit)
	if err != nil {
		return err
	}
	cmFile := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fluentBitFileConfigMapName,
			Namespace: controllerNamespace,
		},
		Data: map[string]string{
			"labelmap.json": `
kubernetes:
  namespace_name: namespace
  labels:
    app: app
    "app.kubernetes.io/component": component
    "app.kubernetes.io/name": app
    "serverless.kyma-project.io/function-name": function
     host: node
  container_name: container
  pod_name: pod
stream: stream`,
		},
	}
	err = k8sClient.Create(ctx, &cmFile)

	return err
}
