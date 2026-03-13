package vpastatus

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

func TestVpaCRDExists(t *testing.T) {
	tests := []struct {
		name           string
		handler        http.HandlerFunc
		expectedResult bool
		expectError    bool
	}{
		{
			name: "VPA CRD exists",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := metav1.APIResourceList{
					GroupVersion: "autoscaling.k8s.io/v1",
					APIResources: []metav1.APIResource{
						{Kind: "VerticalPodAutoscaler"},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			},
			expectedResult: true,
		},
		{
			name: "VPA group version exists but VPA kind not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := metav1.APIResourceList{
					GroupVersion: "autoscaling.k8s.io/v1",
					APIResources: []metav1.APIResource{
						{Kind: "SomeOtherKind"},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			},
			expectedResult: false,
		},
		{
			name: "VPA group version not found returns false",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectedResult: false,
		},
		{
			name: "server returns internal error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectedResult: false,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			checker := NewChecker(&rest.Config{Host: server.URL})
			result, err := checker.VpaCRDExists(t.Context(), nil)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
