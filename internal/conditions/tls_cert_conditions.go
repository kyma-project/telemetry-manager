package conditions

import (
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

func EvaluateTLSCertCondition(errValidation error) (status metav1.ConditionStatus, reason, message string) {
	if errCertExpired, ok := errors.AsType[*tlscert.CertExpiredError](errValidation); ok {
		return metav1.ConditionFalse, ReasonTLSCertificateExpired, errCertExpired.Error()
	}

	if errCertAboutToExpire, ok := errors.AsType[*tlscert.CertAboutToExpireError](errValidation); ok {
		return metav1.ConditionTrue, ReasonTLSCertificateAboutToExpire, errCertAboutToExpire.Error()
	}

	return metav1.ConditionFalse, ReasonTLSConfigurationInvalid, fmt.Sprintf(commonMessages[ReasonTLSConfigurationInvalid], errValidation)
}
