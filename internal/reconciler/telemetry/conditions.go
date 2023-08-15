package telemetry

import (
	"context"
	"fmt"
	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type conditionsProber interface {
	endpoints(ctx context.Context, config Config, endpoints operatorv1alpha1.Endpoints) (operatorv1alpha1.Endpoints, error)
	isComponentHealthy(ctx context.Context) (*metav1.Condition, error)
	name() string
}

func (r *Reconciler) updateConditions(ctx context.Context, cp conditionsProber, obj *operatorv1alpha1.Telemetry) error {
	logf := log.FromContext(ctx)
	logf.Info(fmt.Sprintf("Updating condition for: %s", cp.name()))
	conditions := &obj.Status.Conditions
	newCondition, err := cp.isComponentHealthy(ctx)
	if err != nil {
		return fmt.Errorf("unable to update conditions for: %v, %w", cp.name(), err)
	}
	logf.Info(fmt.Sprintf("Got condition: %+v\n", newCondition))

	operatorStatus := operatorv1alpha1.Status{State: "Ready"}
	for _, c := range *conditions {
		if c.Status == "False" {
			operatorStatus.State = "Error"
		}
	}
	newCondition.ObservedGeneration = obj.GetGeneration()

	meta.SetStatusCondition(&obj.Status.Conditions, *newCondition)
	obj.Status.Status = operatorStatus
	r.serverSideApplyStatus(ctx, obj)
	return nil
}

func (r *Reconciler) updateEndpoints(ctx context.Context, cp conditionsProber, obj *operatorv1alpha1.Telemetry) error {
	endpoints := obj.Status.Endpoints
	updatedEndpoints, err := cp.endpoints(ctx, r.TelemetryConfig, endpoints)
	if err != nil {
		return fmt.Errorf("unable to update endpoints for: %s:%w", cp.name(), err)
	}
	obj.Status.Endpoints = updatedEndpoints
	r.serverSideApplyStatus(ctx, obj)
	return nil
}
