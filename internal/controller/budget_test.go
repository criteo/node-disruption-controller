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
	name           nodedisruptionv1alpha1.NamespacedName
	impacted       bool
	tolerate       bool
	supportedTypes []string
	// True if the budget was health checked
	healthChecked bool
	health        error
	// Hook versions: v2 enables prepare/ready, v3 enables accept/prepare/ready
	v2 bool
	v3 bool
	// Errors returned by the hooks, and number of calls to each of them
	acceptErr    error
	prepareErr   error
	readyErr     error
	acceptCalls  int
	prepareCalls int
	readyCalls   int
	// Notice period returned for every disruption type
	notice time.Duration
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

func (m *MockBudget) V2HooksReady() bool {
	return m.v2
}

func (m *MockBudget) V3HooksReady() bool {
	return m.v3
}

func (m *MockBudget) CallAcceptHook(context.Context, nodedisruptionv1alpha1.NodeDisruption, time.Duration) error {
	m.acceptCalls++
	return m.acceptErr
}

func (m *MockBudget) CallPrepareHook(context.Context, nodedisruptionv1alpha1.NodeDisruption, time.Duration) error {
	m.prepareCalls++
	return m.prepareErr
}

func (m *MockBudget) CallReadyHook(context.Context, nodedisruptionv1alpha1.NodeDisruption, time.Duration) error {
	m.readyCalls++
	return m.readyErr
}

func (m *MockBudget) CallTerminateHook(context.Context, nodedisruptionv1alpha1.NodeDisruption, time.Duration) error {
	return nil
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

func (m *MockBudget) GetSupportedDisruptionTypes() []string {
	return m.supportedTypes
}

func (m *MockBudget) NoticePeriodFor(_ string) time.Duration {
	return m.notice
}

func TestValidateWithBudgetConstraintsNoImpactedBudget(t *testing.T) {
	nodes := []string{"node-dummy-0", "node-dummy-1"}
	reconciler := controller.SingleNodeDisruptionReconciler{
		Client: fake.NewClientBuilder().Build(),
		NodeDisruption: nodedisruptionv1alpha1.NodeDisruption{
			Spec: nodedisruptionv1alpha1.NodeDisruptionSpec{
				Type: "maintenance",
			},
			Status: nodedisruptionv1alpha1.NodeDisruptionStatus{
				DisruptedNodes: nodes,
			},
		},
	}

	budget1 := MockBudget{
		name:           nodedisruptionv1alpha1.NamespacedName{Namespace: "test1", Name: "test1"},
		impacted:       false,
		tolerate:       false,
		supportedTypes: []string{"maintenance"},
		health:         nil,
	}

	budget2 := MockBudget{
		name:           nodedisruptionv1alpha1.NamespacedName{Namespace: "test2", Name: "test2"},
		impacted:       false,
		tolerate:       false,
		supportedTypes: []string{"maintenance"},
		health:         nil,
	}

	budgets := []controller.Budget{&budget1, &budget2}
	anyFailed, statuses := reconciler.ValidateWithBudgetConstraints(context.Background(), budgets, nil)
	assert.False(t, anyFailed)
	assert.Equal(t, len(statuses), 0)
}

func TestValidationImpactedAllOk(t *testing.T) {
	nodes := []string{"node-dummy-0", "node-dummy-1"}
	reconciler := controller.SingleNodeDisruptionReconciler{
		Client: fake.NewClientBuilder().Build(),
		NodeDisruption: nodedisruptionv1alpha1.NodeDisruption{
			Spec: nodedisruptionv1alpha1.NodeDisruptionSpec{
				Type: "maintenance",
			},
			Status: nodedisruptionv1alpha1.NodeDisruptionStatus{
				DisruptedNodes: nodes,
			},
		},
	}

	budget1 := MockBudget{
		name:           nodedisruptionv1alpha1.NamespacedName{Namespace: "test1", Name: "test1"},
		impacted:       true,
		tolerate:       true,
		supportedTypes: []string{"maintenance"},
		health:         nil,
	}

	budget2 := MockBudget{
		name:           nodedisruptionv1alpha1.NamespacedName{Namespace: "test2", Name: "test2"},
		impacted:       true,
		tolerate:       true,
		supportedTypes: []string{"maintenance"},
		health:         nil,
	}

	budgets := []controller.Budget{&budget1, &budget2}

	anyFailed, statuses := reconciler.ValidateWithBudgetConstraints(context.Background(), budgets, nil)
	assert.False(t, anyFailed)
	assert.Equal(t, len(statuses), 2)
}

func TestValidateWithBudgetConstraintsFailAtDisruption(t *testing.T) {
	nodes := []string{"node-dummy-0", "node-dummy-1"}
	reconciler := controller.SingleNodeDisruptionReconciler{
		Client: fake.NewClientBuilder().Build(),
		NodeDisruption: nodedisruptionv1alpha1.NodeDisruption{
			Spec: nodedisruptionv1alpha1.NodeDisruptionSpec{
				Type: "maintenance",
			},
			Status: nodedisruptionv1alpha1.NodeDisruptionStatus{
				DisruptedNodes: nodes,
			},
		},
	}

	budget1 := MockBudget{
		name:           nodedisruptionv1alpha1.NamespacedName{Namespace: "test1", Name: "test1"},
		impacted:       true,
		tolerate:       false,
		supportedTypes: []string{"maintenance"},
		health:         nil,
	}

	budget2 := MockBudget{
		name:           nodedisruptionv1alpha1.NamespacedName{Namespace: "test2", Name: "test2"},
		impacted:       true,
		tolerate:       true,
		supportedTypes: []string{"maintenance"},
		health:         nil,
	}

	budgets := []controller.Budget{&budget1, &budget2}

	anyFailed, statuses := reconciler.ValidateWithBudgetConstraints(context.Background(), budgets, nil)
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
			Spec: nodedisruptionv1alpha1.NodeDisruptionSpec{
				Type: "maintenance",
			},
			Status: nodedisruptionv1alpha1.NodeDisruptionStatus{
				DisruptedNodes: nodes,
			},
		},
	}

	budget1 := MockBudget{
		name:           nodedisruptionv1alpha1.NamespacedName{Namespace: "test1", Name: "test1"},
		impacted:       true,
		tolerate:       true,
		supportedTypes: []string{"maintenance"},
		health:         fmt.Errorf("testerror"),
	}

	budget2 := MockBudget{
		name:           nodedisruptionv1alpha1.NamespacedName{Namespace: "test2", Name: "test2"},
		impacted:       true,
		tolerate:       true,
		supportedTypes: []string{"maintenance"},
		health:         nil,
	}

	budgets := []controller.Budget{&budget1, &budget2}

	anyFailed, statuses := reconciler.ValidateWithBudgetConstraints(context.Background(), budgets, nil)
	assert.True(t, anyFailed)
	assert.False(t, statuses[0].Ok)
	assert.NotEmpty(t, statuses[0].Reason, "Rejected budget should provide a reason")
	assert.True(t, budget1.healthChecked)
}
