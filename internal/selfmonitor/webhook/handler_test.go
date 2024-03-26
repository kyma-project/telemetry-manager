package webhook

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type errReader struct{}

func (errReader) Read(p []byte) (n int, err error) {
	return 0, assert.AnError
}

func TestHandler(t *testing.T) {
	tests := []struct {
		name           string
		requestMethod  string
		requestBody    io.Reader
		expectedStatus int
		expectEvent    bool
	}{
		{
			name:           "valid",
			requestMethod:  http.MethodPost,
			expectedStatus: http.StatusOK,
			expectEvent:    true,
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
			eventChan := make(chan event.GenericEvent, 1)

			handler := NewHandler(eventChan)
			if tc.requestBody == nil {
				tc.requestBody = bytes.NewBuffer([]byte(`{"key":"value"}`))
			}

			req, err := http.NewRequest(tc.requestMethod, "/", tc.requestBody)
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			require.Equal(t, tc.expectedStatus, rr.Code)
			if tc.expectEvent {
				require.NotEmpty(t, eventChan)
			}
		})
	}
}
