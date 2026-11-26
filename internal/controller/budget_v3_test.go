package controller_test

import (
	"context"
	"errors"
	"testing"
	"time"

	nodedisruptionv1alpha1 "github.com/criteo/node-disruption-controller/api/v1alpha1"
	"github.com/criteo/node-disruption-controller/internal/controller"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newV3TestReconciler() controller.SingleNodeDisruptionReconciler {
	return controller.SingleNodeDisruptionReconciler{
		Client: fake.NewClientBuilder().Build(),
		NodeDisruption: nodedisruptionv1alpha1.NodeDisruption{
			Spec: nodedisruptionv1alpha1.NodeDisruptionSpec{
				Type: "maintenance",
			},
			Status: nodedisruptionv1alpha1.NodeDisruptionStatus{
				DisruptedNodes: []string{"node-dummy-0"},
			},
		},
	}
}

func newHookBudget(name string, v2, v3 bool) *MockBudget {
	return &MockBudget{
		name:           nodedisruptionv1alpha1.NamespacedName{Namespace: name, Name: name, Kind: "ApplicationDisruptionBudget"},
		impacted:       true,
		tolerate:       true,
		supportedTypes: []string{"maintenance"},
		v2:             v2,
		v3:             v3,
	}
}

func statusOf(t *testing.T, statuses []nodedisruptionv1alpha1.DisruptedBudgetStatus, budget *MockBudget) nodedisruptionv1alpha1.DisruptedBudgetStatus {
	t.Helper()
	for _, s := range statuses {
		if s.Reference == budget.name {
			return s
		}
	}
	require.Failf(t, "status not found", "no status for budget %v in %+v", budget.name, statuses)
	return nodedisruptionv1alpha1.DisruptedBudgetStatus{}
}

var errNotNow = errors.New("unable to accept due to internal constraints")

func TestV3AllAcceptedPreparesInSameReconcile(t *testing.T) {
	reconciler := newV3TestReconciler()
	a := newHookBudget("a", false, true)
	b := newHookBudget("b", false, true)
	v2 := newHookBudget("v2", true, false)

	anyFailed, statuses := reconciler.ValidateWithBudgetConstraints(context.Background(), []controller.Budget{a, b, v2}, nil)

	// Preparation was started, so the disruption is not ok yet.
	assert.True(t, anyFailed)
	assert.Equal(t, 1, a.acceptCalls)
	assert.Equal(t, 1, b.acceptCalls)
	assert.Equal(t, 0, v2.acceptCalls, "accept is only called on budgets using v3 hooks")
	for _, budget := range []*MockBudget{a, b, v2} {
		assert.Equal(t, 1, budget.prepareCalls, "prepare must be called in the same reconcile for %v", budget.name)
	}
	assert.Equal(t, nodedisruptionv1alpha1.DisruptedBudgetStatus{Reference: a.name, Preparing: true, Accepted: true}, statusOf(t, statuses, a))
	assert.Equal(t, nodedisruptionv1alpha1.DisruptedBudgetStatus{Reference: b.name, Preparing: true, Accepted: true}, statusOf(t, statuses, b))
	assert.Equal(t, nodedisruptionv1alpha1.DisruptedBudgetStatus{Reference: v2.name, Preparing: true}, statusOf(t, statuses, v2))
}

func TestV3OneBudgetNotAcceptingBlocksEveryPreparation(t *testing.T) {
	reconciler := newV3TestReconciler()
	a := newHookBudget("a", false, true)
	b := newHookBudget("b", false, true)
	b.acceptErr = errNotNow
	v2 := newHookBudget("v2", true, false)

	anyFailed, statuses := reconciler.ValidateWithBudgetConstraints(context.Background(), []controller.Budget{a, b, v2}, nil)

	assert.True(t, anyFailed)
	for _, budget := range []*MockBudget{a, b, v2} {
		assert.Equal(t, 0, budget.prepareCalls, "no budget may be prepared before all accepted, %v was", budget.name)
	}
	assert.True(t, statusOf(t, statuses, a).Accepted)
	assert.False(t, statusOf(t, statuses, a).Preparing)
	assert.False(t, statusOf(t, statuses, b).Accepted)
	assert.Contains(t, statusOf(t, statuses, b).Reason, "not accepted")
	assert.False(t, statusOf(t, statuses, v2).Preparing)
	assert.False(t, statusOf(t, statuses, v2).Accepted)
	// Statuses keep the order of the budgets.
	assert.Equal(t, []nodedisruptionv1alpha1.NamespacedName{a.name, b.name, v2.name},
		[]nodedisruptionv1alpha1.NamespacedName{statuses[0].Reference, statuses[1].Reference, statuses[2].Reference})
}

func TestV3AcceptIsCalledAgainUntilPreparing(t *testing.T) {
	reconciler := newV3TestReconciler()
	a := newHookBudget("a", false, true)
	b := newHookBudget("b", false, true)
	b.acceptErr = errNotNow

	_, statuses := reconciler.ValidateWithBudgetConstraints(context.Background(), []controller.Budget{a, b}, nil)
	require.True(t, statusOf(t, statuses, a).Accepted)

	// a already accepted, it is asked again since acceptance reserves nothing.
	b.acceptErr = nil
	_, statuses = reconciler.ValidateWithBudgetConstraints(context.Background(), []controller.Budget{a, b}, statuses)
	assert.Equal(t, 2, a.acceptCalls)
	assert.Equal(t, 2, b.acceptCalls)
	assert.True(t, statusOf(t, statuses, a).Preparing)
	assert.True(t, statusOf(t, statuses, b).Preparing)

	// Once preparing, accept is no longer called, ready is.
	_, statuses = reconciler.ValidateWithBudgetConstraints(context.Background(), []controller.Budget{a, b}, statuses)
	assert.Equal(t, 2, a.acceptCalls)
	assert.Equal(t, 2, b.acceptCalls)
	assert.Equal(t, 1, a.readyCalls)
	assert.Equal(t, 1, b.readyCalls)
	assert.True(t, statusOf(t, statuses, a).Ok)
	assert.True(t, statusOf(t, statuses, b).Ok)
}

func TestV3AllReadyGrants(t *testing.T) {
	reconciler := newV3TestReconciler()
	a := newHookBudget("a", false, true)
	b := newHookBudget("b", true, false)

	_, statuses := reconciler.ValidateWithBudgetConstraints(context.Background(), []controller.Budget{a, b}, nil)
	anyFailed, statuses := reconciler.ValidateWithBudgetConstraints(context.Background(), []controller.Budget{a, b}, statuses)

	assert.False(t, anyFailed)
	assert.Equal(t, nodedisruptionv1alpha1.DisruptedBudgetStatus{Reference: a.name, Preparing: true, Ok: true, Accepted: true}, statusOf(t, statuses, a))
	assert.Equal(t, nodedisruptionv1alpha1.DisruptedBudgetStatus{Reference: b.name, Preparing: true, Ok: true}, statusOf(t, statuses, b))
}

func TestV3PrepareFailureKeepsBudgetAcceptedAndIsReentrant(t *testing.T) {
	reconciler := newV3TestReconciler()
	a := newHookBudget("a", false, true)
	b := newHookBudget("b", false, true)
	b.prepareErr = errors.New("prepare refused")

	_, statuses := reconciler.ValidateWithBudgetConstraints(context.Background(), []controller.Budget{a, b}, nil)
	assert.True(t, statusOf(t, statuses, a).Preparing)
	bStatus := statusOf(t, statuses, b)
	assert.True(t, bStatus.Accepted, "a budget failing to prepare stays accepted")
	assert.False(t, bStatus.Preparing)
	assert.Contains(t, bStatus.Reason, "cannot prepare")

	// Next reconcile: b goes through accept then prepare again, a keeps preparing (ready).
	b.prepareErr = nil
	_, statuses = reconciler.ValidateWithBudgetConstraints(context.Background(), []controller.Budget{a, b}, statuses)
	assert.Equal(t, 1, a.acceptCalls, "a preparing budget is not asked to accept again")
	assert.Equal(t, 2, b.acceptCalls)
	assert.Equal(t, 2, b.prepareCalls)
	assert.Equal(t, 1, a.prepareCalls)
	assert.Equal(t, 1, a.readyCalls)
	assert.True(t, statusOf(t, statuses, b).Preparing)
}

func TestV3PreparingBudgetIsKeptUntilAllAccepted(t *testing.T) {
	reconciler := newV3TestReconciler()
	a := newHookBudget("a", false, true)
	b := newHookBudget("b", false, true)
	b.acceptErr = errNotNow
	aCurrent := nodedisruptionv1alpha1.DisruptedBudgetStatus{Reference: a.name, Preparing: true, Ok: true, Accepted: true}
	current := []nodedisruptionv1alpha1.DisruptedBudgetStatus{
		aCurrent,
		{Reference: b.name, Accepted: true, Reason: "cannot prepare disruption: prepare refused"},
	}

	anyFailed, statuses := reconciler.ValidateWithBudgetConstraints(context.Background(), []controller.Budget{a, b}, current)

	assert.True(t, anyFailed)
	assert.Equal(t, 0, a.acceptCalls)
	assert.Equal(t, 0, a.readyCalls, "no hook is called on a preparing budget until all budgets accepted")
	assert.Equal(t, aCurrent, statusOf(t, statuses, a))
	assert.False(t, statusOf(t, statuses, b).Accepted, "b withdrew its acceptance")
	assert.Equal(t, 0, b.prepareCalls)
}

func TestV3BudgetWaitingForNoticeBlocksPreparationWithoutFailing(t *testing.T) {
	reconciler := newV3TestReconciler()
	reconciler.NodeDisruption.Spec.StartDate = metav1.NewTime(time.Now().Add(2 * time.Hour))
	waiting := newHookBudget("waiting", false, true)
	waiting.notice = time.Hour // opens one hour from now
	a := newHookBudget("a", false, true)

	anyFailed, statuses := reconciler.ValidateWithBudgetConstraints(context.Background(), []controller.Budget{waiting, a}, nil)

	assert.False(t, anyFailed, "waiting for a notice period is not a failure")
	assert.Equal(t, 0, waiting.acceptCalls, "accept is not called before the notice period opens")
	assert.True(t, statusOf(t, statuses, waiting).Waiting)
	assert.Equal(t, 1, a.acceptCalls)
	assert.True(t, statusOf(t, statuses, a).Accepted)
	assert.Equal(t, 0, a.prepareCalls, "a budget waiting for its notice blocks preparation of the others")
}

func TestWithoutV3BudgetAcceptIsNeverCalled(t *testing.T) {
	reconciler := newV3TestReconciler()
	a := newHookBudget("a", true, false)
	b := newHookBudget("b", true, false)
	b.prepareErr = errors.New("prepare refused")

	anyFailed, statuses := reconciler.ValidateWithBudgetConstraints(context.Background(), []controller.Budget{a, b}, nil)

	// v2 behaviour: each budget is prepared independently, b refusing does not prevent a from preparing.
	assert.True(t, anyFailed)
	assert.Equal(t, 1, a.prepareCalls)
	assert.Equal(t, 1, b.prepareCalls)
	assert.True(t, statusOf(t, statuses, a).Preparing)
	assert.False(t, statusOf(t, statuses, a).Accepted)
	assert.False(t, statusOf(t, statuses, b).Preparing)
}

func newLegacyBudget(name, kind string) *MockBudget {
	budget := newHookBudget(name, false, false)
	budget.name.Kind = kind
	if kind == "NodeDisruptionBudget" {
		budget.name.Namespace = ""
	}
	return budget
}

func TestV3MixedWithV2AndLegacyBudgets(t *testing.T) {
	reconciler := newV3TestReconciler()
	v3 := newHookBudget("v3", false, true)
	v2 := newHookBudget("v2", true, false)
	legacy := newLegacyBudget("legacy", "ApplicationDisruptionBudget")
	ndb := newLegacyBudget("ndb", "NodeDisruptionBudget")
	budgets := []controller.Budget{v3, v2, legacy, ndb}

	// First reconcile: accept is only asked to the v3 budget, then every budget is evaluated
	// in the same reconcile: prepare for v2 and v3, health hook for the legacy budgets.
	anyFailed, statuses := reconciler.ValidateWithBudgetConstraints(context.Background(), budgets, nil)
	assert.True(t, anyFailed, "v2 and v3 budgets are preparing, not ok yet")
	assert.Equal(t, 1, v3.acceptCalls)
	assert.Equal(t, 0, v2.acceptCalls)
	assert.Equal(t, 1, v3.prepareCalls)
	assert.Equal(t, 1, v2.prepareCalls)
	assert.True(t, legacy.healthChecked)
	assert.True(t, ndb.healthChecked)
	assert.Equal(t, nodedisruptionv1alpha1.DisruptedBudgetStatus{Reference: v3.name, Preparing: true, Accepted: true}, statusOf(t, statuses, v3))
	assert.Equal(t, nodedisruptionv1alpha1.DisruptedBudgetStatus{Reference: v2.name, Preparing: true}, statusOf(t, statuses, v2))
	assert.Equal(t, nodedisruptionv1alpha1.DisruptedBudgetStatus{Reference: legacy.name, Ok: true}, statusOf(t, statuses, legacy))
	assert.Equal(t, nodedisruptionv1alpha1.DisruptedBudgetStatus{Reference: ndb.name, Ok: true}, statusOf(t, statuses, ndb))

	// Second reconcile: ready for v2 and v3, health hook again for the legacy budgets
	// (they are stateless), everything is ok.
	legacy.healthChecked, ndb.healthChecked = false, false
	anyFailed, statuses = reconciler.ValidateWithBudgetConstraints(context.Background(), budgets, statuses)
	assert.False(t, anyFailed)
	assert.Equal(t, 1, v3.acceptCalls, "a preparing budget is not asked to accept again")
	assert.Equal(t, 1, v3.readyCalls)
	assert.Equal(t, 1, v2.readyCalls)
	assert.True(t, legacy.healthChecked)
	assert.True(t, ndb.healthChecked)
	assert.Equal(t, nodedisruptionv1alpha1.DisruptedBudgetStatus{Reference: v3.name, Preparing: true, Ok: true, Accepted: true}, statusOf(t, statuses, v3))
	assert.Equal(t, nodedisruptionv1alpha1.DisruptedBudgetStatus{Reference: v2.name, Preparing: true, Ok: true}, statusOf(t, statuses, v2))
	assert.Equal(t, nodedisruptionv1alpha1.DisruptedBudgetStatus{Reference: legacy.name, Ok: true}, statusOf(t, statuses, legacy))
	assert.Equal(t, nodedisruptionv1alpha1.DisruptedBudgetStatus{Reference: ndb.name, Ok: true}, statusOf(t, statuses, ndb))
}

func TestV3NotAcceptingHoldsLegacyBudgetsWithoutCallingThem(t *testing.T) {
	reconciler := newV3TestReconciler()
	v3 := newHookBudget("v3", false, true)
	v3.acceptErr = errNotNow
	legacy := newLegacyBudget("legacy", "ApplicationDisruptionBudget")
	ndb := newLegacyBudget("ndb", "NodeDisruptionBudget")

	anyFailed, statuses := reconciler.ValidateWithBudgetConstraints(context.Background(), []controller.Budget{v3, legacy, ndb}, nil)

	assert.True(t, anyFailed)
	assert.False(t, legacy.healthChecked, "no hook is called on any budget before every budget accepted")
	assert.False(t, ndb.healthChecked)
	assert.Contains(t, statusOf(t, statuses, v3).Reason, "not accepted")
	for _, budget := range []*MockBudget{legacy, ndb} {
		status := statusOf(t, statuses, budget)
		assert.False(t, status.Ok)
		assert.False(t, status.Accepted, "accepted is only set on budgets using v3 hooks")
		assert.Contains(t, status.Reason, "waiting for all impacted budgets to accept")
	}
}

// Legacy budgets have no accept hook and count as accepted: their health hook is called with the
// prepare hooks. An unhealthy legacy budget does not prevent the v3 budgets from being prepared in
// that reconciliation, as with v2 budgets.
func TestV3UnhealthyLegacyBudgetDoesNotPreventPreparation(t *testing.T) {
	reconciler := newV3TestReconciler()
	v3 := newHookBudget("v3", false, true)
	legacy := newLegacyBudget("legacy", "ApplicationDisruptionBudget")
	legacy.health = errors.New("cluster degraded")

	anyFailed, statuses := reconciler.ValidateWithBudgetConstraints(context.Background(), []controller.Budget{v3, legacy}, nil)

	assert.True(t, anyFailed)
	assert.Equal(t, 1, v3.prepareCalls)
	assert.True(t, statusOf(t, statuses, v3).Preparing)
	assert.Contains(t, statusOf(t, statuses, legacy).Reason, "Unhealthy")
	assert.False(t, statusOf(t, statuses, legacy).Ok)
}
