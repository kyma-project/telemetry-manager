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
		given           tlscert.TLSCertValidationResult
		expectedStatus  metav1.ConditionStatus
		expectedReason  string
		expectedMessage string
	}{
		{
			name:            "Invalid Certificate",
			given:           tlscert.TLSCertValidationResult{CertValid: false, CertValidationMessage: "Cert is invalid", PrivateKeyValid: true, Validity: time.Now().AddDate(1, 0, 0)},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  ReasonTLSCertificateInvalid,
			expectedMessage: fmt.Sprintf(commonMessages[ReasonTLSCertificateInvalid], "Cert is invalid"),
		},
		{
			name:            "Invalid Certificate and PrivateKey",
			given:           tlscert.TLSCertValidationResult{CertValid: false, CertValidationMessage: "Cert is invalid", PrivateKeyValid: false, PrivateKeyValidationMessage: "PrivateKey is invalid", Validity: time.Now().AddDate(1, 0, 0)},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  ReasonTLSCertificateInvalid,
			expectedMessage: fmt.Sprintf(MessageForLogPipeline(ReasonTLSCertificateInvalid), "Cert is invalid"),
		},
		{
			name:            "Invalid PrivateKey",
			given:           tlscert.TLSCertValidationResult{CertValid: true, PrivateKeyValid: false, PrivateKeyValidationMessage: "PrivateKey is invalid", Validity: time.Now().AddDate(1, 0, 0)},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  ReasonTLSPrivateKeyInvalid,
			expectedMessage: fmt.Sprintf(MessageForLogPipeline(ReasonTLSPrivateKeyInvalid), "PrivateKey is invalid"),
		},
		{
			name:            "Expired Certificate",
			given:           tlscert.TLSCertValidationResult{CertValid: true, PrivateKeyValid: true, Validity: time.Date(2000, 2, 1, 12, 30, 0, 0, time.UTC)},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  ReasonTLSCertificateExpired,
			expectedMessage: fmt.Sprintf(MessageForLogPipeline(ReasonTLSCertificateExpired), "2000-02-01"),
		},
		{
			name:            "Certificate about to expire",
			given:           tlscert.TLSCertValidationResult{CertValid: true, PrivateKeyValid: true, Validity: time.Now().AddDate(0, 0, 7)},
			expectedStatus:  metav1.ConditionTrue,
			expectedReason:  ReasonTLSCertificateAboutToExpire,
			expectedMessage: fmt.Sprintf(MessageForLogPipeline(ReasonTLSCertificateAboutToExpire), time.Now().AddDate(0, 0, 7).Format(time.DateOnly)),
		},
		{
			name:            "TLS Cert/key valid",
			given:           tlscert.TLSCertValidationResult{CertValid: true, PrivateKeyValid: true, Validity: time.Now().AddDate(1, 0, 0)},
			expectedStatus:  metav1.ConditionTrue,
			expectedReason:  ReasonConfigurationGenerated,
			expectedMessage: MessageForLogPipeline(ReasonConfigurationGenerated),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, reason, msg := EvaluateTLSCertCondition(test.given)
			require.Equal(t, test.expectedStatus, status)
			require.Equal(t, test.expectedReason, reason)
			require.Equal(t, test.expectedMessage, msg)

		})
	}
}
