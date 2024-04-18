package conditions

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/telemetry-manager/internal/tlscert"
)

const twoWeeks = time.Hour * 24 * 7 * 2

func EvaluateTLSCertCondition(certValidationResult tlscert.TLSCertValidationResult) (status metav1.ConditionStatus, reason, message string) {
	status = metav1.ConditionTrue
	reason = ReasonConfigurationGenerated
	message = ""
	validationMsg := ""

	if !certValidationResult.CertValid {
		status = metav1.ConditionFalse
		reason = ReasonTLSCertificateInvalid
		validationMsg = certValidationResult.CertValidationMessage
	}

	if !certValidationResult.PrivateKeyValid {
		status = metav1.ConditionFalse
		reason = ReasonTLSPrivateKeyInvalid
		validationMsg = certValidationResult.PrivateKeyValidationMessage
	}

	if time.Now().After(certValidationResult.Validity) {
		status = metav1.ConditionFalse
		reason = ReasonTLSCertificateExpired
		validationMsg = certValidationResult.Validity.Format(time.DateOnly)
	}

	//ensure not expired and about to expire
	validUntil := time.Until(certValidationResult.Validity)
	if validUntil > 0 && validUntil <= twoWeeks {
		status = metav1.ConditionTrue
		reason = ReasonTLSCertificateAboutToExpire
		validationMsg = certValidationResult.Validity.Format(time.DateOnly)
	}

	if validationMsg != "" {
		message = fmt.Sprintf(CommonMessages(reason), validationMsg)
	}

	return status, reason, message
}
