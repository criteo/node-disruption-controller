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

package controller

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nodedisruptionv1alpha1 "github.com/criteo/node-disruption-controller/api/v1alpha1"

	"github.com/golang-collections/collections/set"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApplicationDisruptionBudgetReconciler reconciles a ApplicationDisruptionBudget object
type ApplicationDisruptionBudgetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=nodedisruption.criteo.com,resources=applicationdisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nodedisruption.criteo.com,resources=applicationdisruptionbudgets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nodedisruption.criteo.com,resources=applicationdisruptionbudgets/finalizers,verbs=update

//+kubebuilder:rbac:groups="",resources=pods;persistentvolumeclaims;persistentvolumes;nodes,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ApplicationDisruptionBudget object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *ApplicationDisruptionBudgetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	adb := &nodedisruptionv1alpha1.ApplicationDisruptionBudget{}
	err := r.Client.Get(ctx, req.NamespacedName, adb)

	if err != nil {
		return ctrl.Result{}, err
	}

	resolver := ApplicationDisruptionBudgetResolver{
		ApplicationDisruptionBudget: adb,
		Client:                      r.Client,
	}

	node_names, err := resolver.ResolveNodes(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create a slice to store the set elements
	nodes := make([]string, 0, node_names.Len())

	// Iterate over the set and append elements to the slice
	node_names.Do(func(item interface{}) {
		nodes = append(nodes, item.(string))
	})

	disruption_nr, err := resolver.ResolveDisruption(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	adb.Status.WatchedNodes = nodes
	adb.Status.CurrentDisruptions = disruption_nr
	adb.Status.DisruptionsAllowed = adb.Spec.MaxDisruptions - disruption_nr

	err = r.Status().Update(ctx, adb, []client.SubResourceUpdateOption{}...)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationDisruptionBudgetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&nodedisruptionv1alpha1.ApplicationDisruptionBudget{}).
		Complete(r)
}

type ApplicationDisruptionBudgetResolver struct {
	ApplicationDisruptionBudget *nodedisruptionv1alpha1.ApplicationDisruptionBudget
	Client                      client.Client
}

func (adbr *ApplicationDisruptionBudgetResolver) ResolveNodes(ctx context.Context) (*set.Set, error) {
	node_names := set.New()

	nodes_from_pods, err := adbr.ResolveFromPodSelector(ctx)
	if err != nil {
		return node_names, err
	}
	nodes_from_PVCs, err := adbr.ResolveFromPVCSelector(ctx)
	if err != nil {
		return node_names, err
	}

	return nodes_from_pods.Union(nodes_from_PVCs), nil
}

func (adbr *ApplicationDisruptionBudgetResolver) ResolveFromPodSelector(ctx context.Context) (*set.Set, error) {
	node_names := set.New()
	selector, err := metav1.LabelSelectorAsSelector(&adbr.ApplicationDisruptionBudget.Spec.PodSelector)
	if err != nil || selector.Empty() {
		return node_names, err
	}
	opts := []client.ListOption{
		client.InNamespace(adbr.ApplicationDisruptionBudget.Namespace),
		client.MatchingLabelsSelector{Selector: selector},
	}
	pods := &corev1.PodList{}
	err = adbr.Client.List(ctx, pods, opts...)
	if err != nil {
		return node_names, err
	}

	for _, pod := range pods.Items {
		node_names.Insert(pod.Spec.NodeName)
	}
	return node_names, nil
}

// NodeLabelSelectorAsRequirement converts a NodeSelectorRequirement to a labels.Requirement
// I have not been able to find a function for that in Kubernetes code, if it exists please replace this
func NodeLabelSelectorAsRequirement(expr *corev1.NodeSelectorRequirement) (*labels.Requirement, error) {
	var op selection.Operator
	switch expr.Operator {
	case corev1.NodeSelectorOpIn:
		op = selection.In
	case corev1.NodeSelectorOpNotIn:
		op = selection.NotIn
	case corev1.NodeSelectorOpExists:
		op = selection.Exists
	case corev1.NodeSelectorOpDoesNotExist:
		op = selection.DoesNotExist
	default:
		return nil, fmt.Errorf("%q is not a valid label selector operator", expr.Operator)
	}
	return labels.NewRequirement(expr.Key, op, append([]string(nil), expr.Values...))
}

