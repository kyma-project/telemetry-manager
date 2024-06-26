package conditions

import (
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/telemetry-manager/internal/tlscert"
)

func EvaluateTLSCertCondition(errValidation error, configuredReason string, configuredMessage string) (status metav1.ConditionStatus, reason, message string) {
	// No Error
	if errValidation == nil {
		return metav1.ConditionTrue, configuredReason, configuredMessage
	}

	// Cert/CA Expired Error
	var errCertExpired *tlscert.CertExpiredError
	if errors.As(errValidation, &errCertExpired) {
		return metav1.ConditionFalse, ReasonTLSCertificateExpired, errCertExpired.Error()
	}

	// Cert/CA About to Expire Error
	var errCertAboutToExpire *tlscert.CertAboutToExpireError
	if errors.As(errValidation, &errCertAboutToExpire) {
		return metav1.ConditionTrue, ReasonTLSCertificateAboutToExpire, errCertAboutToExpire.Error()
	}

	// Invalid TLS Error
	return metav1.ConditionFalse, ReasonTLSConfigurationInvalid, fmt.Sprintf(commonMessages[ReasonTLSConfigurationInvalid], errValidation)
}
