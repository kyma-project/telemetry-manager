package logpipeline

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	logpipelinevalidationmocks "github.com/kyma-project/telemetry-manager/webhook/logpipeline/validation/mocks"
)

func TestHandle(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	t.Run("should execute validations for max pipelines, variables, files", func(t *testing.T) {
		maxPipelinesValidatorMock := &logpipelinevalidationmocks.MaxPipelinesValidator{}
		variableValidatorMock := &logpipelinevalidationmocks.VariablesValidator{}
		fileValidatorMock := &logpipelinevalidationmocks.FilesValidator{}

		maxPipelinesValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)
		variableValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)
		fileValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)

		logPipeline := testutils.NewLogPipelineBuilder().Build()
		pipelineJSON, _ := json.Marshal(logPipeline)
		admissionRequest := admissionv1.AdmissionRequest{
			Object: runtime.RawExtension{Raw: pipelineJSON},
		}
		request := admission.Request{
			AdmissionRequest: admissionRequest,
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects().Build()
		validationConfig := &telemetryv1alpha1.LogPipelineValidationConfig{DeniedOutPutPlugins: []string{}, DeniedFilterPlugins: []string{}}
		logPipelineValidatingWebhookHandler := NewValidatingWebhookHandler(fakeClient, variableValidatorMock, maxPipelinesValidatorMock, fileValidatorMock, admission.NewDecoder(clientgoscheme.Scheme), validationConfig)

		response := logPipelineValidatingWebhookHandler.Handle(context.Background(), request)
		require.True(t, response.Allowed)

		variableValidatorMock.AssertExpectations(t)
		maxPipelinesValidatorMock.AssertExpectations(t)
		fileValidatorMock.AssertExpectations(t)
	})

	t.Run("should execute validations for API semantic", func(t *testing.T) {
		maxPipelinesValidatorMock := &logpipelinevalidationmocks.MaxPipelinesValidator{}
		variableValidatorMock := &logpipelinevalidationmocks.VariablesValidator{}
		fileValidatorMock := &logpipelinevalidationmocks.FilesValidator{}

		maxPipelinesValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)
		variableValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)
		fileValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)

		logPipeline := testutils.NewLogPipelineBuilder().WithName("denied-filter").WithCustomFilter("Name kubernetes").Build()
		pipelineJSON, _ := json.Marshal(logPipeline)
		admissionRequest := admissionv1.AdmissionRequest{
			Object: runtime.RawExtension{Raw: pipelineJSON},
		}
		request := admission.Request{
			AdmissionRequest: admissionRequest,
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects().Build()
		validationConfig := &telemetryv1alpha1.LogPipelineValidationConfig{DeniedOutPutPlugins: []string{}, DeniedFilterPlugins: []string{"kubernetes"}}
		logPipelineValidatingWebhookHandler := NewValidatingWebhookHandler(fakeClient, variableValidatorMock, maxPipelinesValidatorMock, fileValidatorMock, admission.NewDecoder(clientgoscheme.Scheme), validationConfig)

		response := logPipelineValidatingWebhookHandler.Handle(context.Background(), request)
		require.False(t, response.Allowed)
		require.Equal(t, int32(http.StatusForbidden), response.Result.Code)
		require.Equal(t, "InvalidConfiguration", string(response.Result.Reason))
		require.Contains(t, response.Result.Message, "filter plugin 'kubernetes' is forbidden")
	})

	t.Run("should return a warning when a custom plugin is used", func(t *testing.T) {
		maxPipelinesValidatorMock := &logpipelinevalidationmocks.MaxPipelinesValidator{}
		variableValidatorMock := &logpipelinevalidationmocks.VariablesValidator{}
		fileValidatorMock := &logpipelinevalidationmocks.FilesValidator{}

		maxPipelinesValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)
		variableValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)
		fileValidatorMock.On("Validate", mock.Anything, mock.Anything).Return(nil).Times(1)

		logPipeline := testutils.NewLogPipelineBuilder().WithName("custom-output").WithCustomOutput("Name stdout").Build()
		pipelineJSON, _ := json.Marshal(logPipeline)
		admissionRequest := admissionv1.AdmissionRequest{
			Object: runtime.RawExtension{Raw: pipelineJSON},
		}
		request := admission.Request{
			AdmissionRequest: admissionRequest,
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects().Build()
		validationConfig := &telemetryv1alpha1.LogPipelineValidationConfig{DeniedOutPutPlugins: []string{}, DeniedFilterPlugins: []string{}}
		logPipelineValidatingWebhookHandler := NewValidatingWebhookHandler(fakeClient, variableValidatorMock, maxPipelinesValidatorMock, fileValidatorMock, admission.NewDecoder(clientgoscheme.Scheme), validationConfig)

		response := logPipelineValidatingWebhookHandler.Handle(context.Background(), request)
		require.True(t, response.Allowed)
		require.Contains(t, response.Warnings, "Logpipeline 'custom-output' uses unsupported custom filters or outputs. We recommend changing the pipeline to use supported filters or output. See the documentation: https://kyma-project.io/#/telemetry-manager/user/02-logs")
	})
}
