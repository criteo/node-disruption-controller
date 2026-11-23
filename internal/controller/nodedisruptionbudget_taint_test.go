package controller

import (
	"context"
	"testing"

	nodedisruptionv1alpha1 "github.com/criteo/node-disruption-controller/api/v1alpha1"
	"github.com/criteo/node-disruption-controller/pkg/resolver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	taintTestPoolLabels = map[string]string{"pool": "a"}
	ciliumTaint         = corev1.Taint{Key: "node.cilium.io/agent-not-ready", Effect: corev1.TaintEffectNoSchedule}
	dedicatedTaint      = corev1.Taint{Key: "crto.in/dedicated", Value: "team", Effect: corev1.TaintEffectNoSchedule}
	hardTaintsSelector  = &nodedisruptionv1alpha1.TaintSelector{
		Include: []corev1.Toleration{
			{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
			{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoExecute},
		},
		Exclude: []corev1.Toleration{
			{Key: "crto.in/dedicated", Operator: corev1.TolerationOpExists},
		},
	}
)

func taintTestNode(name string, taints ...corev1.Taint) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{"pool": "a", "kubernetes.io/hostname": name}},
		Spec:       corev1.NodeSpec{Taints: taints},
	}
}

func taintTestND(name string, state nodedisruptionv1alpha1.NodeDisruptionState, node string) *nodedisruptionv1alpha1.NodeDisruption {
	return &nodedisruptionv1alpha1.NodeDisruption{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: nodedisruptionv1alpha1.NodeDisruptionSpec{
			NodeSelector: metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/hostname": node}},
		},
		Status: nodedisruptionv1alpha1.NodeDisruptionStatus{State: state},
	}
}

func syncedTaintTestNDB(t *testing.T, maxDisrupted int, taintSelector *nodedisruptionv1alpha1.TaintSelector, objects ...client.Object) *NodeDisruptionBudgetResolver {
	s := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(s))
	require.NoError(t, nodedisruptionv1alpha1.AddToScheme(s))

	c := fake.NewClientBuilder().WithScheme(s).WithObjects(objects...).Build()
	ndb := &nodedisruptionv1alpha1.NodeDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Name: "ndb"},
		Spec: nodedisruptionv1alpha1.NodeDisruptionBudgetSpec{
			NodeSelector:      metav1.LabelSelector{MatchLabels: taintTestPoolLabels},
			MaxDisruptedNodes: maxDisrupted,
			TaintSelector:     taintSelector,
		},
	}
	r := &NodeDisruptionBudgetResolver{
		NodeDisruptionBudget: ndb,
		Client:               c,
		Resolver:             resolver.Resolver{Client: c},
	}
	require.NoError(t, r.Sync(context.Background()))
	return r
}

func TestNDBTaintSelectorAbsentIgnoresTaints(t *testing.T) {
	r := syncedTaintTestNDB(t, 1, nil,
		taintTestNode("node1", ciliumTaint),
		taintTestNode("node2"),
	)

	assert.Nil(t, r.NodeDisruptionBudget.Status.TaintedNodes)
	assert.Equal(t, 0, r.NodeDisruptionBudget.Status.CurrentDisruptions)
	assert.Equal(t, 1, r.NodeDisruptionBudget.Status.DisruptionsAllowed)
}

func TestNDBTaintedNodesCountAsDisrupted(t *testing.T) {
	r := syncedTaintTestNDB(t, 2, hardTaintsSelector,
		taintTestNode("node1", ciliumTaint),
		taintTestNode("node2", dedicatedTaint),
		taintTestNode("node3"),
	)

	assert.Equal(t, []nodedisruptionv1alpha1.TaintedNode{{Name: "node1", Taints: []string{"node.cilium.io/agent-not-ready:NoSchedule"}}}, r.NodeDisruptionBudget.Status.TaintedNodes)
	assert.Equal(t, 1, r.NodeDisruptionBudget.Status.CurrentDisruptions)
	assert.Equal(t, 1, r.NodeDisruptionBudget.Status.DisruptionsAllowed)
}

func TestNDBTaintedNodeUnderGrantedDisruptionCountedOnce(t *testing.T) {
	r := syncedTaintTestNDB(t, 2, hardTaintsSelector,
		taintTestNode("node1", ciliumTaint),
		taintTestNode("node2", ciliumTaint),
		taintTestNode("node3"),
		taintTestND("nd-node1", nodedisruptionv1alpha1.Granted, "node1"),
	)

	assert.Equal(t, []nodedisruptionv1alpha1.TaintedNode{{Name: "node1", Taints: []string{"node.cilium.io/agent-not-ready:NoSchedule"}}, {Name: "node2", Taints: []string{"node.cilium.io/agent-not-ready:NoSchedule"}}}, r.NodeDisruptionBudget.Status.TaintedNodes)
	assert.Equal(t, 2, r.NodeDisruptionBudget.Status.CurrentDisruptions)
	assert.Equal(t, 0, r.NodeDisruptionBudget.Status.DisruptionsAllowed)
}

func TestNDBPendingDisruptionDoesNotCoverTaintedNode(t *testing.T) {
	r := syncedTaintTestNDB(t, 2, hardTaintsSelector,
		taintTestNode("node1", ciliumTaint),
		taintTestNode("node2"),
		taintTestND("nd-node1", nodedisruptionv1alpha1.Pending, "node1"),
	)

	assert.Equal(t, 1, r.NodeDisruptionBudget.Status.CurrentDisruptions)
	assert.Equal(t, 1, r.NodeDisruptionBudget.Status.DisruptionsAllowed)
}

func TestNDBTolerateDisruptionOnTaintedNode(t *testing.T) {
	r := syncedTaintTestNDB(t, 1, hardTaintsSelector,
		taintTestNode("node1", ciliumTaint),
		taintTestNode("node2"),
	)

	assert.Equal(t, 0, r.NodeDisruptionBudget.Status.DisruptionsAllowed)
	assert.True(t, r.TolerateDisruption(resolver.NewNodeSetFromStringList([]string{"node1"})))
	assert.False(t, r.TolerateDisruption(resolver.NewNodeSetFromStringList([]string{"node2"})))
	assert.False(t, r.TolerateDisruption(resolver.NewNodeSetFromStringList([]string{"node1", "node2"})))
}
