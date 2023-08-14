package telemetry

import (
	"context"
	"fmt"
	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// TODO: move to status

type conditionsProber interface {
	isComponentHealthy(ctx context.Context) (*metav1.Condition, error)
	name() string
}

func (r *Reconciler) updateConditions(ctx context.Context, cp conditionsProber, obj *operatorv1alpha1.Telemetry) error {
	logf := log.FromContext(ctx)
	condition, err := cp.isComponentHealthy(ctx)
	logf.Info(fmt.Sprintf("Got condition: %+v\n", condition))

	if err != nil {
		return fmt.Errorf("unable to update conditions for: %v, %w", cp.name(), err)
	}
	operatorStatus := operatorv1alpha1.Status{State: "Ready"}
	if condition.Status == "False" {
		operatorStatus.State = "Error"
	}
	meta.SetStatusCondition(&obj.Status.Conditions, *condition)
	obj.Status.Status = operatorStatus
	r.serverSideApplyStatus(ctx, obj)
	return nil
}
