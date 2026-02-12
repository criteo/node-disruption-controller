package controller

import (
	"context"
	"time"

	nodedisruptionv1alpha1 "github.com/criteo/node-disruption-controller/api/v1alpha1"
	"github.com/criteo/node-disruption-controller/pkg/resolver"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Budget interface {
	// Sync ensure the budget's status is up to date
	Sync(context.Context) error
	// Check if the budget would be impacted by an operation on the provided set of nodes
	IsImpacted(resolver.NodeSet) bool
	// Return the number of disruption allowed considering a list of current node disruptions
	TolerateDisruption(resolver.NodeSet) bool
	// Return true if the budget has v2 hooks configured (for prepare, ready, cancel)
	V2HooksReady() bool
	// Call the prepare hook to trigger the preparation of the application for disruption
	CallPrepareHook(context.Context, nodedisruptionv1alpha1.NodeDisruption, time.Duration) error
	// Call the ready hook to validate that the application is ready for disruption
	CallReadyHook(context.Context, nodedisruptionv1alpha1.NodeDisruption, time.Duration) error
	// Call the cancel hook to cancel any preparation for disruption
	CallCancelHook(context.Context, nodedisruptionv1alpha1.NodeDisruption, time.Duration) error
	// Call a lifecycle hook in order to synchronously validate a Node Disruption
	CallHealthHook(context.Context, nodedisruptionv1alpha1.NodeDisruption, time.Duration) error
	// Apply the budget's status to Kubernetes
	UpdateStatus(context.Context) error
	// Get the name, namespace and kind of budget
	GetNamespacedName() nodedisruptionv1alpha1.NamespacedName
	// Get the list of supported node disruption types for this budget
	GetSupportedDisruptionTypes() []string
}

// PruneBudgetMetrics remove metrics for a Disruption Budget that doesn't exist anymore
func PruneBudgetStatusMetrics(ref nodedisruptionv1alpha1.NamespacedName) {
	DisruptionBudgetDisruptions.DeletePartialMatch(prometheus.Labels{"disruption_budget_namespace": ref.Namespace, "disruption_budget_name": ref.Name, "disruption_budget_kind": ref.Kind})
	DisruptionBudgetWatchedNodes.DeletePartialMatch(prometheus.Labels{"disruption_budget_namespace": ref.Namespace, "disruption_budget_name": ref.Name, "disruption_budget_kind": ref.Kind})
	DisruptionBudgetDisruptionsAllowed.DeletePartialMatch(prometheus.Labels{"disruption_budget_namespace": ref.Namespace, "disruption_budget_name": ref.Name, "disruption_budget_kind": ref.Kind})
	DisruptionBudgetCurrentDisruptions.DeletePartialMatch(prometheus.Labels{"disruption_budget_namespace": ref.Namespace, "disruption_budget_name": ref.Name, "disruption_budget_kind": ref.Kind})

	DisruptionBudgetRejectedTotal.DeletePartialMatch(prometheus.Labels{"disruption_budget_namespace": ref.Namespace, "disruption_budget_name": ref.Name, "disruption_budget_kind": ref.Kind})
	DisruptionBudgetGrantedTotal.DeletePartialMatch(prometheus.Labels{"disruption_budget_namespace": ref.Namespace, "disruption_budget_name": ref.Name, "disruption_budget_kind": ref.Kind})
	DisruptionBudgetCheckHealthHookStatusCodeTotal.DeletePartialMatch(prometheus.Labels{"disruption_budget_namespace": ref.Namespace, "disruption_budget_name": ref.Name, "disruption_budget_kind": ref.Kind})
	DisruptionBudgetCheckHealthHookErrorTotal.DeletePartialMatch(prometheus.Labels{"disruption_budget_namespace": ref.Namespace, "disruption_budget_name": ref.Name, "disruption_budget_kind": ref.Kind})
}

func UpdateBudgetStatusMetrics(ref nodedisruptionv1alpha1.NamespacedName, status nodedisruptionv1alpha1.DisruptionBudgetStatus) {
	// delete before updating to avoid leaking metrics/nodes over time
	DisruptionBudgetWatchedNodes.DeletePartialMatch(prometheus.Labels{"disruption_budget_namespace": ref.Namespace, "disruption_budget_name": ref.Name, "disruption_budget_kind": ref.Kind})
	for _, node_name := range status.WatchedNodes {
		DisruptionBudgetWatchedNodes.WithLabelValues(ref.Namespace, ref.Name, ref.Kind, node_name).Set(1)
	}
	// delete before updating to avoid leaking metrics/disruptions over timex
	DisruptionBudgetDisruptions.DeletePartialMatch(prometheus.Labels{"disruption_budget_namespace": ref.Namespace, "disruption_budget_name": ref.Name, "disruption_budget_kind": ref.Kind})
	for _, disruption := range status.Disruptions {
		nd_state := 0
		state := nodedisruptionv1alpha1.NodeDisruptionState(disruption.State)
		switch state {
		case nodedisruptionv1alpha1.Pending:
			nd_state = 0
		case nodedisruptionv1alpha1.Rejected:
			nd_state = -1
		case nodedisruptionv1alpha1.Granted:
			nd_state = 1
		}
		DisruptionBudgetDisruptions.WithLabelValues(ref.Namespace, ref.Name, ref.Kind, disruption.Name).Set(float64(nd_state))
	}
	DisruptionBudgetDisruptionsAllowed.WithLabelValues(ref.Namespace, ref.Name, ref.Kind).Set(float64(status.DisruptionsAllowed))
	DisruptionBudgetCurrentDisruptions.WithLabelValues(ref.Namespace, ref.Name, ref.Kind).Set(float64(status.CurrentDisruptions))
}

// GetAllBudgetsInSync fetch all the budgets from Kubernetes and synchronise them
func GetAllBudgetsInSync(ctx context.Context, k8sClient client.Client) ([]Budget, error) {
	opts := []client.ListOption{}
	budgets := []Budget{}

	applicationDisruptionBudgets := &nodedisruptionv1alpha1.ApplicationDisruptionBudgetList{}
	err := k8sClient.List(ctx, applicationDisruptionBudgets, opts...)
	if err != nil {
		return budgets, err
	}
	for _, adb := range applicationDisruptionBudgets.Items {
		adbResolver := ApplicationDisruptionBudgetResolver{
			ApplicationDisruptionBudget: adb.DeepCopy(),
			Client:                      k8sClient,
			Resolver:                    resolver.Resolver{Client: k8sClient},
		}
		budgets = append(budgets, &adbResolver)
	}

	nodeDisruptionBudget := &nodedisruptionv1alpha1.NodeDisruptionBudgetList{}
	err = k8sClient.List(ctx, nodeDisruptionBudget, opts...)
	if err != nil {
		return budgets, err
	}
	for _, ndb := range nodeDisruptionBudget.Items {
		ndbResolver := NodeDisruptionBudgetResolver{
			NodeDisruptionBudget: ndb.DeepCopy(),
			Client:               k8sClient,
			Resolver:             resolver.Resolver{Client: k8sClient},
		}
		budgets = append(budgets, &ndbResolver)
	}

	for _, budget := range budgets {
		err := budget.Sync(ctx)
		if err != nil {
			return budgets, err
		}
	}

	return budgets, nil
}
