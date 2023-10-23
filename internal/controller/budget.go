package controller

import (
	"context"
	"sort"

	nodedisruptionv1alpha1 "github.com/criteo/node-disruption-controller/api/v1alpha1"
	"github.com/golang-collections/collections/set"
)

func NewNodeSetFromStringList(nodes []string) *set.Set {
	nodeSet := set.New()
	for _, node := range nodes {
		nodeSet.Insert(node)
	}
	return nodeSet
}

func NodeSetToStringList(nodeSet *set.Set) []string {
	// Iterate over the set and append elements to the slice
	nodes := make([]string, 0, nodeSet.Len())
	nodeSet.Do(func(item interface{}) {
		nodes = append(nodes, item.(string))
	})
	sort.Strings(nodes)
	return nodes
}

type NodeDisruption struct {
	ImpactedNodes *set.Set
}

type Budget interface {
	// Sync ensure the budget's status is up to date
	Sync(context.Context) error
	// Check if the budget would be impacted by an operation on the provided set of nodes
	IsImpacted(NodeDisruption) bool
	// Return the number of disruption allowed considering a list of current node disruptions
	TolerateDisruption(NodeDisruption) bool
	// Check health make a synchronous health check on the underlying ressource of a budget
	CheckHealth(context.Context) error
	// Call a lifecycle hook in order to synchronously validate a Node Disruption
	CallHealthHook(context.Context, nodedisruptionv1alpha1.NodeDisruption) error
	// Apply the budget's status to Kubernetes
	UpdateStatus(context.Context) error
	// Get the name, namespace and kind of bduget
	GetNamespacedName() nodedisruptionv1alpha1.NamespacedName
}
