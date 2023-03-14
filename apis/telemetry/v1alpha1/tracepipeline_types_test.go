package v1alpha1

import (
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTracePipelineGetCondition(t *testing.T) {
	exampleStatus := TracePipelineStatus{Conditions: []TracePipelineCondition{{Type: TracePipelinePending}}}

	tests := []struct {
		name     string
		status   TracePipelineStatus
		condType TracePipelineConditionType
		expected bool
	}{
		{
			name:     "condition exists",
			status:   exampleStatus,
			condType: TracePipelinePending,
			expected: true,
		},
		{
			name:     "condition does not exist",
			status:   exampleStatus,
			condType: TracePipelineRunning,
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cond := test.status.GetCondition(test.condType)
			exists := cond != nil
			if exists != test.expected {
				t.Errorf("%s: expected condition to exist: %t, got: %t", test.name, test.expected, exists)
			}
		})
	}
}

func TestTracePipelineSetCondition(t *testing.T) {
	condPending := TracePipelineCondition{Type: TracePipelinePending, Reason: "ForSomeReason"}
	condRunning := TracePipelineCondition{Type: TracePipelineRunning, Reason: "ForSomeOtherReason"}
	condRunningOtherReason := TracePipelineCondition{Type: TracePipelineRunning, Reason: "BecauseItIs"}

	ts := metav1.Now()
	tsLater := metav1.NewTime(ts.Add(1 * time.Minute))

	tests := []struct {
		name           string
		status         TracePipelineStatus
		cond           TracePipelineCondition
		expectedStatus TracePipelineStatus
	}{
		{
			name:           "set for the first time",
			status:         TracePipelineStatus{},
			cond:           condPending,
			expectedStatus: TracePipelineStatus{Conditions: []TracePipelineCondition{condPending}},
		},
		{
			name:           "simple set",
			status:         TracePipelineStatus{Conditions: []TracePipelineCondition{condPending}},
			cond:           condRunning,
			expectedStatus: TracePipelineStatus{Conditions: []TracePipelineCondition{condPending, condRunning}},
		},
		{
			name:           "overwrite",
			status:         TracePipelineStatus{Conditions: []TracePipelineCondition{condRunning}},
			cond:           condRunningOtherReason,
			expectedStatus: TracePipelineStatus{Conditions: []TracePipelineCondition{condRunningOtherReason}},
		},
		{
			name:           "overwrite",
			status:         TracePipelineStatus{Conditions: []TracePipelineCondition{condRunning}},
			cond:           condRunningOtherReason,
			expectedStatus: TracePipelineStatus{Conditions: []TracePipelineCondition{condRunningOtherReason}},
		},
		{
			name:           "not overwrite last transition time",
			status:         TracePipelineStatus{Conditions: []TracePipelineCondition{{Type: TracePipelinePending, LastTransitionTime: ts}}},
			cond:           TracePipelineCondition{Type: TracePipelinePending, LastTransitionTime: tsLater},
			expectedStatus: TracePipelineStatus{Conditions: []TracePipelineCondition{{Type: TracePipelinePending, LastTransitionTime: ts}}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.status.SetCondition(test.cond)
			if !reflect.DeepEqual(test.status, test.expectedStatus) {
				t.Errorf("%s: expected status: %v, got: %v", test.name, test.expectedStatus, test.status)
			}
		})
	}
}
