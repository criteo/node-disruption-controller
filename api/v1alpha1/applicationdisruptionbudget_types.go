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
	"k8s.io/apimachinery/pkg/labels"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ApplicationDisruptionBudgetSpec defines the desired state of ApplicationDisruptionBudget
type ApplicationDisruptionBudgetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// A NodeDisruption is allowed if at most "maxDisruptions" nodes selected by selectors are unavailable after the disruption.
	MaxDisruptions int `json:"maxDisruptions"`
	// PodSelector query over pods whose nodes are managed by the disruption budget.
	PodSelector metav1.LabelSelector `json:"podSelector,omitempty"`
	// PVCSelector query over PVCs whose nodes are managed by the disruption budget.
	PVCSelector metav1.LabelSelector `json:"pvcSelector,omitempty"`
	// Define a optional hook to call when validating a NodeDisruption.
	// It perform a POST http request containing the NodeDisruption that is being validated.
	// Maintenance will proceed only if the endpoint responds 2XX.
	// +kubebuilder:validation:Optional
	HealthHook HookSpec `json:"healthHook,omitempty"`

	// HookV2BasePath holds the base path for the prepare, ready, cancel hooks that will be
	// called at different stages of the NodeDisruption lifecycle.
	// A POST http request containing a Disruption that is being reconciled is sent ot each of the hooks.
	// +kubebuilder:validation:Optional
	HookV2BasePath HookSpec `json:"hookV2BasePath,omitempty"`

	// SupportedNodeDisruptionTypes is the list of node disruption types that this budget supports.
	// When set, this budget will only be considered during reconciliation of NodeDisruptions whose type
	// is in this list. When empty, the controller's default node disruption types are used.
	// +kubebuilder:validation:Optional
	SupportedNodeDisruptionTypes []string `json:"supportedNodeDisruptionTypes,omitempty"`
}

type HookSpec struct {
	// a PEM encoded CA bundle which will be used to validate the webhook's server certificate. If unspecified, system trust roots on the apiserver are used.
	CaBundle string `json:"caBundle,omitempty"`
	// URL that will be called by the hook, in standard URL form (`scheme://host:port/path`).
	URL string `json:"url,omitempty"`
}

// DisruptionBudgetStatus defines the observed state of ApplicationDisruptionBudget
type DisruptionBudgetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// List of nodes that are being watched by the controller
	// Disruption on this nodes will will be made according to the budget
	// of this cluster.
	WatchedNodes []string `json:"watchedNodes,omitempty"`

	// Number of disruption allowed on the nodes of this
	// +kubebuilder:default=0
	DisruptionsAllowed int `json:"disruptionsAllowed"`

	// Number of disruption currently seen on the cluster
	// +kubebuilder:default=0
	CurrentDisruptions int `json:"currentDisruptions"`

	// Disruptions contains a list of disruptions that are related to the budget
	Disruptions []Disruption `json:"disruptions"`
}

// Basic information about disruptions
type Disruption struct {
	// Name of the disruption
	Name string `json:"name"`
	// State of the disruption
	State string `json:"state"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=adb
//+kubebuilder:printcolumn:name="Max Unavailable",type=integer,JSONPath=`.spec.maxDisruptions`
//+kubebuilder:printcolumn:name="Disruptions Allowed",type=integer,JSONPath=`.status.disruptionsAllowed`
//+kubebuilder:printcolumn:name="Current Disruptions",type=integer,JSONPath=`.status.currentDisruptions`

// ApplicationDisruptionBudget is the Schema for the applicationdisruptionbudgets API
type ApplicationDisruptionBudget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationDisruptionBudgetSpec `json:"spec,omitempty"`
	Status DisruptionBudgetStatus          `json:"status,omitempty"`
}

// SelectorMatchesObject return true if the object is matched by one of the selectors
func (adb *ApplicationDisruptionBudget) SelectorMatchesObject(object client.Object) bool {
	objectLabelSet := labels.Set(object.GetLabels())

	switch object.(type) {
	case *corev1.Pod:
		selector, _ := metav1.LabelSelectorAsSelector(&adb.Spec.PodSelector)
		if selector.Empty() {
			return false
		}
		return selector.Matches(objectLabelSet)
	case *corev1.PersistentVolumeClaim:
		selector, _ := metav1.LabelSelectorAsSelector(&adb.Spec.PVCSelector)
		if selector.Empty() {
			return false
		}
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

// ApplicationDisruptionBudgetList contains a list of ApplicationDisruptionBudget
type ApplicationDisruptionBudgetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ApplicationDisruptionBudget `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ApplicationDisruptionBudget{}, &ApplicationDisruptionBudgetList{})
}
