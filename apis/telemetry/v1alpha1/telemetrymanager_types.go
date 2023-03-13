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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type State string

var (
	ConditionTypeInstallation = "Installation"
	ConditionReasonReady      = "Ready"
)

// Valid Module CR States.
const (
	// StateReady signifies Module CR is Ready and has been installed successfully.
	StateReady State = "Ready"

	// StateProcessing signifies Module CR is reconciling and is in the process of installation.
	// Processing can also signal that the Installation previously encountered an error and is now recovering.
	StateProcessing State = "Processing"

	// StateError signifies an error for Module CR. This signifies that the Installation
	// process encountered an error.
	// Contrary to Processing, it can be expected that this state should change on the next retry.
	StateError State = "Error"

	// StateDeleting signifies Module CR is being deleted. This is the state that is used
	// when a deletionTimestamp was detected and Finalizers are picked up.
	StateDeleting State = "Deleting"
)

// TelemetryManagerSpec defines the desired state of TelemetryManager
type TelemetryManagerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// TelemetryManagerStatus defines the observed state of TelemetryManager
type TelemetryManagerStatus struct {
	Status `json:",inline"`

	// Conditions contain a set of conditionals to determine the State of Status.
	// If all Conditions are met, State is expected to be in StateReady.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// add other fields to status subresource here
}

func (s *TelemetryManagerStatus) WithState(state State) *TelemetryManagerStatus {
	s.State = state
	return s
}

func (s *TelemetryManagerStatus) WithInstallConditionStatus(status metav1.ConditionStatus, objGeneration int64) *TelemetryManagerStatus {
	if s.Conditions == nil {
		s.Conditions = make([]metav1.Condition, 0, 1)
	}

	condition := meta.FindStatusCondition(s.Conditions, ConditionTypeInstallation)

	if condition == nil {
		condition = &metav1.Condition{
			Type:    ConditionTypeInstallation,
			Reason:  ConditionReasonReady,
			Message: "installation is ready and resources can be used",
		}
	}

	condition.Status = status
	condition.ObservedGeneration = objGeneration
	meta.SetStatusCondition(&s.Conditions, *condition)
	return s
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// TelemetryManager is the Schema for the telemetrymanagers API
type TelemetryManager struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TelemetryManagerSpec   `json:"spec,omitempty"`
	Status TelemetryManagerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TelemetryManagerList contains a list of TelemetryManager
type TelemetryManagerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TelemetryManager `json:"items"`
}

//nolint:gochecknoinits
func init() {
	SchemeBuilder.Register(&TelemetryManager{}, &TelemetryManagerList{})
}

// +k8s:deepcopy-gen=true

// Status defines the observed state of Module CR.
type Status struct {
	// State signifies current state of Module CR.
	// Value can be one of ("Ready", "Processing", "Error", "Deleting").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Processing;Deleting;Ready;Error
	State State `json:"state"`
}
