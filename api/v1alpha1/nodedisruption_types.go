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

// This is the same as types.NamespacedName but serialisable to JSON
type NamespacedName struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// NodeDisruptionSpec defines the desired state of NodeDisruption
type NodeDisruptionSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Label query over nodes that will be impacted by the disruption
	NodeSelector metav1.LabelSelector `json:"nodeSelector,omitempty"`
}

// NodeDisruptionStatus defines the observed state of NodeDisruption (/!\ it is eventually consistent)
type NodeDisruptionStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// List of all the ApplicationDisruptionBudget that are disrupted by this NodeDisruption
	DisruptedADB []NamespacedName `json:"disruptedADB"`
	// List of all the NDB that are disrupted by this NodeDisruption
	DisruptedNodes []string `json:"disruptedNodes"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// NodeDisruption is the Schema for the nodedisruptions API
type NodeDisruption struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeDisruptionSpec   `json:"spec,omitempty"`
	Status NodeDisruptionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NodeDisruptionList contains a list of NodeDisruption
type NodeDisruptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeDisruption `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeDisruption{}, &NodeDisruptionList{})
}
