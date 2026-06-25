package predicate

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrlpredicate "sigs.k8s.io/controller-runtime/pkg/predicate"
)

func UpdateOrDelete() ctrlpredicate.Predicate {
	return ctrlpredicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		DeleteFunc:  func(e event.DeleteEvent) bool { return true },
		UpdateFunc:  func(e event.UpdateEvent) bool { return true },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
}

// OwnedResourceChanged is a predicate for watching owned resources. It detects external changes
// to spec or metadata (e.g. by users) so the controller can reconcile them back to the desired state.
// It ignores create events (owned resources are created by the controller itself) and reacts only
// to updates with a changed resource version and to deletions.
func OwnedResourceChanged() ctrlpredicate.Predicate {
	return ctrlpredicate.And(
		ctrlpredicate.ResourceVersionChangedPredicate{},
		ctrlpredicate.Funcs{
			CreateFunc:  func(e event.CreateEvent) bool { return false },
			DeleteFunc:  func(e event.DeleteEvent) bool { return true },
			UpdateFunc:  func(e event.UpdateEvent) bool { return true },
			GenericFunc: func(e event.GenericEvent) bool { return false },
		})
}
