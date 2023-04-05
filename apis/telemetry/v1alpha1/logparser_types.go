/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LogParserSpec defines the desired state of LogParser.
type LogParserSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// [Fluent Bit Parsers](https://docs.fluentbit.io/manual/pipeline/parsers). The parser specified here has no effect until it is referenced by a [Pod annotation](https://docs.fluentbit.io/manual/pipeline/filters/kubernetes#kubernetes-annotations) on your workload or by a [Parser Filter](https://docs.fluentbit.io/manual/pipeline/filters/parser) defined in a pipeline's filters section.
	Parser string `json:"parser,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.conditions[-1].type`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LogParser is the Schema for the logparsers API.
type LogParser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Defines the desired state of LogParser.
	Spec LogParserSpec `json:"spec,omitempty"`
	// Shows the observed state of the LogParser.
	Status LogParserStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LogParserList contains a list of LogParser.
type LogParserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LogParser `json:"items"`
}

type LogParserConditionType string

// These are the valid statuses of LogParser.
const (
	LogParserPending LogParserConditionType = "Pending"
	LogParserRunning LogParserConditionType = "Running"
)

type LogParserCondition struct {
	// An array of conditions describing the status of the parser.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// An array of conditions describing the status of the parser.
	Reason string `json:"reason,omitempty"`
	// The possible transition types are:<br>- `Running`: The parser is ready and usable.<br>- `Pending`: The parser is being activated.
	Type LogParserConditionType `json:"type,omitempty"`
}

// LogParserStatus shows the observed state of the LogParser.
type LogParserStatus struct {
	// An array of conditions describing the status of the parser.
	Conditions []LogParserCondition `json:"conditions,omitempty"`
}

func (lps *LogParserStatus) GetCondition(condType LogParserConditionType) *LogParserCondition {
	for cond := range lps.Conditions {
		if lps.Conditions[cond].Type == condType {
			return &lps.Conditions[cond]
		}
	}
	return nil
}

func (lps *LogParserStatus) HasCondition(condition LogParserConditionType) bool {
	return lps.GetCondition(condition) != nil
}

func NewLogParserCondition(reason string, condType LogParserConditionType) *LogParserCondition {
	return &LogParserCondition{
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Type:               condType,
	}
}

func (lps *LogParserStatus) SetCondition(cond LogParserCondition) {
	currentCond := lps.GetCondition(cond.Type)
	if currentCond != nil && currentCond.Reason == cond.Reason {
		return
	}
	if currentCond != nil {
		cond.LastTransitionTime = currentCond.LastTransitionTime
	}
	newConditions := lps.filterOutCondition(lps.Conditions, cond.Type)
	lps.Conditions = append(newConditions, cond)
}

func (lps *LogParserStatus) filterOutCondition(conditions []LogParserCondition, condType LogParserConditionType) []LogParserCondition {
	var newConditions []LogParserCondition
	for _, cond := range conditions {
		if cond.Type == condType {
			continue
		}
		newConditions = append(newConditions, cond)
	}
	return newConditions
}

//nolint:gochecknoinits // SchemeBuilder's registration is required.
func init() {
	SchemeBuilder.Register(&LogParser{}, &LogParserList{})
}
