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
	// Check health make a synchronous health check on the underlying ressource of a budget
	CheckHealth(context.Context) error
	// Call a lifecycle hook in order to synchronously validate a Node Disruption
	CallHealthHook(context.Context, nodedisruptionv1alpha1.NodeDisruption) error
	// Apply the budget's status to Kubernetes
	UpdateStatus(context.Context) error
	// Get the name, namespace and kind of bduget
	GetNamespacedName() nodedisruptionv1alpha1.NamespacedName
}
