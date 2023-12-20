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
package telemetry

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logparser"
)

var (
	testLogParserConfig = logparser.Config{
		ParsersConfigMap:  types.NamespacedName{Name: "test-telemetry-fluent-bit-parser", Namespace: "default"},
		DaemonSet:         types.NamespacedName{Name: "test-telemetry-fluent-bit", Namespace: "default"},
		OverrideConfigMap: types.NamespacedName{Name: "override-config", Namespace: "default"},
	}
)

var _ = Describe("LogParser controller", Ordered, func() {
	const (
		timeout      = time.Second * 10
		interval     = time.Millisecond * 250
		parserConfig = `
		Format regex
		Regex  ^(?<user>[^ ]*) (?<pass>[^ ]*)$
		Time_Key time
		Time_Format %d/%b/%Y:%H:%M:%S %z
		Types user:string pass:string
	`
	)
	var expectefParserCmData = `[PARSER]
    Name regex-parser
    Format regex
    Regex  ^(?<user>[^ ]*) (?<pass>[^ ]*)$
    Time_Key time
    Time_Format %d/%b/%Y:%H:%M:%S %z
    Types user:string pass:string

`
	var logparser = &telemetryv1alpha1.LogParser{
		ObjectMeta: metav1.ObjectMeta{
			Name: "regex-parser",
		},
		Spec: telemetryv1alpha1.LogParserSpec{Parser: parserConfig},
	}
	Context("When creating a log parser", Ordered, func() {
		It("Should successfully create log parser", func() {
			err := k8sClient.Create(ctx, logparser)
			Expect(err).ShouldNot(HaveOccurred())
		})
		It("Should have configuration copied to parser configmap", func() {
			Eventually(func() string {
				var parserCm corev1.ConfigMap
				err := k8sClient.Get(ctx, testLogParserConfig.ParsersConfigMap, &parserCm)
				if err != nil {
					return err.Error()
				}
				return parserCm.Data["parsers.conf"]
			}, timeout, interval).Should(Equal(expectefParserCmData))
		})
	})
	Context("When deleting the log parser ", Ordered, func() {
		It("Should successfully delete the logparser", func() {
			err := k8sClient.Delete(ctx, logparser)
			Expect(err).ShouldNot(HaveOccurred())
		})
		It("Should reset to empty fluent-bit parser configmap", func() {
			Eventually(func() string {
				var parserCm corev1.ConfigMap
				err := k8sClient.Get(ctx, testLogParserConfig.ParsersConfigMap, &parserCm)
				if err != nil {
					return err.Error()
				}
				return parserCm.Data["parsers.conf"]
			}, timeout, interval).Should(Equal(""))
		})
	})
})
