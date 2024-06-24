package telemetry

import (
	"fmt"
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
