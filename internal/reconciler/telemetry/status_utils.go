package telemetry

import (
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"slices"
	"strings"
)

type blockingResources struct {
	resourceType  string
	resourceNames []string
}

func generateDeletionBlockedMessage(resources ...blockingResources) string {
	var resourcesDesc []string

	for _, res := range resources {
		if len(res.resourceNames) > 0 {
			slices.Sort(res.resourceNames)
			resourcesDesc = append(resourcesDesc, fmt.Sprintf("%s (%s)", res.resourceType, strings.Join(res.resourceNames, ",")))
		}
	}

	return fmt.Sprintf("The deletion of the module is blocked. To unblock the deletion, delete the following resources: %s",
		strings.Join(resourcesDesc, ", "))
}

func determineTLSCertMsg(statusConditions []v1.Condition) string {
	cond := meta.FindStatusCondition(statusConditions, conditions.TypeConfigurationGenerated)
	if cond != nil && (cond.Reason == conditions.ReasonTLSCertificateAboutToExpire ||
		cond.Reason == conditions.ReasonTLSCertificateExpired ||
		cond.Reason == conditions.ReasonTLSCertificateInvalid ||
		cond.Reason == conditions.ReasonTLSPrivateKeyInvalid) {
		return cond.Message
	}
	return ""
}
