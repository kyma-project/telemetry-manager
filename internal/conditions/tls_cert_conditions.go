package conditions

import (
	"fmt"
	"time"

	"errors"
	"github.com/kyma-project/telemetry-manager/internal/tlscert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func EvaluateTLSCertCondition(errValidation error) (status metav1.ConditionStatus, reason, message string) {
	if errors.Is(errValidation, tlscert.ErrCertDecodeFailed) || errors.Is(errValidation, tlscert.ErrCertParseFailed) {
		return metav1.ConditionFalse, ReasonTLSCertificateInvalid, fmt.Sprintf(commonMessages[ReasonTLSCertificateInvalid], errValidation)
	}

	if errors.Is(errValidation, tlscert.ErrKeyDecodeFailed) || errors.Is(errValidation, tlscert.ErrKeyParseFailed) {
		return metav1.ConditionFalse, ReasonTLSPrivateKeyInvalid, fmt.Sprintf(commonMessages[ReasonTLSPrivateKeyInvalid], errValidation)
	}

	var errCertExpired *tlscert.CertExpiredError
	if errors.As(errValidation, &errCertExpired) {
		return metav1.ConditionFalse, ReasonTLSCertificateExpired, fmt.Sprintf(commonMessages[ReasonTLSCertificateExpired], errCertExpired.Expiry.Format(time.DateOnly))
	}

	var errCertAboutToExpire *tlscert.CertAboutToExpireError
	if errors.As(errValidation, &errCertAboutToExpire) {
		return metav1.ConditionTrue, ReasonTLSCertificateAboutToExpire, fmt.Sprintf(commonMessages[ReasonTLSCertificateAboutToExpire], errCertAboutToExpire.Expiry.Format(time.DateOnly))
	}

	return metav1.ConditionTrue, ReasonConfigurationGenerated, ""
}
