package telemetry

import (
	"context"
	"fmt"
	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type conditionStatus string

//
//type reason string
//type message string

//const (
//	LogCollectorHealthy reason = "LogCollectorHealthy"
//)

// TODO: move to status
var conditions = map[string]string{
	reconciler.ReasonNoPipelineDeployed:  "No pipelines have been deployed",
	"logPipelinePending":                 "Atleast one logpipeline is Pending",
	reconciler.ReasonFluentBitDSNotReady: "Fluent bit Daemonset is not ready",
	reconciler.ReasonFluentBitDSReady:    "Fluent bit daemonset is ready",
}

//type condProber struct {
//	client client.Client
//}

//type telemetryConditions struct {
//	componentType string
//	status        conditionStatus
//	reason        string
//	message       string
//}

type conditionsProber interface {
	isComponentHealthy(ctx context.Context) (*metav1.Condition, error)
	name() string
}

func (r *Reconciler) updateConditions(ctx context.Context, cp conditionsProber, obj *operatorv1alpha1.Telemetry) error {
	logf := log.FromContext(ctx)
	condition, err := cp.isComponentHealthy(ctx)
	logf.Info(fmt.Sprintf("Got condition: %+v\n", condition))

	if err != nil {
		return fmt.Errorf("unable to update conditions for: %v, %w", cp.name)
	}
	meta.SetStatusCondition(&obj.Status.Conditions, *condition)
	r.serverSideApplyStatus(ctx, obj)
	return nil
}
