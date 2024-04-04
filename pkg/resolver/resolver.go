package resolver

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/golang-collections/collections/set"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Resolver use Kubernetes selectors to return resources
type Resolver struct {
	Client client.Client
}

// NodeSet is a set of (Kubernetes) Node names stored as string
type NodeSet struct {
	Nodes *set.Set
}

func (ns *NodeSet) Intersection(other NodeSet) NodeSet {
	return NodeSet{Nodes: ns.Nodes.Intersection(other.Nodes)}
}

func (ns NodeSet) Len() int {
	return ns.Nodes.Len()
}

func (ns *NodeSet) Union(other NodeSet) NodeSet {
	return NodeSet{Nodes: ns.Nodes.Union(other.Nodes)}
}

func NewNodeSetFromStringList(nodes []string) NodeSet {
	nodeSet := set.New()
	for _, node := range nodes {
		nodeSet.Insert(node)
	}
	return NodeSet{Nodes: nodeSet}
}

func NodeSetToStringList(nodeSet NodeSet) []string {
	// Iterate over the set and append elements to the slice
	nodes := make([]string, 0, nodeSet.Nodes.Len())
	nodeSet.Nodes.Do(func(item interface{}) {
		nodes = append(nodes, item.(string))
	})
	sort.Strings(nodes)
	return nodes
}

// GetNodesFromNamespacedPodSelector find the nodes used by pods matched for a pod selector and in a given namespace
func (r *Resolver) GetNodesFromNamespacedPodSelector(ctx context.Context, podSelector metav1.LabelSelector, namespace string) (NodeSet, error) {
	nodeNames := set.New()
	nodeSet := NodeSet{Nodes: nodeNames}
	selector, err := metav1.LabelSelectorAsSelector(&podSelector)
	if err != nil || selector.Empty() {
		return nodeSet, err
	}
	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{Selector: selector},
	}
	pods := &corev1.PodList{}
	err = r.Client.List(ctx, pods, opts...)
	if err != nil {
		return nodeSet, err
	}

	for _, pod := range pods.Items {
		if pod.Spec.NodeName != "" {
			nodeNames.Insert(pod.Spec.NodeName)
		}
	}
	return nodeSet, nil
}

// NodeLabelSelectorAsRequirement converts a NodeSelectorRequirement to a labels.Requirement
// I have not been able to find a function for that in Kubernetes code, if it exists please replace this
func NodeLabelSelectorAsRequirement(expr *corev1.NodeSelectorRequirement) (*labels.Requirement, error) {
	var op selection.Operator
	switch expr.Operator {
	case corev1.NodeSelectorOpIn:
		op = selection.In
	case corev1.NodeSelectorOpNotIn:
		op = selection.NotIn
	case corev1.NodeSelectorOpExists:
		op = selection.Exists
	case corev1.NodeSelectorOpDoesNotExist:
		op = selection.DoesNotExist
	default:
		return nil, fmt.Errorf("%q is not a valid label selector operator", expr.Operator)
	}
	return labels.NewRequirement(expr.Key, op, append([]string(nil), expr.Values...))
}

// NodeSelectorAsSelector converts a NodeSelector to a label selector and field selector
// I have not been able to find a function for that in Kubernetes code, if it exists please replace this
func NodeSelectorAsSelector(nodeSelector *corev1.NodeSelector) (labels.Selector, fields.Selector, error) {
	if nodeSelector == nil {
		return labels.Nothing(), fields.Nothing(), nil
	}

	if len(nodeSelector.NodeSelectorTerms) == 0 {
		return labels.Everything(), fields.Everything(), nil
	}

	labelsRequirements := make([]labels.Requirement, 0, len(nodeSelector.NodeSelectorTerms))
	fieldsRequirements := make([]string, 0, len(nodeSelector.NodeSelectorTerms))

	for _, term := range nodeSelector.NodeSelectorTerms {
		for _, expr := range term.MatchExpressions {
			r, err := NodeLabelSelectorAsRequirement(&expr)
			if err != nil {
				return nil, nil, err
			}
			labelsRequirements = append(labelsRequirements, *r)
		}

		for _, expr := range term.MatchFields {
			r, err := NodeLabelSelectorAsRequirement(&expr)
			if err != nil {
				return nil, nil, err
			}
			fieldsRequirements = append(fieldsRequirements, r.String())
		}
	}

	labelSelector := labels.NewSelector()
	labelSelector = labelSelector.Add(labelsRequirements...)
	fieldSelector, err := fields.ParseSelector(strings.Join(fieldsRequirements, ","))
	return labelSelector, fieldSelector, err
}

