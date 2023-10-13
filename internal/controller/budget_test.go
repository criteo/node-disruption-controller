package controller_test

import (
	"context"
	"fmt"
	"testing"

	nodedisruptionv1alpha1 "github.com/criteo/node-disruption-controller/api/v1alpha1"
	"github.com/criteo/node-disruption-controller/internal/controller"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type MockBudget struct {
	name     nodedisruptionv1alpha1.NamespacedName
	impacted bool
	tolerate bool
	// True if the budget was health checked
	health_checked bool
	health         error
}

func (m *MockBudget) Sync(context.Context) error {
	return nil
}

// Check if the budget would be impacted by an operation on the provided set of nodes
func (m *MockBudget) IsImpacted(controller.NodeDisruption) bool {
	return m.impacted
}

// Return the number of disruption allowed considering a list of current node disruptions
func (m *MockBudget) TolerateDisruption(controller.NodeDisruption) bool {
	return m.tolerate
}

// Check health make a synchronous health check on the underlying ressource of a budget
func (m *MockBudget) CheckHealth(context.Context) error {
	m.health_checked = true
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

func TestValidationNoImpactedBudget(t *testing.T) {
	resolver := controller.NodeDisruptionResolver{Client: fake.NewClientBuilder().Build()}
	nodes := []string{"node-dummy-0", "node-dummy-1"}

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

	disruption := controller.NodeDisruption{controller.NewNodeSetFromStringList(nodes)}
	any_failed, statuses := resolver.DoValidateDisruption(context.Background(), budgets, disruption)
	assert.False(t, any_failed)
	assert.Equal(t, len(statuses), 0)
}

func TestValidationImpactedAllOk(t *testing.T) {
	resolver := controller.NodeDisruptionResolver{Client: fake.NewClientBuilder().Build()}
	nodes := []string{"node-dummy-0", "node-dummy-1"}

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

	disruption := controller.NodeDisruption{controller.NewNodeSetFromStringList(nodes)}
	any_failed, statuses := resolver.DoValidateDisruption(context.Background(), budgets, disruption)
	assert.False(t, any_failed)
	assert.Equal(t, len(statuses), 2)
}

func TestValidationFailAtDisruption(t *testing.T) {
	resolver := controller.NodeDisruptionResolver{Client: fake.NewClientBuilder().Build()}
	nodes := []string{"node-dummy-0", "node-dummy-1"}

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

	disruption := controller.NodeDisruption{controller.NewNodeSetFromStringList(nodes)}
	any_failed, statuses := resolver.DoValidateDisruption(context.Background(), budgets, disruption)
	assert.True(t, any_failed)
	assert.Equal(t, len(statuses), 1)
	assert.False(t, statuses[0].Ok)
	assert.NotEmpty(t, statuses[0].Reason, "Rejected budget should provide a reason")

	assert.False(t, budget1.health_checked, "Health check should not be made if disruption check failed")
	assert.False(t, budget2.health_checked, "Health check should not be made if disruption check failed")
}

func TestValidationFailAtHealth(t *testing.T) {
	resolver := controller.NodeDisruptionResolver{Client: fake.NewClientBuilder().Build()}
	nodes := []string{"node-dummy-0", "node-dummy-1"}

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

	disruption := controller.NodeDisruption{controller.NewNodeSetFromStringList(nodes)}
	any_failed, statuses := resolver.DoValidateDisruption(context.Background(), budgets, disruption)
	assert.True(t, any_failed)
	assert.False(t, statuses[0].Ok)
	assert.NotEmpty(t, statuses[0].Reason, "Rejected budget should provide a reason")
	assert.True(t, budget1.health_checked)
}
