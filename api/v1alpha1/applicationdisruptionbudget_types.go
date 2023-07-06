/*
Copyright 2023.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ApplicationDisruptionBudgetSpec defines the desired state of ApplicationDisruptionBudget
type ApplicationDisruptionBudgetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// A NodeDisruption is allowed if at most "maxUnavailable" nodes selected by selectors are unavailable after the disruption.
	MaxUnavailable int `json:"maxUnavailable,omitempty"`
	// A MinAvailable is allowed if at least "MinAvailable" nodes selected by selectors will still be available after the disruption.
	MinAvailable int `json:"minAvailable,omitempty"`
	// PodSelector query over pods whose nodes are managed by the disruption budget.
	PodSelector metav1.LabelSelector `json:"podSelector,omitempty"`
	// PVCSelector query over PVCs whose nodes are managed by the disruption budget.
	PVCSelector metav1.LabelSelector `json:"pvcSelector,omitempty"`
}

// ApplicationDisruptionBudgetStatus defines the observed state of ApplicationDisruptionBudget
type ApplicationDisruptionBudgetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// List of nodes that are being watched by the controller
	// Disruption on this nodes will will be made according to the budget
	// of this cluster.
	WatchedNodes []string `json:"watchedNodes"`

	// Number of disruption allowed on the nodes of this
	DisruptionsAllowed int `json:"disruptionsAllowed"`

	// Number of disruption currently seen on the cluster
	CurrentDisruptions int `json:"currentDisruptions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ApplicationDisruptionBudget is the Schema for the applicationdisruptionbudgets API
type ApplicationDisruptionBudget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationDisruptionBudgetSpec   `json:"spec,omitempty"`
	Status ApplicationDisruptionBudgetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ApplicationDisruptionBudgetList contains a list of ApplicationDisruptionBudget
type ApplicationDisruptionBudgetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ApplicationDisruptionBudget `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ApplicationDisruptionBudget{}, &ApplicationDisruptionBudgetList{})
}
