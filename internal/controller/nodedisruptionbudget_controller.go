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
	"reflect"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nodedisruptionv1alpha1 "github.com/criteo/node-disruption-controller/api/v1alpha1"
	"github.com/criteo/node-disruption-controller/pkg/resolver"
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
		if errors.IsNotFound(err) {
			// If the ressource was not found, nothing has to be done
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	resolver := NodeDisruptionBudgetResolver{
		NodeDisruptionBudget: ndb.DeepCopy(),
		Client:               r.Client,
		Resolver:             resolver.Resolver{Client: r.Client},
	}

	err = resolver.Sync(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !reflect.DeepEqual(resolver.NodeDisruptionBudget.Status, ndb.Status) {
		err = resolver.UpdateStatus(ctx)
	}

	return ctrl.Result{}, err
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
	Resolver             resolver.Resolver
}

// Sync ensure the budget's status is up to date
func (r *NodeDisruptionBudgetResolver) Sync(ctx context.Context) error {
	nodeNames, err := r.GetSelectedNodes(ctx)
	if err != nil {
		return err
	}

	nodes := resolver.NodeSetToStringList(nodeNames)

	disruptionCount, err := r.ResolveDisruption(ctx)
	if err != nil {
		return err
	}

	r.NodeDisruptionBudget.Status.WatchedNodes = nodes
	r.NodeDisruptionBudget.Status.CurrentDisruptions = disruptionCount
	disruptionsForMax := r.NodeDisruptionBudget.Spec.MaxDisruptedNodes - disruptionCount
	disruptionsForMin := (len(nodes) - disruptionCount) - r.NodeDisruptionBudget.Spec.MinUndisruptedNodes
	r.NodeDisruptionBudget.Status.DisruptionsAllowed = int(math.Min(float64(disruptionsForMax), float64(disruptionsForMin))) - disruptionCount

	return nil
}

// Check if the budget would be impacted by an operation on the provided set of nodes
func (r *NodeDisruptionBudgetResolver) IsImpacted(disruptedNodes resolver.NodeSet) bool {
	watchedNodes := resolver.NewNodeSetFromStringList(r.NodeDisruptionBudget.Status.WatchedNodes)
	return watchedNodes.Intersection(disruptedNodes).Len() > 0
}

// Return the number of disruption allowed considering a list of current node disruptions
func (r *NodeDisruptionBudgetResolver) TolerateDisruption(disruptedNodes resolver.NodeSet) bool {
	watchedNodes := resolver.NewNodeSetFromStringList(r.NodeDisruptionBudget.Status.WatchedNodes)
	disruptedNodesCount := watchedNodes.Intersection(disruptedNodes).Len()
	return r.NodeDisruptionBudget.Status.DisruptionsAllowed-disruptedNodesCount >= 0
}

// NodeDisruption CheckHealth is always true
func (r *NodeDisruptionBudgetResolver) CheckHealth(context.Context) error {
	return nil
}

// Call a lifecycle hook in order to synchronously validate a Node Disruption
func (r *NodeDisruptionBudgetResolver) CallHealthHook(_ context.Context, _ nodedisruptionv1alpha1.NodeDisruption) error {
	return nil
}

func (r *NodeDisruptionBudgetResolver) UpdateStatus(ctx context.Context) error {
	return r.Client.Status().Update(ctx, r.NodeDisruptionBudget.DeepCopy(), []client.SubResourceUpdateOption{}...)
}

func (r *NodeDisruptionBudgetResolver) GetNamespacedName() nodedisruptionv1alpha1.NamespacedName {
	return nodedisruptionv1alpha1.NamespacedName{
		Namespace: r.NodeDisruptionBudget.Namespace,
		Name:      r.NodeDisruptionBudget.Name,
		Kind:      r.NodeDisruptionBudget.Kind,
	}
}

func (r *NodeDisruptionBudgetResolver) GetSelectedNodes(ctx context.Context) (resolver.NodeSet, error) {
	nodesFromPods, err := r.Resolver.GetNodeFromNodeSelector(ctx, r.NodeDisruptionBudget.Spec.NodeSelector)
	if err != nil {
		return resolver.NodeSet{}, err
	}

	return nodesFromPods, nil
}

func (r *NodeDisruptionBudgetResolver) ResolveDisruption(ctx context.Context) (int, error) {
	selectedNodes, err := r.GetSelectedNodes(ctx)
	if err != nil {
		return 0, err
	}

	disruptions := 0

	opts := []client.ListOption{}
	nodeDisruptions := &nodedisruptionv1alpha1.NodeDisruptionList{}

	err = r.Client.List(ctx, nodeDisruptions, opts...)
	if err != nil {
		return 0, err
	}

	for _, nd := range nodeDisruptions.Items {
		if nd.Status.State != nodedisruptionv1alpha1.Granted {
			continue
		}

		impactedNodes, err := r.Resolver.GetNodeFromNodeSelector(ctx, nd.Spec.NodeSelector)
		if err != nil {
			return 0, err
		}
		if selectedNodes.Intersection(impactedNodes).Len() > 0 {
			disruptions++
		}
	}
	return disruptions, nil
}
