package controller_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	nodedisruptionv1alpha1 "github.com/criteo/node-disruption-controller/api/v1alpha1"
	"github.com/criteo/node-disruption-controller/internal/controller"
	"github.com/criteo/node-disruption-controller/pkg/resolver"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type MockBudget struct {
	name     nodedisruptionv1alpha1.NamespacedName
	impacted bool
	tolerate bool
	// True if the budget was health checked
	healthChecked bool
	health        error
}

func (m *MockBudget) Sync(context.Context) error {
	return nil
}

// Check if the budget would be impacted by an operation on the provided set of nodes
func (m *MockBudget) IsImpacted(resolver.NodeSet) bool {
	return m.impacted
}

// Return the number of disruption allowed considering a list of current node disruptions
func (m *MockBudget) TolerateDisruption(resolver.NodeSet) bool {
	return m.tolerate
}

// Check health make a synchronous health check on the underlying resource of a budget
func (m *MockBudget) CallHealthHook(context.Context, nodedisruptionv1alpha1.NodeDisruption, time.Duration) error {
	m.healthChecked = true
	return m.health
}

// Apply the budget's status to Kubernetes
func (m *MockBudget) UpdateStatus(context.Context) error {
	return nil
}

// Get the name, namespace and kind of bduget
func (m *MockBudget) GetNamespacedName() nodedisruptionv1alpha1.NamespacedName {
	return m.name
}

func TestValidateWithBudgetConstraintsNoImpactedBudget(t *testing.T) {
	nodes := []string{"node-dummy-0", "node-dummy-1"}
	reconciler := controller.SingleNodeDisruptionReconciler{
		Client: fake.NewClientBuilder().Build(),
		NodeDisruption: nodedisruptionv1alpha1.NodeDisruption{
			Status: nodedisruptionv1alpha1.NodeDisruptionStatus{
				DisruptedNodes: nodes,
			},
		},
	}

	budget1 := MockBudget{
		name:     nodedisruptionv1alpha1.NamespacedName{Namespace: "test1", Name: "test1"},
		impacted: false,
		tolerate: false,
		health:   nil,
	}

	budget2 := MockBudget{
		name:     nodedisruptionv1alpha1.NamespacedName{Namespace: "test2", Name: "test2"},
		impacted: false,
		tolerate: false,
		health:   nil,
	}

	budgets := []controller.Budget{&budget1, &budget2}
	anyFailed, statuses := reconciler.ValidateWithBudgetConstraints(context.Background(), budgets)
	assert.False(t, anyFailed)
	assert.Equal(t, len(statuses), 0)
}

func TestValidationImpactedAllOk(t *testing.T) {
	nodes := []string{"node-dummy-0", "node-dummy-1"}
	reconciler := controller.SingleNodeDisruptionReconciler{
		Client: fake.NewClientBuilder().Build(),
		NodeDisruption: nodedisruptionv1alpha1.NodeDisruption{
			Status: nodedisruptionv1alpha1.NodeDisruptionStatus{
				DisruptedNodes: nodes,
			},
		},
	}

	budget1 := MockBudget{
		name:     nodedisruptionv1alpha1.NamespacedName{Namespace: "test1", Name: "test1"},
		impacted: true,
		tolerate: true,
		health:   nil,
	}

	budget2 := MockBudget{
		name:     nodedisruptionv1alpha1.NamespacedName{Namespace: "test2", Name: "test2"},
		impacted: true,
		tolerate: true,
		health:   nil,
	}

	budgets := []controller.Budget{&budget1, &budget2}

	anyFailed, statuses := reconciler.ValidateWithBudgetConstraints(context.Background(), budgets)
	assert.False(t, anyFailed)
	assert.Equal(t, len(statuses), 2)
}

func TestValidateWithBudgetConstraintsFailAtDisruption(t *testing.T) {
	nodes := []string{"node-dummy-0", "node-dummy-1"}
	reconciler := controller.SingleNodeDisruptionReconciler{
		Client: fake.NewClientBuilder().Build(),
		NodeDisruption: nodedisruptionv1alpha1.NodeDisruption{
			Status: nodedisruptionv1alpha1.NodeDisruptionStatus{
				DisruptedNodes: nodes,
			},
		},
	}

	budget1 := MockBudget{
		name:     nodedisruptionv1alpha1.NamespacedName{Namespace: "test1", Name: "test1"},
		impacted: true,
		tolerate: false,
		health:   nil,
	}

	budget2 := MockBudget{
		name:     nodedisruptionv1alpha1.NamespacedName{Namespace: "test2", Name: "test2"},
		impacted: true,
		tolerate: true,
		health:   nil,
	}

	budgets := []controller.Budget{&budget1, &budget2}

	anyFailed, statuses := reconciler.ValidateWithBudgetConstraints(context.Background(), budgets)
	assert.True(t, anyFailed)
	assert.Equal(t, len(statuses), 1)
	assert.False(t, statuses[0].Ok)
	assert.NotEmpty(t, statuses[0].Reason, "Rejected budget should provide a reason")

	assert.False(t, budget1.healthChecked, "Health check should not be made if disruption check failed")
	assert.False(t, budget2.healthChecked, "Health check should not be made if disruption check failed")
}

func TestValidateWithBudgetConstraintsFailAtHealth(t *testing.T) {
	nodes := []string{"node-dummy-0", "node-dummy-1"}
	reconciler := controller.SingleNodeDisruptionReconciler{
		Client: fake.NewClientBuilder().Build(),
		NodeDisruption: nodedisruptionv1alpha1.NodeDisruption{
			Status: nodedisruptionv1alpha1.NodeDisruptionStatus{
				DisruptedNodes: nodes,
			},
		},
	}

	budget1 := MockBudget{
		name:     nodedisruptionv1alpha1.NamespacedName{Namespace: "test1", Name: "test1"},
		impacted: true,
		tolerate: true,
		health:   fmt.Errorf("testerror"),
	}

	budget2 := MockBudget{
		name:     nodedisruptionv1alpha1.NamespacedName{Namespace: "test2", Name: "test2"},
		impacted: true,
		tolerate: true,
		health:   nil,
	}

	budgets := []controller.Budget{&budget1, &budget2}

	anyFailed, statuses := reconciler.ValidateWithBudgetConstraints(context.Background(), budgets)
	assert.True(t, anyFailed)
	assert.False(t, statuses[0].Ok)
	assert.NotEmpty(t, statuses[0].Reason, "Rejected budget should provide a reason")
	assert.True(t, budget1.healthChecked)
}