func (r *Resolver) GetNodesFromNamespacedPVCSelector(ctx context.Context, pvcSelector metav1.LabelSelector, namespace string) (NodeSet, error) {
	logger := log.FromContext(ctx)

	nodeNames := set.New()
	nodeSet := NodeSet{Nodes: nodeNames}

	selector, err := metav1.LabelSelectorAsSelector(&pvcSelector)
	if err != nil {
		return nodeSet, err
	}
	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{Selector: selector},
	}
	PVCs := &corev1.PersistentVolumeClaimList{}
	err = r.Client.List(ctx, PVCs, opts...)
	if err != nil {
		return nodeSet, err
	}

	PVsToFetch := []string{}

	for _, pvc := range PVCs.Items {
		PVsToFetch = append(PVsToFetch, pvc.Spec.VolumeName)
	}

	getOptions := []client.GetOption{}
	for _, PVName := range PVsToFetch {
		pv := &corev1.PersistentVolume{}

		err = r.Client.Get(ctx, types.NamespacedName{Name: PVName, Namespace: ""}, pv, getOptions...)
		if err != nil {
			return nodeSet, err
		}

		if pv.Spec.NodeAffinity == nil || pv.Spec.NodeAffinity.Required == nil {
			logger.Info("Skipping PV because NodeAffinity or Required is not defined", "pv_name", pv.Name)
			continue
		}

		nodeSelector := pv.Spec.NodeAffinity.Required

		opts := []client.ListOption{}
		labelSelector, fieldSelector, err := NodeSelectorAsSelector(nodeSelector)
		if err != nil {
			return nodeSet, err
		}

		if labelSelector.Empty() && fieldSelector.Empty() {
			// Ignore this PV
			logger.Info("Skipping PV because there are no selector defined", "pv_name", pv.Name)
			continue
		}

		if !labelSelector.Empty() {
			opts = append(opts, client.MatchingLabelsSelector{Selector: labelSelector})
		}

		if !fieldSelector.Empty() {
			opts = append(opts, client.MatchingFieldsSelector{Selector: fieldSelector})
		}

		nodes := &corev1.NodeList{}
		err = r.Client.List(ctx, nodes, opts...)
		if err != nil {
			return nodeSet, err
		}

		for _, node := range nodes.Items {
			nodeNames.Insert(node.Name)
		}
	}

	return nodeSet, nil
}

func (r *Resolver) GetNodeFromNodeSelector(ctx context.Context, nodeSelector metav1.LabelSelector) (NodeSet, error) {
	nodeNames := set.New()
	nodeSet := NodeSet{Nodes: nodeNames}

	selector, err := metav1.LabelSelectorAsSelector(&nodeSelector)
	if err != nil || selector.Empty() {
		return nodeSet, err
	}
	opts := []client.ListOption{
		client.MatchingLabelsSelector{Selector: selector},
	}
	nodes := &corev1.NodeList{}
	err = r.Client.List(ctx, nodes, opts...)
	if err != nil {
		return nodeSet, err
	}

	for _, node := range nodes.Items {
		nodeNames.Insert(node.ObjectMeta.Name)
	}
	return nodeSet, nil
}
