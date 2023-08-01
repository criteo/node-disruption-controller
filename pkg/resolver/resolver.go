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
func (r *Resolver) GetNodesFromNamespacedPodSelector(ctx context.Context, pod_selector metav1.LabelSelector, namespace string) (*set.Set, error) {
	node_names := set.New()
	selector, err := metav1.LabelSelectorAsSelector(&pod_selector)
	if err != nil || selector.Empty() {
		return node_names, err
	}
	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{Selector: selector},
	}
	pods := &corev1.PodList{}
	err = r.Client.List(ctx, pods, opts...)
	if err != nil {
		return node_names, err
	}

	for _, pod := range pods.Items {
		node_names.Insert(pod.Spec.NodeName)
	}
	return node_names, nil
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
func NodeSelectorAsSelector(node_selector *corev1.NodeSelector) (labels.Selector, fields.Selector, error) {
	if node_selector == nil {
		return labels.Nothing(), fields.Nothing(), nil
	}

	if len(node_selector.NodeSelectorTerms) == 0 {
		return labels.Everything(), fields.Everything(), nil
	}

	labels_requirements := make([]labels.Requirement, 0, len(node_selector.NodeSelectorTerms))
	fields_requirements := make([]string, 0, len(node_selector.NodeSelectorTerms))

	for _, term := range node_selector.NodeSelectorTerms {
		for _, expr := range term.MatchExpressions {
			r, err := NodeLabelSelectorAsRequirement(&expr)
			if err != nil {
				return nil, nil, err
			}
			labels_requirements = append(labels_requirements, *r)
		}

		for _, expr := range term.MatchFields {
			r, err := NodeLabelSelectorAsRequirement(&expr)
			if err != nil {
				return nil, nil, err
			}
			fields_requirements = append(fields_requirements, r.String())
		}
	}

	label_selector := labels.NewSelector()
	label_selector = label_selector.Add(labels_requirements...)
	field_selector, err := fields.ParseSelector(strings.Join(fields_requirements, ","))
	return label_selector, field_selector, err
}

func (r *Resolver) GetNodesFromNamespacedPVCSelector(ctx context.Context, pvc_selector metav1.LabelSelector, namespace string) (*set.Set, error) {
	node_names := set.New()
	selector, err := metav1.LabelSelectorAsSelector(&pvc_selector)
	if err != nil {
		return node_names, err
	}
	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{Selector: selector},
	}
	PVCs := &corev1.PersistentVolumeClaimList{}
	err = r.Client.List(ctx, PVCs, opts...)
	if err != nil {
		return node_names, err
	}

	pvs_to_fetch := []string{}

	for _, pvc := range PVCs.Items {
		pvs_to_fetch = append(pvs_to_fetch, pvc.Spec.VolumeName)
	}

	get_options := []client.GetOption{}
	for _, pv_name := range pvs_to_fetch {
		pv := &corev1.PersistentVolume{}

		err = r.Client.Get(ctx, types.NamespacedName{Name: pv_name, Namespace: ""}, pv, get_options...)
		if err != nil {
			return node_names, err
		}

		node_selector := pv.Spec.NodeAffinity.Required
		if node_selector == nil {
			continue
		}

		opts := []client.ListOption{}
		label_selector, field_selector, err := NodeSelectorAsSelector(node_selector)
		if err != nil {
			return node_names, err
		}

		if label_selector.Empty() && field_selector.Empty() {
			// Ignore this PV
			fmt.Printf("skipping %s, no affinity", pv_name)
			continue
		}

		if !label_selector.Empty() {
			opts = append(opts, client.MatchingLabelsSelector{Selector: label_selector})
		}

		if !field_selector.Empty() {
			opts = append(opts, client.MatchingFieldsSelector{Selector: field_selector})
		}

		nodes := &corev1.NodeList{}
		err = r.Client.List(ctx, nodes, opts...)
		if err != nil {
			return node_names, err
		}

		for _, node := range nodes.Items {
			node_names.Insert(node.Name)
		}
	}

	return node_names, nil
}

func (r *Resolver) GetNodeFromNodeSelector(ctx context.Context, node_selector metav1.LabelSelector) (*set.Set, error) {
	node_names := set.New()
	selector, err := metav1.LabelSelectorAsSelector(&node_selector)
	if err != nil || selector.Empty() {
		return node_names, err
	}
	opts := []client.ListOption{
		client.MatchingLabelsSelector{Selector: selector},
	}
	nodes := &corev1.NodeList{}
	err = r.Client.List(ctx, nodes, opts...)
	if err != nil {
		return node_names, err
	}

	for _, node := range nodes.Items {
		node_names.Insert(node.ObjectMeta.Name)
	}
	return node_names, nil
}