// NodeSelectorAsSelector converts a NodeSelector to a label selector and field selector
// I have not been able to find a function for that in Kubernetes code, if it exists please replace this
func NodeSelectorAsSelector(ns *corev1.NodeSelector) (labels.Selector, fields.Selector, error) {
	if ns == nil {
		return labels.Nothing(), fields.Nothing(), nil
	}

	if len(ns.NodeSelectorTerms) == 0 {
		return labels.Everything(), fields.Everything(), nil
	}

	labels_requirements := make([]labels.Requirement, 0, len(ns.NodeSelectorTerms))
	fields_requirements := make([]string, 0, len(ns.NodeSelectorTerms))

	for _, term := range ns.NodeSelectorTerms {
		for _, expr := range term.MatchExpressions {
			r, err := NodeLabelSelectorAsRequirement(&expr)
			if err != nil {
				return nil, nil, err
			}
			labels_requirements = append(labels_requirements, *r)
		}

		for _, expr := range term.MatchFields {
			r, err := NodeLabelSelectorAsRequirement(&expr)
			if err != nil {
				return nil, nil, err
			}
			fields_requirements = append(fields_requirements, r.String())
		}
	}

	label_selector := labels.NewSelector()
	label_selector = label_selector.Add(labels_requirements...)
	field_selector, err := fields.ParseSelector(strings.Join(fields_requirements, ","))
	return label_selector, field_selector, err
}

func (adbr *ApplicationDisruptionBudgetResolver) ResolveFromPVCSelector(ctx context.Context) (*set.Set, error) {
	node_names := set.New()
	selector, err := metav1.LabelSelectorAsSelector(&adbr.ApplicationDisruptionBudget.Spec.PVCSelector)
	if err != nil {
		return node_names, err
	}
	opts := []client.ListOption{
		client.InNamespace(adbr.ApplicationDisruptionBudget.Namespace),
		client.MatchingLabelsSelector{Selector: selector},
	}
	PVCs := &corev1.PersistentVolumeClaimList{}
	err = adbr.Client.List(ctx, PVCs, opts...)
	if err != nil {
		return node_names, err
	}

	pvs_to_fetch := []string{}

	for _, pvc := range PVCs.Items {
		pvs_to_fetch = append(pvs_to_fetch, pvc.Spec.VolumeName)
	}

	get_options := []client.GetOption{}
	for _, pv_name := range pvs_to_fetch {
		pv := &corev1.PersistentVolume{}

		err = adbr.Client.Get(ctx, types.NamespacedName{Name: pv_name, Namespace: ""}, pv, get_options...)
		if err != nil {
			return node_names, err
		}

		node_selector := pv.Spec.NodeAffinity.Required
		if node_selector == nil {
			continue
		}

		opts := []client.ListOption{}
		label_selector, field_selector, err := NodeSelectorAsSelector(node_selector)
		if err != nil {
			return node_names, err
		}

		if label_selector.Empty() && field_selector.Empty() {
			// Ignore this PV
			fmt.Printf("skipping %s, no affinity", pv_name)
			continue
		}

		if !label_selector.Empty() {
			opts = append(opts, client.MatchingLabelsSelector{Selector: label_selector})
		}

		if !field_selector.Empty() {
			opts = append(opts, client.MatchingFieldsSelector{Selector: field_selector})
		}

		nodes := &corev1.NodeList{}
		err = adbr.Client.List(ctx, nodes, opts...)
		if err != nil {
			return node_names, err
		}

		for _, node := range nodes.Items {
			node_names.Insert(node.Name)
		}
	}

	return node_names, nil
}

func (adbr *ApplicationDisruptionBudgetResolver) ResolveDisruption(ctx context.Context) (int, error) {
	selected_nodes, err := adbr.ResolveNodes(ctx)
	if err != nil {
		return 0, err
	}

	disruptions := 0

	opts := []client.ListOption{}
	node_disruptions := &nodedisruptionv1alpha1.NodeDisruptionList{}

	err = adbr.Client.List(ctx, node_disruptions, opts...)
	if err != nil {
		return 0, err
	}

	for _, nd := range node_disruptions.Items {
		if nd.Spec.State != nodedisruptionv1alpha1.Granted {
			continue
		}
		node_disruption_resolver := NodeDisruptionResolver{
			NodeDisruption: &nd,
			Client:         adbr.Client,
		}
		disrupted_nodes, err := node_disruption_resolver.ResolveNodes(ctx)
		if err != nil {
			return 0, err
		}
		if selected_nodes.Intersection(disrupted_nodes).Len() > 0 {
			disruptions += 1
		}
	}
	return disruptions, nil
}

// AllowDisruption will be true if the NDB was impacted, Allowed will be true if NDB allow one more disruption
func (adbr *ApplicationDisruptionBudgetResolver) AllowDisruption(ctx context.Context, nodes *set.Set) (disrupted, allowed bool, err error) {
	selected_nodes, err := adbr.ResolveNodes(ctx)
	if err != nil {
		return false, false, err
	}
	if nodes.Intersection(selected_nodes).Len() == 0 {
		return false, false, nil
	}

	disruption, err := adbr.ResolveDisruption(ctx)
	if err != nil {
		return true, false, err
	}

	if disruption+1 > adbr.ApplicationDisruptionBudget.Spec.MaxDisruptions {
		return true, false, nil
	}
	return true, true, nil
}
