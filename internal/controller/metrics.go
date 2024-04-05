package controller

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	METIC_PREFIX = "node_disruption_controller_"
)

var (
	// NODE DISRUPTION METRICS
	NodeDisruptionGrantedTotal = promauto.With(metrics.Registry).NewCounterVec(
		prometheus.CounterOpts{
			Name: METIC_PREFIX + "node_disruption_granted_total",
			Help: "Total number of granted node disruptions",
		},
		[]string{},
	)
	NodeDisruptionRejectedTotal = promauto.With(metrics.Registry).NewCounterVec(
		prometheus.CounterOpts{
			Name: METIC_PREFIX + "node_disruption_rejected_total",
			Help: "Total number of rejected node disruptions",
		},
		[]string{},
	)
	NodeDisruptionStateAsValue = promauto.With(metrics.Registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: METIC_PREFIX + "node_disruption_state_value",
			Help: "State of node disruption: pending=0, rejected=-1, accepted=1",
		},
		[]string{"node_disruption_name"},
	)
	NodeDisruptionStateAsLabel = promauto.With(metrics.Registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: METIC_PREFIX + "node_disruption_state_label",
			Help: "State of node disruption: 0 not in this state; 1 is in state",
		},
		[]string{"node_disruption_name", "state"},
	)
	NodeDisruptionCreated = promauto.With(metrics.Registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: METIC_PREFIX + "node_disruption_created",
			Help: "Date of create of the node disruption",
		},
		[]string{"node_disruption_name"},
	)
	NodeDisruptionDeadline = promauto.With(metrics.Registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: METIC_PREFIX + "node_disruption_deadline",
			Help: "Date of the deadline of the node disruption (0 if unset)",
		},
		[]string{"node_disruption_name"},
	)
	NodeDisruptionImpactedNodes = promauto.With(metrics.Registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: METIC_PREFIX + "node_disruption_impacted_node",
			Help: "high cardinality: create a metric for each node impacted by a given node disruption",
		},
		[]string{"node_disruption_name", "node_name"},
	)
	// DISRUPTION BUDGET METRICS
	DisruptionBudgetCheckHealthHookStatusCodeTotal = promauto.With(metrics.Registry).NewCounterVec(
		prometheus.CounterOpts{
			Name: METIC_PREFIX + "disruption_budget_health_hook_status_code_total",
			Help: "Total number of request by HTTP status code",
		},
		[]string{"disruption_budget_namespace", "disruption_budget_name", "disruption_budget_kind", "status_code"},
	)
	DisruptionBudgetCheckHealthHookErrorTotal = promauto.With(metrics.Registry).NewCounterVec(
		prometheus.CounterOpts{
			Name: METIC_PREFIX + "disruption_budget_health_hook_error_total",
			Help: "Total number of connection/response errors while requesting health hook",
		},
		[]string{"disruption_budget_namespace", "disruption_budget_name", "disruption_budget_kind"},
	)
	DisruptionBudgetRejectedTotal = promauto.With(metrics.Registry).NewCounterVec(
		prometheus.CounterOpts{
			Name: METIC_PREFIX + "disruption_budget_rejected_total",
			Help: "Total number of rejected node disruption by the disruption budget",
		},
		[]string{"disruption_budget_namespace", "disruption_budget_name", "disruption_budget_kind"},
	)
	DisruptionBudgetGrantedTotal = promauto.With(metrics.Registry).NewCounterVec(
		prometheus.CounterOpts{
			Name: METIC_PREFIX + "disruption_budget_granted_total",
			Help: "Total number of granted node disruption by the disruption budget",
		},
		[]string{"disruption_budget_namespace", "disruption_budget_name", "disruption_budget_kind"},
	)
	DisruptionBudgetMaxDisruptions = promauto.With(metrics.Registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: METIC_PREFIX + "disruption_budget_max_disruptions",
			Help: "Reflect the MaxDisruptions fields from budget Spec",
		},
		[]string{"disruption_budget_namespace", "disruption_budget_name", "disruption_budget_kind"},
	)
	DisruptionBudgetCurrentDisruptions = promauto.With(metrics.Registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: METIC_PREFIX + "disruption_budget_current_disruptions",
			Help: "Reflect the CurrentDisruptions fields from budget Status",
		},
		[]string{"disruption_budget_namespace", "disruption_budget_name", "disruption_budget_kind"},
	)
	DisruptionBudgetDisruptionsAllowed = promauto.With(metrics.Registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: METIC_PREFIX + "disruption_budget_disruptions_allowed",
			Help: "Reflect the DisruptionsAllowed fields from budget Status",
		},
		[]string{"disruption_budget_namespace", "disruption_budget_name", "disruption_budget_kind"},
	)
	DisruptionBudgetMaxDisruptedNodes = promauto.With(metrics.Registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: METIC_PREFIX + "disruption_budget_max_disrupted_nodes",
			Help: "Reflect the MaxDisruptedNodes fields from budget Spec",
		},
		[]string{"disruption_budget_namespace", "disruption_budget_name", "disruption_budget_kind"},
	)
	DisruptionBudgetMinUndisruptedNodes = promauto.With(metrics.Registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: METIC_PREFIX + "disruption_budget_min_undisrupted_nodes",
			Help: "Reflect the MinUndisruptedNodes fields from budget Spec",
		},
		[]string{"disruption_budget_namespace", "disruption_budget_name", "disruption_budget_kind"},
	)
	DisruptionBudgetWatchedNodes = promauto.With(metrics.Registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: METIC_PREFIX + "node_disruption_watched_nodes",
			Help: "high cardinality: create a metric for each node watched by a budget",
		},
		[]string{"disruption_budget_namespace", "disruption_budget_name", "disruption_budget_kind", "node_name"},
	)
	DisruptionBudgetDisruptions = promauto.With(metrics.Registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: METIC_PREFIX + "budget_disruption_disruptions",
			Help: "high cardinality: create a metric for each disruption by a budget",
		},
		[]string{"disruption_budget_namespace", "disruption_budget_name", "disruption_budget_kind", "node_disruption_name"},
	)
)
