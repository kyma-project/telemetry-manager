package conditions

import (
	"errors"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/telemetry-manager/internal/tlscert"
)

func EvaluateTLSCertCondition(errValidation error, configuredReason string, configuredMessage string) (status metav1.ConditionStatus, reason, message string) {
	if errors.Is(errValidation, tlscert.ErrCertDecodeFailed) || errors.Is(errValidation, tlscert.ErrCertParseFailed) || // validate certificate
		errors.Is(errValidation, tlscert.ErrKeyDecodeFailed) || errors.Is(errValidation, tlscert.ErrKeyParseFailed) || // validate key
		errors.Is(errValidation, tlscert.ErrInvalidCertificateKeyPair) || // validate certificate-key pair
		errors.Is(errValidation, tlscert.ErrCertIsNotCA) { // validate CA
		return metav1.ConditionFalse, ReasonTLSCertificateInvalid, fmt.Sprintf(commonMessages[ReasonTLSCertificateInvalid], errValidation)
	}

	var errCertExpired *tlscert.CertExpiredError
	if errors.As(errValidation, &errCertExpired) {
		return metav1.ConditionFalse, ReasonTLSCertificateExpired, fmt.Sprintf(commonMessages[ReasonTLSCertificateExpired], errCertExpired.Expiry.Format(time.DateOnly))
	}

	var errCertAboutToExpire *tlscert.CertAboutToExpireError
	if errors.As(errValidation, &errCertAboutToExpire) {
		return metav1.ConditionTrue, ReasonTLSCertificateAboutToExpire, fmt.Sprintf(commonMessages[ReasonTLSCertificateAboutToExpire], errCertAboutToExpire.Expiry.Format(time.DateOnly))
	}

	return metav1.ConditionTrue, configuredReason, configuredMessage
}
