package logpipeline

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

var testLogPipeline = types.NamespacedName{
	Name:      "log-pipeline",
	Namespace: controllerNamespace,
}

// getLogPipeline creates a standard LopPipeline
func getLogPipeline() *telemetryv1alpha1.LogPipeline {
	file := telemetryv1alpha1.FileMount{
		Name:    "1st-file",
		Content: "file-content",
	}
	output := telemetryv1alpha1.Output{
		Custom: "Name   foo\n",
	}
	logPipeline := &telemetryv1alpha1.LogPipeline{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "telemetry.kyma-project.io/v1alpha1",
			Kind:       "LogPipeline",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      testLogPipeline.Name,
			Namespace: testLogPipeline.Namespace,
		},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.Output{Custom: output.Custom},
			Files:  []telemetryv1alpha1.FileMount{file},
		},
	}

	return logPipeline
}

// getLogPipeline creates a standard LopPipeline
func getLokiPipeline() *telemetryv1alpha1.LogPipeline {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "telemetry.kyma-project.io/v1alpha1",
			Kind:       "LogPipeline",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      testLogPipeline.Name,
			Namespace: testLogPipeline.Namespace,
		},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.Output{Loki: &telemetryv1alpha1.LokiOutput{
				URL: telemetryv1alpha1.ValueType{
					Value: "http://foo.bar",
				},
				Labels:     map[string]string{"job": "telemetry-fluent-bit"},
				RemoveKeys: []string{"kubernetes", "stream"},
			}},
		},
	}

	return logPipeline
}

var invalidOutput = telemetryv1alpha1.Output{
	Custom: "Name   stdout\n",
}

var invalidFilter = telemetryv1alpha1.Filter{
	Custom: "Name   stdout\n",
}

