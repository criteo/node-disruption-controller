package controller

import "github.com/prometheus/client_golang/prometheus"

const (
	METIC_PREFIX = "node_disruption_controller_"
)

var (
	// NODE DISRUPTION METRICS
	NodeDisruptionGrantedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: METIC_PREFIX + "node_disruption_granted_total",
			Help: "Total number of granted node disruptions",
		},
		[]string{},
	)
	NodeDisruptionRejectedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: METIC_PREFIX + "node_disruption_rejected_total",
			Help: "Total number of rejected node disruptions",
		},
		[]string{},
	)
	NodeDisruptionState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: METIC_PREFIX + "node_disruption_state",
			Help: "State of node disruption: pending=0, rejected=-1, accepted=1",
		},
		[]string{"node_disruption_name"},
	)
	NodeDisruptionCreated = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: METIC_PREFIX + "node_disruption_created",
			Help: "Date of create of the node disruption",
		},
		[]string{"node_disruption_name"},
	)
	NodeDisruptionDeadline = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: METIC_PREFIX + "node_disruption_deadline",
			Help: "Date of the deadline of the node disruption (0 if unset)",
		},
		[]string{"node_disruption_name"},
	)
	NodeDisruptionImpactedNodes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: METIC_PREFIX + "node_disruption_impacted_node",
			Help: "high cardinality: create a metric for each node impacted by a given node disruption",
		},
		[]string{"node_disruption_name", "node_name"},
	)
	// APPLICATION DISRUPTION BUDGET METRICS
	DisruptionBudgetCheckHealthHookStatusCodeTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: METIC_PREFIX + "disruption_budget_health_hook_status_code_total",
			Help: "Total number of request by HTTP status code",
		},
		[]string{"disruption_budget_namespace", "disruption_budget_name", "disruption_budget_kind", "status_code"},
	)
	DisruptionBudgetCheckHealthHookErrorTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: METIC_PREFIX + "disruption_budget_health_hook_error_total",
			Help: "Total number of connection/response errors while requesting health hook",
		},
		[]string{"disruption_budget_namespace", "disruption_budget_name", "disruption_budget_kind"},
	)
	DisruptionBudgetRejectedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: METIC_PREFIX + "disruption_budget_rejected_total",
			Help: "Total number of rejected node disruption by the disruption budget",
		},
		[]string{"disruption_budget_namespace", "disruption_budget_name", "disruption_budget_kind"},
	)
	DisruptionBudgetGrantedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: METIC_PREFIX + "disruption_budget_granted_total",
			Help: "Total number of granted node disruption by the disruption budget",
		},
		[]string{"disruption_budget_namespace", "disruption_budget_name", "disruption_budget_kind"},
	)
)
