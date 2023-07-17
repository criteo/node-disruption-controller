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
	"math"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nodedisruptionv1alpha1 "github.com/criteo/node-disruption-controller/api/v1alpha1"
	"github.com/golang-collections/collections/set"
)

// NodeDisruptionBudgetReconciler reconciles a NodeDisruptionBudget object
type NodeDisruptionBudgetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=nodedisruption.criteo.com,resources=nodedisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nodedisruption.criteo.com,resources=nodedisruptionbudgets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nodedisruption.criteo.com,resources=nodedisruptionbudgets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the NodeDisruptionBudget object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *NodeDisruptionBudgetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ndb := &nodedisruptionv1alpha1.NodeDisruptionBudget{}
	err := r.Client.Get(ctx, req.NamespacedName, ndb)

	if err != nil {
		return ctrl.Result{}, err
	}

	resolver := NodeDisruptionBudgetResolver{
		NodeDisruptionBudget: ndb,
		Client:               r.Client,
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

	ndb.Status.WatchedNodes = nodes
	ndb.Status.CurrentDisruptions = disruption_nr
	disruptions_for_max := ndb.Spec.MaxDisruptedNodes - disruption_nr
	disruptions_for_min := (len(nodes) - disruption_nr) - ndb.Spec.MinUndisruptedNodes
	ndb.Status.DisruptionsAllowed = int(math.Min(float64(disruptions_for_max), float64(disruptions_for_min))) - disruption_nr

	err = r.Status().Update(ctx, ndb, []client.SubResourceUpdateOption{}...)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeDisruptionBudgetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&nodedisruptionv1alpha1.NodeDisruptionBudget{}).
		Complete(r)
}

type NodeDisruptionBudgetResolver struct {
	NodeDisruptionBudget *nodedisruptionv1alpha1.NodeDisruptionBudget
	Client               client.Client
}

func (ndbr *NodeDisruptionBudgetResolver) ResolveNodes(ctx context.Context) (*set.Set, error) {
	node_names := set.New()

	nodes_from_pods, err := ndbr.ResolveFromNodeSelector(ctx)
	if err != nil {
		return node_names, err
	}

	return nodes_from_pods, nil
}

func (ndbr *NodeDisruptionBudgetResolver) ResolveFromNodeSelector(ctx context.Context) (*set.Set, error) {
	node_names := set.New()
	selector, err := metav1.LabelSelectorAsSelector(&ndbr.NodeDisruptionBudget.Spec.NodeSelector)
	if err != nil || selector.Empty() {
		return node_names, err
	}
	opts := []client.ListOption{
		client.MatchingLabelsSelector{Selector: selector},
	}
	nodes := &corev1.NodeList{}
	err = ndbr.Client.List(ctx, nodes, opts...)
	if err != nil {
		return node_names, err
	}

	for _, node := range nodes.Items {
		node_names.Insert(node.ObjectMeta.Name)
	}
	return node_names, nil
}

func (ndbr *NodeDisruptionBudgetResolver) ResolveDisruption(ctx context.Context) (int, error) {
	selected_nodes, err := ndbr.ResolveNodes(ctx)
	if err != nil {
		return 0, err
	}

	disruptions := 0

	opts := []client.ListOption{}
	node_disruptions := &nodedisruptionv1alpha1.NodeDisruptionList{}

	err = ndbr.Client.List(ctx, node_disruptions, opts...)
	if err != nil {
		return 0, err
	}

	for _, nd := range node_disruptions.Items {
		if nd.Spec.State != nodedisruptionv1alpha1.Granted {
			continue
		}
		node_disruption_resolver := NodeDisruptionResolver{
			NodeDisruption: &nd,
			Client:         ndbr.Client,
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
func (ndbr *NodeDisruptionBudgetResolver) AllowDisruption(ctx context.Context, nodes *set.Set) (disrupted, allowed bool, err error) {
	selected_nodes, err := ndbr.ResolveNodes(ctx)
	if err != nil {
		return false, false, err
	}
	if nodes.Intersection(selected_nodes).Len() == 0 {
		return false, false, nil
	}

	disruption_nr, err := ndbr.ResolveDisruption(ctx)
	if err != nil {
		return true, false, err
	}

	disruptions_for_max := ndbr.NodeDisruptionBudget.Spec.MaxDisruptedNodes - disruption_nr
	disruptions_for_min := (selected_nodes.Len() - disruption_nr) - ndbr.NodeDisruptionBudget.Spec.MinUndisruptedNodes
	disruption_allowed := int(math.Min(float64(disruptions_for_max), float64(disruptions_for_min))) - disruption_nr

	if disruption_nr+1 > disruption_allowed {
		return true, false, nil
	}
	return true, true, nil
}
