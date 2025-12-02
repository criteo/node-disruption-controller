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
	"time"

	nodedisruptionv1alpha1 "github.com/criteo/node-disruption-controller/api/v1alpha1"

	"github.com/criteo/node-disruption-controller/pkg/resolver"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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
	logger := log.FromContext(ctx)
	ndb := &nodedisruptionv1alpha1.NodeDisruptionBudget{}
	err := r.Get(ctx, req.NamespacedName, ndb)
	ref := nodedisruptionv1alpha1.NamespacedName{
		Namespace: req.Namespace,
		Name:      req.Name,
		Kind:      "NodeDisruptionBudget",
	}

	if err != nil {
		if errors.IsNotFound(err) {
			// If the resource was not found, nothing has to be done
			PruneNDBMetrics(ref)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	UpdateNDBMetrics(ref, ndb)
	logger.Info("Start reconcile of NDB", "version", ndb.ResourceVersion)

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

// PruneNodeDisruptionMetric remove metrics for a NDB that don't exist anymore
func PruneNDBMetrics(ref nodedisruptionv1alpha1.NamespacedName) {
	DisruptionBudgetMaxDisruptedNodes.DeletePartialMatch(prometheus.Labels{"budget_disruption_namespace": ref.Namespace, "budget_disruption_name": ref.Name, "budget_disruption_kind": ref.Kind})
	DisruptionBudgetMinUndisruptedNodes.DeletePartialMatch(prometheus.Labels{"budget_disruption_namespace": ref.Namespace, "budget_disruption_name": ref.Name, "budget_disruption_kind": ref.Kind})
	PruneBudgetStatusMetrics(ref)
}

// UpdateNDBMetrics update metrics for a NDB
func UpdateNDBMetrics(ref nodedisruptionv1alpha1.NamespacedName, ndb *nodedisruptionv1alpha1.NodeDisruptionBudget) {
	DisruptionBudgetMaxDisruptedNodes.WithLabelValues(ref.Namespace, ref.Name, ref.Kind).Set(float64(ndb.Spec.MaxDisruptedNodes))
	DisruptionBudgetMinUndisruptedNodes.WithLabelValues(ref.Namespace, ref.Name, ref.Kind).Set(float64(ndb.Spec.MinUndisruptedNodes))
	UpdateBudgetStatusMetrics(ref, ndb.Status)
}

// MapFuncBuilder returns a MapFunc that is used to dispatch reconcile requests to
// budgets when an event is triggered by one of their matching object
func (r *NodeDisruptionBudgetReconciler) MapFuncBuilder() handler.MapFunc {
	// Look for all NDBs in the namespace, then see if they match the object
	return func(ctx context.Context, object client.Object) (requests []reconcile.Request) {
		ndbs := nodedisruptionv1alpha1.NodeDisruptionBudgetList{}
		err := r.List(ctx, &ndbs, &client.ListOptions{Namespace: object.GetNamespace()})
		if err != nil {
			// We cannot return an error so at least it should be logged
			logger := log.FromContext(context.Background())
			logger.Error(err, "Could not list NDBs in watch function")
			return requests
		}

		for _, ndb := range ndbs.Items {
			if ndb.SelectorMatchesObject(object) {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      ndb.Name,
						Namespace: ndb.Namespace,
					},
				})
			}
		}
		return requests
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeDisruptionBudgetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.TypedOptions[reconcile.Request]{SkipNameValidation: ptr.To(true)}). // TODO(j.clerc): refactor tests to avoid skipping name validation
		For(&nodedisruptionv1alpha1.NodeDisruptionBudget{}).
		Watches(
			&corev1.Node{},
			handler.EnqueueRequestsFromMapFunc(r.MapFuncBuilder()),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(
			&nodedisruptionv1alpha1.NodeDisruption{},
			handler.EnqueueRequestsFromMapFunc(r.MapFuncBuilder()),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
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

	disruptionCount, disruptions, err := r.ResolveDisruption(ctx)
	if err != nil {
		return err
	}

	r.NodeDisruptionBudget.Status.WatchedNodes = nodes
	r.NodeDisruptionBudget.Status.CurrentDisruptions = disruptionCount
	disruptionsForMax := r.NodeDisruptionBudget.Spec.MaxDisruptedNodes - disruptionCount
	disruptionsForMin := (len(nodes) - disruptionCount) - r.NodeDisruptionBudget.Spec.MinUndisruptedNodes
	r.NodeDisruptionBudget.Status.DisruptionsAllowed = int(math.Min(float64(disruptionsForMax), float64(disruptionsForMin))) - disruptionCount
	r.NodeDisruptionBudget.Status.Disruptions = disruptions
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

func (r *NodeDisruptionBudgetResolver) V2HooksReady() bool {
	return false
}

func (r *NodeDisruptionBudgetResolver) CallPrepareHook(ctx context.Context, nd nodedisruptionv1alpha1.NodeDisruption, timeout time.Duration) error {
	return nil
}

func (r *NodeDisruptionBudgetResolver) CallReadyHook(ctx context.Context, nd nodedisruptionv1alpha1.NodeDisruption, timeout time.Duration) error {
	return nil
}

func (r *NodeDisruptionBudgetResolver) CallCancelHook(ctx context.Context, nd nodedisruptionv1alpha1.NodeDisruption, timeout time.Duration) error {
	return nil
}

// Call a lifecycle hook in order to synchronously validate a Node Disruption
func (r *NodeDisruptionBudgetResolver) CallHealthHook(_ context.Context, _ nodedisruptionv1alpha1.NodeDisruption, _ time.Duration) error {
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

func (r *NodeDisruptionBudgetResolver) ResolveDisruption(ctx context.Context) (int, []nodedisruptionv1alpha1.Disruption, error) {
	disruptions := []nodedisruptionv1alpha1.Disruption{}
	selectedNodes, err := r.GetSelectedNodes(ctx)
	if err != nil {
		return 0, disruptions, err
	}

	disruptionCount := 0

	opts := []client.ListOption{}
	nodeDisruptions := &nodedisruptionv1alpha1.NodeDisruptionList{}

	err = r.Client.List(ctx, nodeDisruptions, opts...)
	if err != nil {
		return 0, disruptions, err
	}

	for _, nd := range nodeDisruptions.Items {
		if nd.Status.State == nodedisruptionv1alpha1.Unknown {
			// node disruption has just been created and hasn't been reconciliated yet
			continue
		}

		impactedNodes, err := r.Resolver.GetNodeFromNodeSelector(ctx, nd.Spec.NodeSelector)
		if err != nil {
			return 0, disruptions, err
		}

		if selectedNodes.Intersection(impactedNodes).Len() > 0 {
			if nd.Status.State == nodedisruptionv1alpha1.Granted {
				disruptionCount++
			}
			disruptions = append(disruptions, nodedisruptionv1alpha1.Disruption{
				Name:  nd.Name,
				State: string(nd.Status.State),
			})
		}
	}
	return disruptionCount, disruptions, err
}
