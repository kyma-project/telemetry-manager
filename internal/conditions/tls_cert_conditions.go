package conditions

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/telemetry-manager/internal/tlscert"
)

const twoWeeks = time.Hour * 24 * 7 * 2

func EvaluateTLSCertCondition(certValidationResult tlscert.TLSCertValidationResult) (status metav1.ConditionStatus, reason, message string) {
	if !certValidationResult.CertValid {
		return metav1.ConditionFalse, ReasonTLSCertificateInvalid, fmt.Sprintf(commonMessages[ReasonTLSCertificateInvalid], certValidationResult.CertValidationMessage)
	}

	if !certValidationResult.PrivateKeyValid {
		return metav1.ConditionFalse, ReasonTLSPrivateKeyInvalid, fmt.Sprintf(commonMessages[ReasonTLSPrivateKeyInvalid], certValidationResult.PrivateKeyValidationMessage)

	}

	if time.Now().After(certValidationResult.Validity) {
		return metav1.ConditionFalse, ReasonTLSCertificateExpired, fmt.Sprintf(commonMessages[ReasonTLSCertificateExpired], certValidationResult.Validity.Format(time.DateOnly))
	}

	//ensure not expired and about to expire
	validUntil := time.Until(certValidationResult.Validity)
	if validUntil > 0 && validUntil <= twoWeeks {
		return metav1.ConditionTrue, ReasonTLSCertificateAboutToExpire, fmt.Sprintf(commonMessages[ReasonTLSCertificateAboutToExpire], certValidationResult.Validity.Format(time.DateOnly))
	}

	return metav1.ConditionTrue, ReasonConfigurationGenerated, ""
}
