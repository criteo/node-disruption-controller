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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NodeDisruptionBudgetSpec defines the desired state of NodeDisruptionBudget
type NodeDisruptionBudgetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// A NodeDisruption is allowed if at most "maxDisruptedNodes" nodes selected by selectors are unavailable after the disruption.
	MaxDisruptedNodes int `json:"maxDisruptedNodes"`
	// A NodeDisruption is allowed if at most "minUndisruptedNodes" nodes selected by selectors are unavailable after the disruption.
	MinUndisruptedNodes int `json:"minUndisruptedNodes"`
	// NodeSelector query over pods whose nodes are managed by the disruption budget.
	NodeSelector metav1.LabelSelector `json:"nodeSelector,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster,shortName=ndb
//+kubebuilder:printcolumn:name="Max Disrupted Nodes",type=integer,JSONPath=`.spec.maxDisruptedNodes`
//+kubebuilder:printcolumn:name="Min Undisrupted Nodes",type=integer,JSONPath=`.spec.minUndisruptedNodes`
//+kubebuilder:printcolumn:name="Disruptions Allowed",type=integer,JSONPath=`.status.disruptionsAllowed`
//+kubebuilder:printcolumn:name="Current Disruptions",type=integer,JSONPath=`.status.currentDisruptions`

// NodeDisruptionBudget is the Schema for the nodedisruptionbudgets API
type NodeDisruptionBudget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeDisruptionBudgetSpec `json:"spec,omitempty"`
	Status DisruptionBudgetStatus   `json:"status,omitempty"`
}

// SelectorMatchesObject return true if the object is matched by one of the selectors
func (adb *NodeDisruptionBudget) SelectorMatchesObject(object client.Object) bool {
	objectLabelSet := labels.Set(object.GetLabels())

	switch object.(type) {
	case *corev1.Node:
		selector, _ := metav1.LabelSelectorAsSelector(&adb.Spec.NodeSelector)
		return selector.Matches(objectLabelSet)
	case *NodeDisruption:
		// It is faster to trigger a reconcile for each ADB instead of checking if the
		// Node Disruption is impacting the current ADB
		return true
	default:
		return false
	}
}

//+kubebuilder:object:root=true

// NodeDisruptionBudgetList contains a list of NodeDisruptionBudget
type NodeDisruptionBudgetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeDisruptionBudget `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeDisruptionBudget{}, &NodeDisruptionBudgetList{})
}
