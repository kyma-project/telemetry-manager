package v1alpha1

import (
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMetricPipelineGetCondition(t *testing.T) {
	exampleStatus := MetricPipelineStatus{Conditions: []MetricPipelineCondition{{Type: MetricPipelinePending}}}

	tests := []struct {
		name     string
		status   MetricPipelineStatus
		condType MetricPipelineConditionType
		expected bool
	}{
		{
			name:     "condition exists",
			status:   exampleStatus,
			condType: MetricPipelinePending,
			expected: true,
		},
		{
			name:     "condition does not exist",
			status:   exampleStatus,
			condType: MetricPipelineRunning,
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

func TestMetricPipelineSetCondition(t *testing.T) {
	condPending := MetricPipelineCondition{Type: MetricPipelinePending, Reason: "ForSomeReason"}
	condRunning := MetricPipelineCondition{Type: MetricPipelineRunning, Reason: "ForSomeOtherReason"}
	condRunningOtherReason := MetricPipelineCondition{Type: MetricPipelineRunning, Reason: "BecauseItIs"}

	ts := metav1.Now()
	tsLater := metav1.NewTime(ts.Add(1 * time.Minute))

	tests := []struct {
		name           string
		status         MetricPipelineStatus
		cond           MetricPipelineCondition
		expectedStatus MetricPipelineStatus
	}{
		{
			name:           "set for the first time",
			status:         MetricPipelineStatus{},
			cond:           condPending,
			expectedStatus: MetricPipelineStatus{Conditions: []MetricPipelineCondition{condPending}},
		},
		{
			name:           "simple set",
			status:         MetricPipelineStatus{Conditions: []MetricPipelineCondition{condPending}},
			cond:           condRunning,
			expectedStatus: MetricPipelineStatus{Conditions: []MetricPipelineCondition{condPending, condRunning}},
		},
		{
			name:           "overwrite",
			status:         MetricPipelineStatus{Conditions: []MetricPipelineCondition{condRunning}},
			cond:           condRunningOtherReason,
			expectedStatus: MetricPipelineStatus{Conditions: []MetricPipelineCondition{condRunningOtherReason}},
		},
		{
			name:           "overwrite",
			status:         MetricPipelineStatus{Conditions: []MetricPipelineCondition{condRunning}},
			cond:           condRunningOtherReason,
			expectedStatus: MetricPipelineStatus{Conditions: []MetricPipelineCondition{condRunningOtherReason}},
		},
		{
			name:           "not overwrite last transition time",
			status:         MetricPipelineStatus{Conditions: []MetricPipelineCondition{{Type: MetricPipelinePending, LastTransitionTime: ts}}},
			cond:           MetricPipelineCondition{Type: MetricPipelinePending, LastTransitionTime: tsLater},
			expectedStatus: MetricPipelineStatus{Conditions: []MetricPipelineCondition{{Type: MetricPipelinePending, LastTransitionTime: ts}}},
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
