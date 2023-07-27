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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	nodedisruptionv1alpha1 "github.com/criteo/node-disruption-controller/api/v1alpha1"
	"github.com/golang-collections/collections/set"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeDisruptionReconciler reconciles a NodeDisruption object
type NodeDisruptionReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=nodedisruption.criteo.com,resources=nodedisruptions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nodedisruption.criteo.com,resources=nodedisruptions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nodedisruption.criteo.com,resources=nodedisruptions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the NodeDisruption object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *NodeDisruptionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Start reconcile of NodeDisruption")

	nd := &nodedisruptionv1alpha1.NodeDisruption{}
	err := r.Client.Get(ctx, req.NamespacedName, nd)

	if err != nil {
		return ctrl.Result{}, err
	}

	// TODO: check to see if another one is marked in progress
	if nd.Spec.State == nodedisruptionv1alpha1.Pending {
		nd.Spec.State = nodedisruptionv1alpha1.Processing
		err = r.Update(ctx, nd.DeepCopy(), []client.UpdateOption{}...)
		if err != nil {
			return ctrl.Result{}, err
		}

		nd := &nodedisruptionv1alpha1.NodeDisruption{}
		err := r.Client.Get(ctx, req.NamespacedName, nd)

		if err != nil {
			return ctrl.Result{}, err
		}
	}

	resolver := NodeDisruptionResolver{
		NodeDisruption: nd,
		Client:         r.Client,
	}

	budgets, err := resolver.GetAllBudgetsInSync(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	disruption, err := resolver.GetDisruption(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	any_failed, statuses := resolver.ValidateDisruption(ctx, budgets, disruption)

	nd.Status.DisruptedDisruptionBudgets = statuses
	nd.Status.DisruptedNodes = NodeSetToStringList(disruption.ImpactedNodes)

	if nd.Spec.State == nodedisruptionv1alpha1.Processing {
		if !any_failed {
			nd.Spec.State = nodedisruptionv1alpha1.Granted
		} else {
			nd.Spec.State = nodedisruptionv1alpha1.Rejected
		}
	}

	err = r.Update(ctx, nd.DeepCopy(), []client.UpdateOption{}...)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.Status().Update(ctx, nd, []client.SubResourceUpdateOption{}...)
	if err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Reconcilation successful", "state", nd.Spec.State)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeDisruptionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("node-disruption-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&nodedisruptionv1alpha1.NodeDisruption{}).
		Complete(r)
}

type NodeDisruptionResolver struct {
	NodeDisruption *nodedisruptionv1alpha1.NodeDisruption
	Client         client.Client
}

// Resolve the nodes impacted by the NodeDisruption
func (ndr *NodeDisruptionResolver) GetDisruption(ctx context.Context) (NodeDisruption, error) {
	node_names := set.New()
	selector, err := metav1.LabelSelectorAsSelector(&ndr.NodeDisruption.Spec.NodeSelector)
	if err != nil {
		return NodeDisruption{}, err
	}
	opts := []client.ListOption{
		client.MatchingLabelsSelector{Selector: selector},
	}
	nodes := &corev1.NodeList{}
	err = ndr.Client.List(ctx, nodes, opts...)
	if err != nil {
		return NodeDisruption{}, err
	}

	for _, node := range nodes.Items {
		node_names.Insert(node.Name)
	}
	return NodeDisruption{
		ImpactedNodes: node_names,
	}, nil
}

// GetAllBudgetsInSync fetch all the budgets from Kubernetes and synchronise them
func (ndr *NodeDisruptionResolver) GetAllBudgetsInSync(ctx context.Context) ([]Budget, error) {
	opts := []client.ListOption{}
	budgets := []Budget{}

	application_disruption_budgets := &nodedisruptionv1alpha1.ApplicationDisruptionBudgetList{}
	err := ndr.Client.List(ctx, application_disruption_budgets, opts...)
	if err != nil {
		return budgets, err
	}
	for _, adb := range application_disruption_budgets.Items {
		adb_resolver := ApplicationDisruptionBudgetResolver{
			ApplicationDisruptionBudget: &adb,
			Client:                      ndr.Client,
		}
		budgets = append(budgets, &adb_resolver)
	}

	node_disruption_budget := &nodedisruptionv1alpha1.NodeDisruptionBudgetList{}
	err = ndr.Client.List(ctx, node_disruption_budget, opts...)
	if err != nil {
		return budgets, err
	}
	for _, ndb := range node_disruption_budget.Items {
		ndb_resolver := NodeDisruptionBudgetResolver{
			NodeDisruptionBudget: &ndb,
			Client:               ndr.Client,
		}
		budgets = append(budgets, &ndb_resolver)
	}

	for _, budget := range budgets {
		err := budget.Sync(ctx)
		if err != nil {
			return budgets, err
		}
	}

	return budgets, nil
}

// Validate a disruption give a list of budget
func (ndr *NodeDisruptionResolver) ValidateDisruption(ctx context.Context, budgets []Budget, nd NodeDisruption) (any_failed bool, status []nodedisruptionv1alpha1.DisruptedBudgetStatus) {
	any_failed = false

	impacted_budgets := []Budget{}
	for _, budget := range budgets {
		if !budget.IsImpacted(nd) {
			continue
		}

		if !budget.TolerateDisruption(nd) {
			any_failed = true
			status = append(status, nodedisruptionv1alpha1.DisruptedBudgetStatus{
				Reference: budget.GetNamespacedName(),
				Reason:    "No more disruption allowed",
				Ok:        false,
			})
			break
		}
		impacted_budgets = append(impacted_budgets, budget)
	}

	if any_failed {
		return any_failed, status
	}

	for _, budget := range impacted_budgets {
		err := budget.CheckHealth(ctx)
		if err != nil {
			any_failed = true
			status = append(status, nodedisruptionv1alpha1.DisruptedBudgetStatus{
				Reference: budget.GetNamespacedName(),
				Reason:    fmt.Sprintf("Unhealthy: %s", err),
				Ok:        false,
			})
			break
		}
		status = append(status, nodedisruptionv1alpha1.DisruptedBudgetStatus{
			Reference: budget.GetNamespacedName(),
			Reason:    "",
			Ok:        true,
		})
	}
	return any_failed, status
}
