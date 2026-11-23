package v1alpha1

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestTaintSelectorMatchingTaints(t *testing.T) {
	dedicated := corev1.Taint{Key: "crto.in/dedicated", Value: "team", Effect: corev1.TaintEffectNoSchedule}
	cilium := corev1.Taint{Key: "node.cilium.io/agent-not-ready", Effect: corev1.TaintEffectNoSchedule}
	unreachable := corev1.Taint{Key: "node.kubernetes.io/unreachable", Effect: corev1.TaintEffectNoExecute}
	prefer := corev1.Taint{Key: "example.com/prefer", Effect: corev1.TaintEffectPreferNoSchedule}

	allHardEffectsButDedicated := TaintSelector{
		Include: []corev1.Toleration{
			{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
			{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoExecute},
		},
		Exclude: []corev1.Toleration{
			{Key: "crto.in/dedicated", Operator: corev1.TolerationOpExists},
		},
	}
	ciliumOnly := TaintSelector{
		Include: []corev1.Toleration{
			{Key: "node.cilium.io/agent-not-ready", Operator: corev1.TolerationOpExists},
		},
	}
	allButDedicated := TaintSelector{
		Exclude: []corev1.Toleration{
			{Key: "crto.in/dedicated", Operator: corev1.TolerationOpExists},
		},
	}

	tests := []struct {
		name     string
		selector TaintSelector
		taints   []corev1.Taint
		want     []string
	}{
		{"no taints", allHardEffectsButDedicated, nil, nil},
		{"excluded taint only", allHardEffectsButDedicated, []corev1.Taint{dedicated}, nil},
		{"included NoSchedule taint", allHardEffectsButDedicated, []corev1.Taint{cilium}, []string{"node.cilium.io/agent-not-ready:NoSchedule"}},
		{"included NoExecute taint", allHardEffectsButDedicated, []corev1.Taint{unreachable}, []string{"node.kubernetes.io/unreachable:NoExecute"}},
		{"PreferNoSchedule not included", allHardEffectsButDedicated, []corev1.Taint{prefer}, nil},
		{"excluded taint does not hide an included one", allHardEffectsButDedicated, []corev1.Taint{dedicated, cilium}, []string{"node.cilium.io/agent-not-ready:NoSchedule"}},
		{"allowlist matches listed key", ciliumOnly, []corev1.Taint{cilium}, []string{"node.cilium.io/agent-not-ready:NoSchedule"}},
		{"allowlist ignores other keys", ciliumOnly, []corev1.Taint{dedicated, unreachable}, nil},
		{"empty include matches any taint", allButDedicated, []corev1.Taint{prefer, unreachable}, []string{"example.com/prefer:PreferNoSchedule", "node.kubernetes.io/unreachable:NoExecute"}},
		{"empty include still applies exclude", allButDedicated, []corev1.Taint{dedicated}, nil},
		{"empty selector matches any taint", TaintSelector{}, []corev1.Taint{dedicated}, []string{"crto.in/dedicated=team:NoSchedule"}},
		{"empty selector needs a taint", TaintSelector{}, nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &corev1.Node{Spec: corev1.NodeSpec{Taints: tt.taints}}
			if got := tt.selector.MatchingTaints(node); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MatchingTaints() = %v, want %v", got, tt.want)
			}
		})
	}
}
