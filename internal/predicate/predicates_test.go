package predicate

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestUpdateOrDelete(t *testing.T) {
	sut := UpdateOrDelete()

	t.Run("should return true when update event", func(t *testing.T) {
		require.True(t, sut.Update(event.UpdateEvent{}))
	})

	t.Run("should return true when delete event", func(t *testing.T) {
		require.True(t, sut.Delete(event.DeleteEvent{}))
	})

	t.Run("should return false when create or generic event", func(t *testing.T) {
		require.False(t, sut.Create(event.CreateEvent{}), "Create event")
		require.False(t, sut.Generic(event.GenericEvent{}), "Generic event")
	})
}

func TestCreateOrUpdateOrDelete(t *testing.T) {
	sut := CreateOrUpdateOrDelete()

	t.Run("should return true when create event", func(t *testing.T) {
		require.True(t, sut.Create(event.CreateEvent{}))
	})

	t.Run("should return true when update event", func(t *testing.T) {
		require.True(t, sut.Update(event.UpdateEvent{}))
	})

	t.Run("should return true when delete event", func(t *testing.T) {
		require.True(t, sut.Delete(event.DeleteEvent{}))
	})

	t.Run("should return false when generic event", func(t *testing.T) {
		require.False(t, sut.Generic(event.GenericEvent{}))
	})
}

func TestCreateOrDelete(t *testing.T) {
	sut := CreateOrDelete()

	t.Run("should return true when create event", func(t *testing.T) {
		require.True(t, sut.Create(event.CreateEvent{}))
	})

	t.Run("should return true when delete event", func(t *testing.T) {
		require.True(t, sut.Delete(event.DeleteEvent{}))
	})

	t.Run("should return false when update or generic event", func(t *testing.T) {
		require.False(t, sut.Update(event.UpdateEvent{}), "Update event")
		require.False(t, sut.Generic(event.GenericEvent{}), "Generic event")
	})
}

func TestOwnedResourceChanged(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-pod",
			ResourceVersion: "1",
		},
	}

	sut := OwnedResourceChanged()

	t.Run("should return true when resource version changed", func(t *testing.T) {
		podCopy := pod.DeepCopy()
		podCopy.ResourceVersion = "2"
		require.True(t, sut.Update(event.UpdateEvent{ObjectOld: pod, ObjectNew: podCopy}), "Update event with resource version changed")
	})

	t.Run("should return false when resource version not changed", func(t *testing.T) {
		podCopy := pod.DeepCopy()
		require.False(t, sut.Update(event.UpdateEvent{ObjectOld: pod, ObjectNew: podCopy}), "Update event with resource version not changed")
	})

	t.Run("should return true when resource deleted", func(t *testing.T) {
		require.True(t, sut.Delete(event.DeleteEvent{}), "Delete event")
	})

	t.Run("should return false when create or generic event", func(t *testing.T) {
		require.False(t, sut.Create(event.CreateEvent{}), "Create event")
		require.False(t, sut.Generic(event.GenericEvent{}), "Generic event")
	})
}
