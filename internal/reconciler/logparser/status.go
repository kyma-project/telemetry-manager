package logparser

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *Reconciler) updateStatus(ctx context.Context, parserName string) error {
	log := logf.FromContext(ctx)
	var parser telemetryv1alpha1.LogParser
	if err := r.Get(ctx, types.NamespacedName{Name: parserName}, &parser); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("failed to get LogParser: %v", err)
	}

	if parser.DeletionTimestamp != nil {
		return nil
	}

	// If one of the conditions has an empty "Status", it means that the old LogParserCondition was used when this parser was created
	// In this case, the required "Status" and "Message" fields need to be populated with proper values
	if len(parser.Status.Conditions) > 0 && parser.Status.Conditions[0].Status == "" {
		populateMissingConditionFields(ctx, r.Client, &parser)
	}

	fluentBitReady, err := r.prober.IsReady(ctx, r.config.DaemonSet)
	if err != nil {
		return err
	}

	if fluentBitReady {
		if parser.Status.HasCondition(conditions.TypeRunning) {
			return nil
		}

		running := newCondition(
			conditions.TypeRunning,
			conditions.ReasonFluentBitDSReady,
			metav1.ConditionTrue,
			parser.Generation,
		)
		return setCondition(ctx, r.Client, &parser, running)
	}

	pending := newCondition(
		conditions.TypePending,
		conditions.ReasonFluentBitDSNotReady,
		metav1.ConditionTrue,
		parser.Generation,
	)

	if parser.Status.HasCondition(conditions.TypeRunning) {
		log.V(1).Info(fmt.Sprintf("Updating the status of %s to %s. Resetting previous conditions", parser.Name, pending.Type))
		parser.Status.Conditions = []metav1.Condition{}
	}

	return setCondition(ctx, r.Client, &parser, pending)
}

func populateMissingConditionFields(ctx context.Context, client client.Client, parser *telemetryv1alpha1.LogParser) error {
	log := logf.FromContext(ctx)
	log.V(1).Info(fmt.Sprintf("Populating missing fields in the Status conditions for %s", parser.Name))

	for i := range parser.Status.Conditions {
		parser.Status.Conditions[i].Status = metav1.ConditionTrue
		parser.Status.Conditions[i].Message = conditions.CommonMessageFor(parser.Status.Conditions[i].Reason)
	}

	if err := client.Status().Update(ctx, parser); err != nil {
		return fmt.Errorf("failed to update LogParser status when poplulating missing fields in conditions: %v", err)
	}
	return nil
}

func newCondition(condType, reason string, status metav1.ConditionStatus, generation int64) *metav1.Condition {
	return &metav1.Condition{
		LastTransitionTime: metav1.Now(),
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            conditions.CommonMessageFor(reason),
		ObservedGeneration: generation,
	}
}

func setCondition(ctx context.Context, client client.Client, parser *telemetryv1alpha1.LogParser, condition *metav1.Condition) error {
	log := logf.FromContext(ctx)

	log.V(1).Info(fmt.Sprintf("Updating the status of %s to %s", parser.Name, condition.Type))

	parser.Status.SetCondition(*condition)

	if err := client.Status().Update(ctx, parser); err != nil {
		return fmt.Errorf("failed to update LogParser status to %s: %v", condition.Type, err)
	}
	return nil
}
