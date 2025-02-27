package webhook

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

type errReader struct{}

func (errReader) Read(p []byte) (n int, err error) {
	return 0, assert.AnError
}

func TestHandler(t *testing.T) {
	tests := []struct {
		name                       string
		requestMethod              string
		requestBody                io.Reader
		resources                  []client.Object
		expectedStatus             int
		metricPipelinesToReconcile []string
		tracePipelinesToReconcile  []string
		logPipelinesToReconcile    []string
	}{
		{
			name:          "alert matches metric pipeline with same name",
			requestMethod: http.MethodPost,
			requestBody:   bytes.NewBuffer([]byte(`[{"labels":{"alertname":"MetricGatewayExporterDroppedData","pipeline_name":"cls"}}]`)),
			resources: []client.Object{
				ptr.To(testutils.NewMetricPipelineBuilder().WithName("cls").Build()),
			},
			expectedStatus:             http.StatusOK,
			metricPipelinesToReconcile: []string{"cls"},
		},
		{
			name:          "alert matches trace pipeline with same name",
			requestMethod: http.MethodPost,
			requestBody:   bytes.NewBuffer([]byte(`[{"labels":{"alertname":"TraceGatewayExporterDroppedData","pipeline_name":"cls"}}]`)),
			resources: []client.Object{
				ptr.To(testutils.NewTracePipelineBuilder().WithName("cls").Build()),
			},
			expectedStatus:            http.StatusOK,
			tracePipelinesToReconcile: []string{"cls"},
		},
		{
			name:          "alert matches log pipeline with same name",
			requestMethod: http.MethodPost,
			requestBody:   bytes.NewBuffer([]byte(`[{"labels":{"alertname":"FluentBitLogAgentExporterDroppedLogs","pipeline_name":"cls"}}]`)),
			resources: []client.Object{
				ptr.To(testutils.NewLogPipelineBuilder().WithName("cls").Build()),
			},
			expectedStatus:          http.StatusOK,
			logPipelinesToReconcile: []string{"cls"},
		},
		{
			name:          "alert does not match pipeline with other name",
			requestMethod: http.MethodPost,
			requestBody:   bytes.NewBuffer([]byte(`[{"labels":{"alertname":"MetricGatewayExporterDroppedData","pipeline_name":"dynatrace"}}]`)),
			resources: []client.Object{
				ptr.To(testutils.NewTracePipelineBuilder().WithName("cls").Build()),
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:          "alert does not match pipeline of other type",
			requestMethod: http.MethodPost,
			requestBody:   bytes.NewBuffer([]byte(`[{"labels":{"alertname":"MetricGatewayExporterDroppedData","pipeline_name":"cls"}}]`)),
			resources: []client.Object{
				ptr.To(testutils.NewTracePipelineBuilder().WithName("cls").Build()),
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:          "alert matches all metric pipelines",
			requestMethod: http.MethodPost,
			requestBody:   bytes.NewBuffer([]byte(`[{"labels":{"alertname":"MetricGatewayReceiverRefusedData"}}]`)),
			resources: []client.Object{
				ptr.To(testutils.NewMetricPipelineBuilder().WithName("cls").Build()),
				ptr.To(testutils.NewMetricPipelineBuilder().WithName("dynatrace").Build()),
			},
			expectedStatus:             http.StatusOK,
			metricPipelinesToReconcile: []string{"cls", "dynatrace"},
		},
		{
			name:          "alert matches all trace pipelines",
			requestMethod: http.MethodPost,
			requestBody:   bytes.NewBuffer([]byte(`[{"labels":{"alertname":"TraceGatewayReceiverRefusedData"}}]`)),
			resources: []client.Object{
				ptr.To(testutils.NewTracePipelineBuilder().WithName("cls").Build()),
				ptr.To(testutils.NewTracePipelineBuilder().WithName("dynatrace").Build()),
			},
			expectedStatus:            http.StatusOK,
			tracePipelinesToReconcile: []string{"cls", "dynatrace"},
		},
		{
			name:          "alert matches all log pipelines",
			requestMethod: http.MethodPost,
			requestBody:   bytes.NewBuffer([]byte(`[{"labels":{"alertname":"FluentBitLogAgentBufferFull"}}]`)),
			resources: []client.Object{
				ptr.To(testutils.NewLogPipelineBuilder().WithName("cls").Build()),
				ptr.To(testutils.NewLogPipelineBuilder().WithName("dynatrace").Build()),
			},
			expectedStatus:          http.StatusOK,
			logPipelinesToReconcile: []string{"cls", "dynatrace"},
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
		{
			name:           "failed to unmarshal request body",
			requestMethod:  http.MethodPost,
			requestBody:    bytes.NewBuffer([]byte(`{"labels":{"alertname":"TraceGatewayReceiverRefusedData"}}`)),
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			metricPipelineEvents := make(chan event.GenericEvent, 1024)
			tracePipelineEvents := make(chan event.GenericEvent, 1024)
			logPipelineEvents := make(chan event.GenericEvent, 1024)

			noopLogger := logr.New(logf.NullLogSink{})

			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = telemetryv1alpha1.AddToScheme(scheme)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.resources...).Build()
			handler := NewHandler(fakeClient,
				WithMetricPipelineSubscriber(metricPipelineEvents),
				WithTracePipelineSubscriber(tracePipelineEvents),
				WithLogPipelineSubscriber(logPipelineEvents),
				WithLogger(noopLogger))

			req, err := http.NewRequestWithContext(t.Context(), tc.requestMethod, "/", tc.requestBody)
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			require.Equal(t, tc.expectedStatus, rr.Code)

			if tc.metricPipelinesToReconcile != nil {
				require.NotEmpty(t, metricPipelineEvents)
				require.ElementsMatch(t, tc.metricPipelinesToReconcile, readAllNamesFromChannel(metricPipelineEvents))
			} else {
				require.Empty(t, metricPipelineEvents)
			}

			if tc.tracePipelinesToReconcile != nil {
				require.NotEmpty(t, tracePipelineEvents)
				require.ElementsMatch(t, tc.tracePipelinesToReconcile, readAllNamesFromChannel(tracePipelineEvents))
			} else {
				require.Empty(t, tracePipelineEvents)
			}

			if tc.logPipelinesToReconcile != nil {
				require.NotEmpty(t, logPipelineEvents)
				require.ElementsMatch(t, tc.logPipelinesToReconcile, readAllNamesFromChannel(logPipelineEvents))
			} else {
				require.Empty(t, logPipelineEvents)
			}

			require.Equal(t, rr.Header().Get("Content-Security-Policy"), "default-src 'self'")
		})
	}
}

func readAllNamesFromChannel(ch <-chan event.GenericEvent) []string {
	var names []string

	for {
		select {
		case event := <-ch:
			names = append(names, event.Object.GetName())
		default:
			return names
		}
	}
}
