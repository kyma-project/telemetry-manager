package conditions

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/telemetry-manager/internal/tlscert"
)

func Test_EvaluateTLSCertCondition(t *testing.T) {
	tests := []struct {
		name            string
		given           error
		expectedStatus  metav1.ConditionStatus
		expectedReason  string
		expectedMessage string
	}{
		{
			name:            "cert decode failed",
			given:           tlscert.ErrCertDecodeFailed,
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  ReasonTLSCertificateInvalid,
			expectedMessage: fmt.Sprintf(commonMessages[ReasonTLSCertificateInvalid], tlscert.ErrCertDecodeFailed),
		},
		{
			name:            "cert parse failed",
			given:           tlscert.ErrCertParseFailed,
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  ReasonTLSCertificateInvalid,
			expectedMessage: fmt.Sprintf(commonMessages[ReasonTLSCertificateInvalid], tlscert.ErrCertParseFailed),
		},
		{
			name:            "private key decode failed",
			given:           tlscert.ErrKeyDecodeFailed,
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  ReasonTLSCertificateInvalid,
			expectedMessage: fmt.Sprintf(MessageForLogPipeline(ReasonTLSCertificateInvalid), tlscert.ErrKeyDecodeFailed),
		},
		{
			name:            "private key parse failed",
			given:           tlscert.ErrKeyParseFailed,
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  ReasonTLSCertificateInvalid,
			expectedMessage: fmt.Sprintf(MessageForLogPipeline(ReasonTLSCertificateInvalid), tlscert.ErrKeyParseFailed),
		},
		{
			name:            "cert expired",
			given:           &tlscert.CertExpiredError{Expiry: time.Date(2000, 2, 1, 0, 0, 0, 0, time.UTC)},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  ReasonTLSCertificateExpired,
			expectedMessage: fmt.Sprintf(MessageForLogPipeline(ReasonTLSCertificateExpired), "2000-02-01"),
		},
		{
			name:            "cert about to expire",
			given:           &tlscert.CertAboutToExpireError{Expiry: time.Now().AddDate(0, 0, 7)},
			expectedStatus:  metav1.ConditionTrue,
			expectedReason:  ReasonTLSCertificateAboutToExpire,
			expectedMessage: fmt.Sprintf(MessageForLogPipeline(ReasonTLSCertificateAboutToExpire), time.Now().AddDate(0, 0, 7).Format(time.DateOnly)),
		},
		{
			name:            "cert and private key valid",
			given:           nil,
			expectedStatus:  metav1.ConditionTrue,
			expectedReason:  ReasonAgentConfigured,
			expectedMessage: MessageForLogPipeline(ReasonAgentConfigured),
		},
		{
			name:            "invalid cert key pair",
			given:           tlscert.ErrInvalidCertificateKeyPair,
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  ReasonTLSCertificateInvalid,
			expectedMessage: fmt.Sprintf(MessageForLogPipeline(ReasonTLSCertificateInvalid), tlscert.ErrInvalidCertificateKeyPair),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, reason, msg := EvaluateTLSCertCondition(test.given, ReasonAgentConfigured, MessageForLogPipeline(ReasonAgentConfigured))
			require.Equal(t, test.expectedStatus, status)
			require.Equal(t, test.expectedReason, reason)
			require.Equal(t, test.expectedMessage, msg)
		})
	}
}
