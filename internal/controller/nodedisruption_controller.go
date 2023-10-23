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
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	nodedisruptionv1alpha1 "github.com/criteo/node-disruption-controller/api/v1alpha1"
	"github.com/criteo/node-disruption-controller/pkg/resolver"
	"github.com/golang-collections/collections/set"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultRetryInterval = time.Minute
)

type NodeDisruptionReconcilerConfig struct {
	// Whether to grant or reject a node disruption matching no node
	RejectEmptyNodeDisruption bool
	// How long to retry between each validation attempt
	RetryInterval time.Duration
}

// NodeDisruptionReconciler reconciles a NodeDisruption object
type NodeDisruptionReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Config   NodeDisruptionReconcilerConfig
}

//+kubebuilder:rbac:groups=nodedisruption.criteo.com,resources=nodedisruptions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nodedisruption.criteo.com,resources=nodedisruptions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nodedisruption.criteo.com,resources=nodedisruptions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *NodeDisruptionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	ctrl_result := ctrl.Result{}

	nd := &nodedisruptionv1alpha1.NodeDisruption{}
	err := r.Client.Get(ctx, req.NamespacedName, nd)

	if err != nil {
		if errors.IsNotFound(err) {
			// If the ressource was not found, nothing has to be done
			return ctrl_result, nil
		}
		return ctrl_result, err
	}

	logger.Info("Start reconcile of NodeDisruption", "state", nd.Status.State, "retryDate", nd.Status.NextRetryDate.Time)

	if nd.Status.State == "" {
		nd.Status.State = nodedisruptionv1alpha1.Pending
		err = r.Client.Status().Update(ctx, nd, []client.SubResourceUpdateOption{}...)

		// Switch to pending and schedule another reconcile run
		ctrl_result.Requeue = true
		return ctrl_result, err
	}

	if nd.Status.State == nodedisruptionv1alpha1.Pending {
		if time.Now().Before(nd.Status.NextRetryDate.Time) {
			ctrl_result.RequeueAfter = time.Until(nd.Status.NextRetryDate.Time)
			logger.Info("Time not elapsed, retry later", "currentDate", time.Now(), "retryDate", nd.Status.NextRetryDate.Time)
			return ctrl_result, nil
		}

		logger.Info("Trying to validate the node disruption")
		status, err := r.tryValidatingDisruption(ctx, nd)
		if err != nil {
			return ctrl_result, err
		}

		nd.Status = status

		err = r.Client.Status().Update(ctx, nd, []client.SubResourceUpdateOption{}...)
		logger.Info("Updating Status, done with", "state", nd.Status.State)
		if err != nil {
			return ctrl_result, err
		}
	}

	logger.Info("Reconcilation successful", "state", nd.Status.State)
	return ctrl_result, nil
}

func (r *NodeDisruptionReconciler) tryValidatingDisruption(ctx context.Context, nd *nodedisruptionv1alpha1.NodeDisruption) (
	status nodedisruptionv1alpha1.NodeDisruptionStatus, err error) {

	resolver := NodeDisruptionResolver{
		NodeDisruption: nd,
		Client:         r.Client,
		Config:         r.Config,
	}

	any_failed, status, err := resolver.ValidateDisruption(ctx)
	if err != nil {
		return status, err
	}

	var retry_date time.Time
	if nd.Spec.Retry.Enabled && !nd.Spec.Retry.IsAfterDeadline() {
		retry_date = metav1.Now().Add(r.Config.RetryInterval)
	} else {
		retry_date = time.Time{}
	}

	status.NextRetryDate = metav1.NewTime(retry_date)

	if !any_failed {
		status.State = nodedisruptionv1alpha1.Granted
	} else {
		if !status.NextRetryDate.IsZero() {
			status.State = nodedisruptionv1alpha1.Pending
		} else {
			status.State = nodedisruptionv1alpha1.Rejected
		}
	}

	return status, nil
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
	Config         NodeDisruptionReconcilerConfig
}

