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
	"reflect"
	"time"

	nodedisruptionv1alpha1 "github.com/criteo/node-disruption-controller/api/v1alpha1"
	"github.com/criteo/node-disruption-controller/pkg/resolver"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	DefaultRetryInterval = time.Minute
)

type NodeDisruptionReconcilerConfig struct {
	// Whether to grant or reject a node disruption matching no node
	RejectEmptyNodeDisruption bool
	// How long to retry between each validation attempt
	RetryInterval time.Duration
	// Reject NodeDisruption if its node selector overlaps an older NodeDisruption's selector
	RejectOverlappingDisruption bool
}

// NodeDisruptionReconciler reconciles NodeDisruptions
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

	clusterResult := ctrl.Result{}

	nd := &nodedisruptionv1alpha1.NodeDisruption{}
	err := r.Client.Get(ctx, req.NamespacedName, nd)

	if err != nil {
		if errors.IsNotFound(err) {
			PruneNodeDisruptionMetrics(req.NamespacedName.Name)
			// If the ressource was not found, nothing has to be done
			return clusterResult, nil
		}
		return clusterResult, err
	}
	logger.Info("Updating metrics")
	UpdateNodeDisruptionMetrics(nd)

	logger.Info("Start reconcile of NodeDisruption", "state", nd.Status.State, "retryDate", nd.Status.NextRetryDate.Time)
	if time.Now().Before(nd.Status.NextRetryDate.Time) {
		logger.Info("Time not elapsed, retry later", "currentDate", time.Now(), "retryDate", nd.Status.NextRetryDate.Time)
		clusterResult.RequeueAfter = time.Until(nd.Status.NextRetryDate.Time)
		return clusterResult, nil
	}

	reconciler := SingleNodeDisruptionReconciler{
		NodeDisruption: *nd.DeepCopy(),
		Client:         r.Client,
		Resolver:       resolver.Resolver{Client: r.Client},
		Config:         r.Config,
	}

	err = reconciler.Reconcile(ctx)
	if err != nil {
		return clusterResult, nil
	}

	if !reflect.DeepEqual(nd.Status, reconciler.NodeDisruption.Status) {
		logger.Info("Updating Status, done with", "state", reconciler.NodeDisruption.Status.State)
		return clusterResult, reconciler.UpdateStatus(ctx)
	}
	logger.Info("Reconciliation successful", "state", reconciler.NodeDisruption.Status.State)
	return clusterResult, nil
}

// PruneNodeDisruptionMetric remove metrics for a Node Disruption that don't exist anymore
func PruneNodeDisruptionMetrics(nd_name string) {
	NodeDisruptionState.DeletePartialMatch(prometheus.Labels{"node_disruption_name": nd_name})
	NodeDisruptionCreated.DeletePartialMatch(prometheus.Labels{"node_disruption_name": nd_name})
	NodeDisruptionDeadline.DeletePartialMatch(prometheus.Labels{"node_disruption_name": nd_name})
	NodeDisruptionImpactedNodes.DeletePartialMatch(prometheus.Labels{"node_disruption_name": nd_name})
}

