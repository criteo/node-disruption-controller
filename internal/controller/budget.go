package controller

import (
	"context"

	nodedisruptionv1alpha1 "github.com/criteo/node-disruption-controller/api/v1alpha1"
	"github.com/criteo/node-disruption-controller/pkg/resolver"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Budget interface {
	// Sync ensure the budget's status is up to date
	Sync(context.Context) error
	// Check if the budget would be impacted by an operation on the provided set of nodes
	IsImpacted(resolver.NodeSet) bool
	// Return the number of disruption allowed considering a list of current node disruptions
	TolerateDisruption(resolver.NodeSet) bool
	// Check health make a synchronous health check on the underlying resource of a budget
	CheckHealth(context.Context) error
	// Call a lifecycle hook in order to synchronously validate a Node Disruption
	CallHealthHook(context.Context, nodedisruptionv1alpha1.NodeDisruption) error
	// Apply the budget's status to Kubernetes
	UpdateStatus(context.Context) error
	// Get the name, namespace and kind of bduget
	GetNamespacedName() nodedisruptionv1alpha1.NamespacedName
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