var _ = Describe("LogPipeline webhook", Ordered, func() {
	Context("When creating LogPipeline", Ordered, func() {
		AfterEach(func() {
			logPipeline := getLogPipeline()
			err := k8sClient.Delete(ctx, logPipeline, client.GracePeriodSeconds(0))
			if !apierrors.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("Should accept valid LogPipeline", func() {
			variableValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)
			maxPipelinesValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)
			fileValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)
			dryRunnerMock.On("RunPipeline", mock.Anything, mock.Anything).Return(nil).Times(1)

			logPipeline := getLokiPipeline()
			err := k8sClient.Create(ctx, logPipeline)

			Expect(err).NotTo(HaveOccurred())
		})

		It("Should reject LogPipeline with invalid indentation in yaml", func() {
			variableValidatorMock.On("Validate", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(1)
			maxPipelinesValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)
			fileValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)

			configErr := errors.New("Error in line 4: Invalid indentation level")
			dryRunnerMock.On("RunPipeline", mock.Anything, mock.Anything).Return(configErr).Times(1)

			logPipeline := getLogPipeline()
			err := k8sClient.Create(ctx, logPipeline)

			Expect(err).To(HaveOccurred())
			var status apierrors.APIStatus
			errors.As(err, &status)

			Expect(StatusReasonConfigurationError).To(Equal(string(status.Status().Reason)))
			Expect(status.Status().Message).To(ContainSubstring(configErr.Error()))
		})

		It("Should reject LogPipeline with forbidden plugin", func() {
			variableValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)
			pluginErr := errors.New("filter plugin 'stdout' is forbidden")
			maxPipelinesValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)

			logPipeline := getLogPipeline()
			logPipeline.Spec.Filters = []telemetryv1alpha1.Filter{invalidFilter}
			err := k8sClient.Create(ctx, logPipeline)

			Expect(err).To(HaveOccurred())
			var status apierrors.APIStatus
			errors.As(err, &status)

			Expect(StatusReasonConfigurationError).To(Equal(string(status.Status().Reason)))
			Expect(status.Status().Message).To(ContainSubstring(pluginErr.Error()))
		})

		It("Should reject LogPipeline with invalid output", func() {
			maxPipelinesValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)

			variableValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)
			outputErr := errors.New("output plugin 'stdout' is forbidden")

			logPipeline := getLogPipeline()
			logPipeline.Spec.Output = invalidOutput
			err := k8sClient.Create(ctx, logPipeline)

			Expect(err).To(HaveOccurred())
			var status apierrors.APIStatus
			errors.As(err, &status)

			Expect(StatusReasonConfigurationError).To(Equal(string(status.Status().Reason)))
			Expect(status.Status().Message).To(ContainSubstring(outputErr.Error()))
		})

		It("Should reject LogPipeline when exceeding pipeline limit", func() {
			maxPipelinesErr := errors.New("too many pipelines")
			maxPipelinesValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(maxPipelinesErr).Times(1)

			logPipeline := getLogPipeline()
			err := k8sClient.Create(ctx, logPipeline)

			Expect(err).To(HaveOccurred())
			var status apierrors.APIStatus
			errors.As(err, &status)

			Expect(StatusReasonConfigurationError).To(Equal(string(status.Status().Reason)))
			Expect(status.Status().Message).To(ContainSubstring(maxPipelinesErr.Error()))
		})

		It("Should reject LogPipeline when duplicate filename is used", func() {
			maxPipelinesValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)
			variableValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)

			fileError := errors.New("duplicate file name: 1st-file")
			fileValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(fileError).Times(1)
			dryRunnerMock.On("RunPipeline", mock.Anything, mock.Anything).Return(nil).Times(1)

			logPipeline := getLogPipeline()
			err := k8sClient.Create(ctx, logPipeline)

			Expect(err).To(HaveOccurred())
			var status apierrors.APIStatus
			errors.As(err, &status)

			Expect(StatusReasonConfigurationError).To(Equal(string(status.Status().Reason)))
			Expect(status.Status().Message).To(ContainSubstring(fileError.Error()))
		})

	})

	Context("When updating LogPipeline", Ordered, func() {
		It("Should create valid LogPipeline", func() {
			maxPipelinesValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)
			variableValidatorMock.On("Validate", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(1)
			fileValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)
			dryRunnerMock.On("RunPipeline", mock.Anything, mock.Anything).Return(nil).Times(1)

			logPipeline := getLogPipeline()
			err := k8sClient.Create(ctx, logPipeline)

			Expect(err).NotTo(HaveOccurred())
		})

		It("Should update previously created valid LogPipeline", func() {
			maxPipelinesValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)
			variableValidatorMock.On("Validate", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(1)
			fileValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)
			dryRunnerMock.On("RunPipeline", mock.Anything, mock.Anything).Return(nil).Times(1)

			var logPipeline telemetryv1alpha1.LogPipeline
			err := k8sClient.Get(ctx, testLogPipeline, &logPipeline)
			Expect(err).NotTo(HaveOccurred())

			logPipeline.Spec.Files = append(logPipeline.Spec.Files, telemetryv1alpha1.FileMount{
				Name:    "2nd-file",
				Content: "file content",
			})
			err = k8sClient.Update(ctx, &logPipeline)

			Expect(err).NotTo(HaveOccurred())
		})

		It("Should reject new update of previously created LogPipeline", func() {
			maxPipelinesValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)
			variableValidatorMock.On("Validate", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(1)
			outputErr := errors.New("configuration section must have name attribute")

			var logPipeline telemetryv1alpha1.LogPipeline
			err := k8sClient.Get(ctx, testLogPipeline, &logPipeline)
			Expect(err).NotTo(HaveOccurred())

			logPipeline.Spec.Output = telemetryv1alpha1.Output{
				Custom: "invalid content",
			}

			err = k8sClient.Update(ctx, &logPipeline)

			Expect(err).To(HaveOccurred())
			var status apierrors.APIStatus
			errors.As(err, &status)

			Expect(StatusReasonConfigurationError).To(Equal(string(status.Status().Reason)))
			Expect(status.Status().Message).To(ContainSubstring(outputErr.Error()))
		})

		It("Should reject new update with invalid plugin usage of previously created LogPipeline", func() {
			pluginErr := errors.New("output plugin 'stdout' is forbidden")
			maxPipelinesValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)
			variableValidatorMock.On("Validate", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(1)

			var logPipeline telemetryv1alpha1.LogPipeline
			err := k8sClient.Get(ctx, testLogPipeline, &logPipeline)
			Expect(err).NotTo(HaveOccurred())

			logPipeline.Spec.Output = invalidOutput

			logPipeline.Spec.Files = append(logPipeline.Spec.Files, telemetryv1alpha1.FileMount{
				Name:    "3rd-file",
				Content: "file content",
			})

			err = k8sClient.Update(ctx, &logPipeline)

			Expect(err).To(HaveOccurred())
			var status apierrors.APIStatus
			errors.As(err, &status)

			Expect(StatusReasonConfigurationError).To(Equal(string(status.Status().Reason)))
			Expect(status.Status().Message).To(ContainSubstring(pluginErr.Error()))
		})

		It("Should delete LogPipeline", func() {
			logPipeline := getLogPipeline()
			err := k8sClient.Delete(ctx, logPipeline, client.GracePeriodSeconds(0))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