// UpdateNodeDisruptionMetrics update metrics for a Node Disruption
func UpdateNodeDisruptionMetrics(nd *nodedisruptionv1alpha1.NodeDisruption) {
	nd_state := 0
	if nd.Status.State == nodedisruptionv1alpha1.Pending {
		nd_state = 0
	} else if nd.Status.State == nodedisruptionv1alpha1.Rejected {
		nd_state = -1
	} else if nd.Status.State == nodedisruptionv1alpha1.Granted {
		nd_state = 1
	}
	NodeDisruptionState.WithLabelValues(nd.Name).Set(float64(nd_state))
	NodeDisruptionCreated.WithLabelValues(nd.Name).Set(float64(nd.CreationTimestamp.Unix()))
	// Deadline might not be set so it will be 0 but timestamp in Go are not Unix epoch
	// so converting a 0 timestamp will not result in epoch 0. We override this to have nice values
	deadline := nd.Spec.Retry.Deadline.Unix()
	if nd.Spec.Retry.Deadline.IsZero() {
		deadline = 0
	}
	NodeDisruptionDeadline.WithLabelValues(nd.Name).Set(float64(deadline))

	for _, node_name := range nd.Status.DisruptedNodes {
		NodeDisruptionImpactedNodes.WithLabelValues(nd.Name, node_name).Set(1)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeDisruptionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("node-disruption-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&nodedisruptionv1alpha1.NodeDisruption{}).
		Complete(r)
}

// Reconcile a single NodeDisruption
type SingleNodeDisruptionReconciler struct {
	NodeDisruption nodedisruptionv1alpha1.NodeDisruption
	Client         client.Client
	Resolver       resolver.Resolver
	Config         NodeDisruptionReconcilerConfig
}

// Perform reconciliation of a NodeDisruption
func (ndr *SingleNodeDisruptionReconciler) Reconcile(ctx context.Context) (err error) {
	err = ndr.Sync(ctx)
	if err != nil {
		return err
	}

	return ndr.TryTransitionState(ctx)
}

// TryTransitionState attempt to move the state of the disruption toward a final state
func (ndr *SingleNodeDisruptionReconciler) TryTransitionState(ctx context.Context) (err error) {
	logger := log.FromContext(ctx)
	// If the state is unknown, switch it to Pending
	if ndr.NodeDisruption.Status.State == "" {
		ndr.NodeDisruption.Status.State = nodedisruptionv1alpha1.Pending

		return err
	}

	if ndr.NodeDisruption.Status.State == nodedisruptionv1alpha1.Pending {
		logger.Info("Trying to validate the node disruption")
		err := ndr.tryTransitionToGranted(ctx)
		if err != nil {
			return err
		}
		if ndr.NodeDisruption.Status.State == nodedisruptionv1alpha1.Granted {
			NodeDisruptionGrantedTotal.WithLabelValues().Inc()
		} else if ndr.NodeDisruption.Status.State == nodedisruptionv1alpha1.Rejected {
			NodeDisruptionRejectedTotal.WithLabelValues().Inc()
		}
	}
	// If the disruption is not Pending nor unknown, the state is final
	return nil
}

func (ndr *SingleNodeDisruptionReconciler) getRetryDate() metav1.Time {
	var retryDate time.Time
	if ndr.NodeDisruption.Spec.Retry.Enabled && !ndr.NodeDisruption.Spec.Retry.IsAfterDeadline() {
		retryDate = metav1.Now().Add(ndr.Config.RetryInterval)
	} else {
		retryDate = time.Time{}
	}

	return metav1.NewTime(retryDate)
}

// tryTransitionToGranted attempt to transition to the granted state but can result in either pending or rejected
func (ndr *SingleNodeDisruptionReconciler) tryTransitionToGranted(ctx context.Context) (err error) {
	nextRetryDate := ndr.getRetryDate()

	var state nodedisruptionv1alpha1.NodeDisruptionState

	budgets, err := GetAllBudgetsInSync(ctx, ndr.Client)
	if err != nil {
		return err
	}

	anyFailed, statuses, err := ndr.ValidateWithInternalConstraints(ctx)
	if err != nil {
		return err
	}

	if !anyFailed {
		anyFailed, statuses = ndr.ValidateWithBudgetConstraints(ctx, budgets)
	}

	if !anyFailed {
		state = nodedisruptionv1alpha1.Granted
	} else {
		if !nextRetryDate.IsZero() {
			state = nodedisruptionv1alpha1.Pending
		} else {
			state = nodedisruptionv1alpha1.Rejected
		}
	}

	ndr.NodeDisruption.Status.DisruptedDisruptionBudgets = statuses
	ndr.NodeDisruption.Status.State = state
	ndr.NodeDisruption.Status.NextRetryDate = nextRetryDate

	return nil
}

// Sync brings the status of the NodeDisruption up to date
func (ndr *SingleNodeDisruptionReconciler) Sync(ctx context.Context) error {
	disruptedNodes, err := ndr.Resolver.GetNodeFromNodeSelector(ctx, ndr.NodeDisruption.Spec.NodeSelector)
	if err != nil {
		return err
	}
	ndr.NodeDisruption.Status.DisruptedNodes = resolver.NodeSetToStringList(disruptedNodes)
	return nil
}

// UpdateStatus update the Status of the NodeDisruption in Kubernetes
func (ndr *SingleNodeDisruptionReconciler) UpdateStatus(ctx context.Context) error {
	return ndr.Client.Status().Update(ctx, &ndr.NodeDisruption, []client.SubResourceUpdateOption{}...)
}

func (ndr *SingleNodeDisruptionReconciler) generateRejectedStatus(reason string) []nodedisruptionv1alpha1.DisruptedBudgetStatus {
	return []nodedisruptionv1alpha1.DisruptedBudgetStatus{
		{
			Reference: nodedisruptionv1alpha1.NamespacedName{
				Namespace: ndr.NodeDisruption.Namespace,
				Name:      ndr.NodeDisruption.Name,
				Kind:      ndr.NodeDisruption.Kind,
			},
			Reason: reason,
			Ok:     false,
		},
	}
}

// ValidateOverlappingDisruption checks that the current disruption doesn't overlap and existing one
func (ndr *SingleNodeDisruptionReconciler) ValidateOverlappingDisruption(ctx context.Context, disruptedNodes resolver.NodeSet) (anyFailed bool, statuses []nodedisruptionv1alpha1.DisruptedBudgetStatus, err error) {
	allDisruptions := &nodedisruptionv1alpha1.NodeDisruptionList{}
	err = ndr.Client.List(ctx, allDisruptions)
	if err != nil {
		return false, nil, err
	}
	for _, otherDisruption := range allDisruptions.Items {
		if otherDisruption.Name == ndr.NodeDisruption.Name {
			continue
		}
		if otherDisruption.Status.State == nodedisruptionv1alpha1.Pending || otherDisruption.Status.State == nodedisruptionv1alpha1.Granted {
			otherDisruptedNodes := resolver.NewNodeSetFromStringList(otherDisruption.Status.DisruptedNodes)
			if otherDisruptedNodes.Intersection(disruptedNodes).Len() > 0 {
				return true, ndr.generateRejectedStatus(fmt.Sprintf(`Selected node(s) overlap with another disruption: ”%s"`, otherDisruption.Name)), nil
			}
		}
	}

	return false, nil, nil
}

// ValidateInternalConstraints check that the Node Disruption is valid against internal constraints
// such as deadline or constraints on number of impacted nodes
func (ndr *SingleNodeDisruptionReconciler) ValidateWithInternalConstraints(ctx context.Context) (anyFailed bool, statuses []nodedisruptionv1alpha1.DisruptedBudgetStatus, err error) {
	disruptedNodes := resolver.NewNodeSetFromStringList(ndr.NodeDisruption.Status.DisruptedNodes)

	if ndr.Config.RejectEmptyNodeDisruption && disruptedNodes.Len() == 0 {
		return true, ndr.generateRejectedStatus("No Node matching selector"), nil
	}

	if ndr.Config.RejectOverlappingDisruption {
		anyFailed, statuses, err = ndr.ValidateOverlappingDisruption(ctx, disruptedNodes)
		if err != nil {
			return false, nil, err
		} else if anyFailed {
			return true, statuses, nil
		}
	}

	if ndr.NodeDisruption.Spec.Retry.IsAfterDeadline() {
		return true, ndr.generateRejectedStatus("Deadline exceeded"), nil
	}

	return false, statuses, nil
}

// ValidateBudgetConstraints check that the Node Disruption is valid against the budgets defined in the cluster
func (ndr *SingleNodeDisruptionReconciler) ValidateWithBudgetConstraints(ctx context.Context, budgets []Budget) (anyFailed bool, statuses []nodedisruptionv1alpha1.DisruptedBudgetStatus) {
	disruptedNodes := resolver.NewNodeSetFromStringList(ndr.NodeDisruption.Status.DisruptedNodes)
	logger := log.FromContext(ctx)
	anyFailed = false

	impactedBudgets := []Budget{}
	for _, budget := range budgets {
		if !budget.IsImpacted(disruptedNodes) {
			continue
		}

		if !budget.TolerateDisruption(disruptedNodes) {
			anyFailed = true
			ref := budget.GetNamespacedName()
			status := nodedisruptionv1alpha1.DisruptedBudgetStatus{
				Reference: ref,
				Reason:    "No more disruption allowed",
				Ok:        false,
			}
			statuses = append(statuses, status)
			logger.Info("Disruption rejected because: ", "status", status)
			DisruptionBudgetRejectedTotal.WithLabelValues(ref.Namespace, ref.Name, ref.Kind).Inc()
			break
		}
		impactedBudgets = append(impactedBudgets, budget)
	}

	if anyFailed {
		return anyFailed, statuses
	}

	for _, budget := range impactedBudgets {
		err := budget.CallHealthHook(ctx, ndr.NodeDisruption)
		ref := budget.GetNamespacedName()
		if err != nil {
			anyFailed = true
			status := nodedisruptionv1alpha1.DisruptedBudgetStatus{
				Reference: ref,
				Reason:    fmt.Sprintf("Unhealthy: %s", err),
				Ok:        false,
			}
			statuses = append(statuses, status)
			logger.Info("Disruption rejected because: ", "status", status)
			DisruptionBudgetRejectedTotal.WithLabelValues(ref.Namespace, ref.Name, ref.Kind).Inc()
			break
		}
		DisruptionBudgetGrantedTotal.WithLabelValues(ref.Namespace, ref.Name, ref.Kind).Inc()
		statuses = append(statuses, nodedisruptionv1alpha1.DisruptedBudgetStatus{
			Reference: budget.GetNamespacedName(),
			Reason:    "",
			Ok:        true,
		})
	}

	return anyFailed, statuses
}