func (ndr *NodeDisruptionResolver) ValidateDisruption(ctx context.Context) (any_failed bool, status nodedisruptionv1alpha1.NodeDisruptionStatus, err error) {
	disruption, err := ndr.GetDisruption(ctx)
	if err != nil {
		return true, status, err
	}

	status.DisruptedNodes = NodeSetToStringList(disruption.ImpactedNodes)

	if ndr.Config.RejectEmptyNodeDisruption && disruption.ImpactedNodes.Len() == 0 {
		status.DisruptedDisruptionBudgets = []nodedisruptionv1alpha1.DisruptedBudgetStatus{
			{
				Reference: nodedisruptionv1alpha1.NamespacedName{
					Namespace: ndr.NodeDisruption.Namespace,
					Name:      ndr.NodeDisruption.Name,
					Kind:      ndr.NodeDisruption.Kind,
				},
				Reason: "No Node matching selector",
				Ok:     false,
			},
		}
		return true, status, nil
	}

	if ndr.NodeDisruption.Spec.Retry.IsAfterDeadline() {
		status.DisruptedDisruptionBudgets = []nodedisruptionv1alpha1.DisruptedBudgetStatus{
			{
				Reference: nodedisruptionv1alpha1.NamespacedName{
					Namespace: ndr.NodeDisruption.Namespace,
					Name:      ndr.NodeDisruption.Name,
					Kind:      ndr.NodeDisruption.Kind,
				},
				Reason: "Deadline exceeded",
				Ok:     false,
			},
		}

		return true, status, nil
	}

	budgets, err := ndr.GetAllBudgetsInSync(ctx)
	if err != nil {
		return true, status, err
	}

	any_failed, statuses := ndr.DoValidateDisruption(ctx, budgets, disruption)

	status.DisruptedDisruptionBudgets = statuses

	return any_failed, status, nil
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
			ApplicationDisruptionBudget: adb.DeepCopy(),
			Client:                      ndr.Client,
			Resolver:                    resolver.Resolver{Client: ndr.Client},
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
			NodeDisruptionBudget: ndb.DeepCopy(),
			Client:               ndr.Client,
			Resolver:             resolver.Resolver{Client: ndr.Client},
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
func (ndr *NodeDisruptionResolver) DoValidateDisruption(ctx context.Context, budgets []Budget, nd NodeDisruption) (any_failed bool, statuses []nodedisruptionv1alpha1.DisruptedBudgetStatus) {
	logger := log.FromContext(ctx)
	any_failed = false

	impacted_budgets := []Budget{}
	for _, budget := range budgets {
		if !budget.IsImpacted(nd) {
			continue
		}

		if !budget.TolerateDisruption(nd) {
			any_failed = true
			status := nodedisruptionv1alpha1.DisruptedBudgetStatus{
				Reference: budget.GetNamespacedName(),
				Reason:    "No more disruption allowed",
				Ok:        false,
			}
			statuses = append(statuses, status)
			logger.Info("Disruption rejected because: ", "status", status)
			break
		}
		impacted_budgets = append(impacted_budgets, budget)
	}

	if any_failed {
		return any_failed, statuses
	}

	for _, budget := range impacted_budgets {
		err := budget.CheckHealth(ctx)
		if err != nil {
			any_failed = true
			status := nodedisruptionv1alpha1.DisruptedBudgetStatus{
				Reference: budget.GetNamespacedName(),
				Reason:    fmt.Sprintf("Unhealthy: %s", err),
				Ok:        false,
			}
			statuses = append(statuses, status)
			logger.Info("Disruption rejected because: ", "status", status)
			break
		}
	}

	if any_failed {
		return any_failed, statuses
	}

	for _, budget := range impacted_budgets {
		err := budget.CallHealthHook(ctx, *ndr.NodeDisruption)
		if err != nil {
			any_failed = true
			status := nodedisruptionv1alpha1.DisruptedBudgetStatus{
				Reference: budget.GetNamespacedName(),
				Reason:    fmt.Sprintf("Unhealthy: %s", err),
				Ok:        false,
			}
			statuses = append(statuses, status)
			logger.Info("Disruption rejected because: ", "status", status)
			break
		}
		statuses = append(statuses, nodedisruptionv1alpha1.DisruptedBudgetStatus{
			Reference: budget.GetNamespacedName(),
			Reason:    "",
			Ok:        true,
		})
	}

	return any_failed, statuses
}
