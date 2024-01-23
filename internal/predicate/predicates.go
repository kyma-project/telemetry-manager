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

func CreateOrUpdateOrDelete() ctrlpredicate.Predicate {
	return ctrlpredicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return true },
		DeleteFunc:  func(e event.DeleteEvent) bool { return true },
		UpdateFunc:  func(e event.UpdateEvent) bool { return true },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
}

func CreateOrDelete() ctrlpredicate.Predicate {
	return ctrlpredicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return true },
		DeleteFunc:  func(e event.DeleteEvent) bool { return true },
		UpdateFunc:  func(e event.UpdateEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
}

// OwnedResourceChanged returns a predicate function that returns true
// when there is a change in the resource version of the owned resource or when
// the resource is deleted.
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
