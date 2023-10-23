package resolver

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang-collections/collections/set"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Resolver use Kubernetes selectors to return ressources
type Resolver struct {
	Client client.Client
}

// GetNodesFromNamespacedPodSelector find the nodes used by pods matched for a pod selector and in a given namespace
func (r *Resolver) GetNodesFromNamespacedPodSelector(ctx context.Context, podSelector metav1.LabelSelector, namespace string) (*set.Set, error) {
	nodeNames := set.New()
	selector, err := metav1.LabelSelectorAsSelector(&podSelector)
	if err != nil || selector.Empty() {
		return nodeNames, err
	}
	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{Selector: selector},
	}
	pods := &corev1.PodList{}
	err = r.Client.List(ctx, pods, opts...)
	if err != nil {
		return nodeNames, err
	}

	for _, pod := range pods.Items {
		nodeNames.Insert(pod.Spec.NodeName)
	}
	return nodeNames, nil
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

func (r *Resolver) GetNodesFromNamespacedPVCSelector(ctx context.Context, pvcSelector metav1.LabelSelector, namespace string) (*set.Set, error) {
	nodeNames := set.New()
	selector, err := metav1.LabelSelectorAsSelector(&pvcSelector)
	if err != nil {
		return nodeNames, err
	}
	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{Selector: selector},
	}
	PVCs := &corev1.PersistentVolumeClaimList{}
	err = r.Client.List(ctx, PVCs, opts...)
	if err != nil {
		return nodeNames, err
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
			return nodeNames, err
		}

		nodeSelector := pv.Spec.NodeAffinity.Required
		if nodeSelector == nil {
			continue
		}

		opts := []client.ListOption{}
		labelSelector, fieldSelector, err := NodeSelectorAsSelector(nodeSelector)
		if err != nil {
			return nodeNames, err
		}

		if labelSelector.Empty() && fieldSelector.Empty() {
			// Ignore this PV
			fmt.Printf("skipping %s, no affinity", PVName)
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
			return nodeNames, err
		}

		for _, node := range nodes.Items {
			nodeNames.Insert(node.Name)
		}
	}

	return nodeNames, nil
}

func (r *Resolver) GetNodeFromNodeSelector(ctx context.Context, nodeSelector metav1.LabelSelector) (*set.Set, error) {
	nodeNames := set.New()
	selector, err := metav1.LabelSelectorAsSelector(&nodeSelector)
	if err != nil || selector.Empty() {
		return nodeNames, err
	}
	opts := []client.ListOption{
		client.MatchingLabelsSelector{Selector: selector},
	}
	nodes := &corev1.NodeList{}
	err = r.Client.List(ctx, nodes, opts...)
	if err != nil {
		return nodeNames, err
	}

	for _, node := range nodes.Items {
		nodeNames.Insert(node.ObjectMeta.Name)
	}
	return nodeNames, nil
}
