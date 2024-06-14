package conditions

import (
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/telemetry-manager/internal/tlscert"
)

func EvaluateTLSCertCondition(errValidation error, configuredReason string, configuredMessage string) (status metav1.ConditionStatus, reason, message string) {
	if isInvalidTLSError(errValidation) {
		return metav1.ConditionFalse, ReasonTLSConfigurationInvalid, fmt.Sprintf(commonMessages[ReasonTLSConfigurationInvalid], errValidation)
	}

	// Cert expired
	var errCertExpired *tlscert.CertExpiredError
	if errors.As(errValidation, &errCertExpired) {
		return metav1.ConditionFalse, ReasonTLSCertificateExpired, errCertExpired.Error()
	}

	// Cert about to expire
	var errCertAboutToExpire *tlscert.CertAboutToExpireError
	if errors.As(errValidation, &errCertAboutToExpire) {
		return metav1.ConditionTrue, ReasonTLSCertificateAboutToExpire, errCertAboutToExpire.Error()
	}

	return metav1.ConditionTrue, configuredReason, configuredMessage
}

func isInvalidTLSError(err error) bool {
	invalidErrors := []error{
		tlscert.ErrCertDecodeFailed,
		tlscert.ErrCertParseFailed,
		tlscert.ErrKeyDecodeFailed,
		tlscert.ErrKeyParseFailed,
		tlscert.ErrCADecodeFailed,
		tlscert.ErrCAParseFailed,
		tlscert.ErrInvalidCertificateKeyPair,
		tlscert.ErrCertIsNotCA,
	}

	for _, e := range invalidErrors {
		if errors.Is(err, e) {
			return true
		}
	}

	return false
}
