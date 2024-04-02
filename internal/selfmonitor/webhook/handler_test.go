package webhook

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type errReader struct{}

func (errReader) Read(p []byte) (n int, err error) {
	return 0, assert.AnError
}

func TestHandler(t *testing.T) {
	tests := []struct {
		name                 string
		requestMethod        string
		requestBody          io.Reader
		resources            []client.Object
		expectedStatus       int
		expectedResourceName string
		expectedResourceType any
	}{
		{
			name:          "alert matches pipeline with same name",
			requestMethod: http.MethodPost,
			requestBody:   bytes.NewBuffer([]byte(`[{"labels":{"alertname":"MetricGatewayExporterDroppedData","exporter":"otlp/cls"}}]`)),
			resources: []client.Object{
				ptr.To(testutils.NewMetricPipelineBuilder().WithName("cls").Build()),
			},
			expectedStatus:       http.StatusOK,
			expectedResourceName: "cls",
			expectedResourceType: &telemetryv1alpha1.MetricPipeline{},
		},
		{
			name:          "alert does not match pipeline with other name",
			requestMethod: http.MethodPost,
			requestBody:   bytes.NewBuffer([]byte(`[{"labels":{"alertname":"MetricGatewayExporterDroppedData","exporter":"otlp/dynatrace"}}]`)),
			resources: []client.Object{
				ptr.To(testutils.NewTracePipelineBuilder().WithName("cls").Build()),
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:          "alert does not match pipeline of other type",
			requestMethod: http.MethodPost,
			requestBody:   bytes.NewBuffer([]byte(`[{"labels":{"alertname":"MetricGatewayExporterDroppedData","exporter":"otlp/cls"}}]`)),
			resources: []client.Object{
				ptr.To(testutils.NewTracePipelineBuilder().WithName("cls").Build()),
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid method",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "failed to read request body",
			requestMethod:  http.MethodPost,
			requestBody:    errReader{},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ch := make(chan event.GenericEvent, 1)

			noopLogger := logr.New(logf.NullLogSink{})

			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = telemetryv1alpha1.AddToScheme(scheme)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.resources...).Build()
			handler := NewHandler(fakeClient, WithSubscriber(ch), WithLogger(noopLogger))

			req, err := http.NewRequest(tc.requestMethod, "/", tc.requestBody)
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			require.Equal(t, tc.expectedStatus, rr.Code)
			if tc.expectedResourceName != "" {
				require.NotEmpty(t, ch)
				event := <-ch
				require.NotNil(t, event.Object)
				require.Equal(t, tc.expectedResourceName, event.Object.GetName())
				require.IsType(t, tc.expectedResourceType, event.Object)
			} else {
				require.Empty(t, ch)
			}
		})
	}
}
