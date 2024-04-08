/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"

	"k8s.io/apimachinery/pkg/api/errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	nodedisruptionv1alpha1 "github.com/criteo/node-disruption-controller/api/v1alpha1"
	"github.com/criteo/node-disruption-controller/pkg/resolver"
	"github.com/prometheus/client_golang/prometheus"
)

// ApplicationDisruptionBudgetReconciler reconciles a ApplicationDisruptionBudget object
type ApplicationDisruptionBudgetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=nodedisruption.criteo.com,resources=applicationdisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nodedisruption.criteo.com,resources=applicationdisruptionbudgets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nodedisruption.criteo.com,resources=applicationdisruptionbudgets/finalizers,verbs=update

//+kubebuilder:rbac:groups="",resources=pods;persistentvolumeclaims;persistentvolumes;nodes,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ApplicationDisruptionBudget object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *ApplicationDisruptionBudgetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	adb := &nodedisruptionv1alpha1.ApplicationDisruptionBudget{}
	err := r.Client.Get(ctx, req.NamespacedName, adb)
	ref := nodedisruptionv1alpha1.NamespacedName{
		Namespace: req.Namespace,
		Name:      req.Name,
		Kind:      "ApplicationDisruptionBudget",
	}

	if err != nil {
		if errors.IsNotFound(err) {
			// If the resource was not found, nothing has to be done
			PruneADBMetrics(ref)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	UpdateADBMetrics(ref, adb)
	logger.Info("Start reconcile of adb", "version", adb.ResourceVersion)

	resolver := ApplicationDisruptionBudgetResolver{
		ApplicationDisruptionBudget: adb.DeepCopy(),
		Client:                      r.Client,
		Resolver:                    resolver.Resolver{Client: r.Client},
	}

	err = resolver.Sync(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !reflect.DeepEqual(resolver.ApplicationDisruptionBudget.Status, adb.Status) {
		logger.Info("Updating with", "version", adb.ResourceVersion)
		err = resolver.UpdateStatus(ctx)
	}

	return ctrl.Result{}, err
}

// PruneNodeDisruptionMetric remove metrics for an ADB that don't exist anymore
func PruneADBMetrics(ref nodedisruptionv1alpha1.NamespacedName) {
	DisruptionBudgetMaxDisruptions.DeletePartialMatch(prometheus.Labels{"budget_disruption_namespace": ref.Namespace, "budget_disruption_name": ref.Name, "budget_disruption_kind": ref.Kind})
	PruneBudgetStatusMetrics(ref)
}

// UpdateADBMetrics update metrics for an ADB
func UpdateADBMetrics(ref nodedisruptionv1alpha1.NamespacedName, adb *nodedisruptionv1alpha1.ApplicationDisruptionBudget) {
	DisruptionBudgetMaxDisruptions.WithLabelValues(ref.Namespace, ref.Name, ref.Kind).Set(float64(adb.Spec.MaxDisruptions))
	UpdateBudgetStatusMetrics(ref, adb.Status)
}

// MapFuncBuilder returns a MapFunc that is used to dispatch reconcile requests to
// budgets when an event is triggered by one of their matching object
func (r *ApplicationDisruptionBudgetReconciler) MapFuncBuilder() handler.MapFunc {
	// Look for all ADBs in the namespace, then see if they match the object
	return func(ctx context.Context, object client.Object) (requests []reconcile.Request) {
		adbs := nodedisruptionv1alpha1.ApplicationDisruptionBudgetList{}
		err := r.Client.List(ctx, &adbs, &client.ListOptions{Namespace: object.GetNamespace()})
		if err != nil {
			// We cannot return an error so at least it should be logged
			logger := log.FromContext(context.Background())
			logger.Error(err, "Could not list adbs in watch function")
			return requests
		}

		for _, adb := range adbs.Items {
			if adb.SelectorMatchesObject(object) {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      adb.Name,
						Namespace: adb.Namespace,
					},
				})
			}
		}
		return requests
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationDisruptionBudgetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&nodedisruptionv1alpha1.ApplicationDisruptionBudget{}).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(r.MapFuncBuilder()),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(
			&corev1.PersistentVolumeClaim{},
			handler.EnqueueRequestsFromMapFunc(r.MapFuncBuilder()),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(
			&nodedisruptionv1alpha1.NodeDisruption{},
			handler.EnqueueRequestsFromMapFunc(r.MapFuncBuilder()),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Complete(r)
}

type ApplicationDisruptionBudgetResolver struct {
	ApplicationDisruptionBudget *nodedisruptionv1alpha1.ApplicationDisruptionBudget
	Client                      client.Client
	Resolver                    resolver.Resolver
}

// Sync ensure the budget's status is up to date
func (r *ApplicationDisruptionBudgetResolver) Sync(ctx context.Context) error {
	nodeNames, err := r.GetSelectedNodes(ctx)
	if err != nil {
		return err
	}

	nodes := resolver.NodeSetToStringList(nodeNames)

	disruptionCount, disruptions, err := r.ResolveDisruption(ctx)
	if err != nil {
		return err
	}

	r.ApplicationDisruptionBudget.Status.WatchedNodes = nodes
	r.ApplicationDisruptionBudget.Status.CurrentDisruptions = disruptionCount
	r.ApplicationDisruptionBudget.Status.DisruptionsAllowed = r.ApplicationDisruptionBudget.Spec.MaxDisruptions - disruptionCount
	r.ApplicationDisruptionBudget.Status.Disruptions = disruptions
	return nil
}

// Check if the budget would be impacted by an operation on the provided set of nodes
func (r *ApplicationDisruptionBudgetResolver) IsImpacted(disruptedNodes resolver.NodeSet) bool {
	watchedNodes := resolver.NewNodeSetFromStringList(r.ApplicationDisruptionBudget.Status.WatchedNodes)
	return watchedNodes.Intersection(disruptedNodes).Len() > 0
}

// Return the number of disruption allowed considering a list of current node disruptions
func (r *ApplicationDisruptionBudgetResolver) TolerateDisruption(_ resolver.NodeSet) bool {
	return r.ApplicationDisruptionBudget.Status.DisruptionsAllowed-1 >= 0
}

func (r *ApplicationDisruptionBudgetResolver) UpdateStatus(ctx context.Context) error {
	return r.Client.Status().Update(ctx, r.ApplicationDisruptionBudget.DeepCopy(), []client.SubResourceUpdateOption{}...)
}

func (r *ApplicationDisruptionBudgetResolver) GetNamespacedName() nodedisruptionv1alpha1.NamespacedName {
	return nodedisruptionv1alpha1.NamespacedName{
		Namespace: r.ApplicationDisruptionBudget.Namespace,
		Name:      r.ApplicationDisruptionBudget.Name,
		Kind:      r.ApplicationDisruptionBudget.Kind,
	}
}

// Call a lifecycle hook in order to synchronously validate a Node Disruption
func (r *ApplicationDisruptionBudgetResolver) CallHealthHook(ctx context.Context, nd nodedisruptionv1alpha1.NodeDisruption) error {
	if r.ApplicationDisruptionBudget.Spec.HealthHook.URL == "" {
		return nil
	}

	client := &http.Client{}
	headers := make(map[string][]string, 1)

	data, err := json.Marshal(nd)
	if err != nil {
		return err
	}

	namespacedName := r.GetNamespacedName()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.ApplicationDisruptionBudget.Spec.HealthHook.URL, bytes.NewReader(data))
	if err != nil {
		DisruptionBudgetCheckHealthHookErrorTotal.WithLabelValues(namespacedName.Namespace, namespacedName.Name, namespacedName.Kind).Inc()
		return err
	}

	headers["Content-Type"] = []string{"application/json"}

	req.Header = headers

	resp, err := client.Do(req)
	if err != nil {
		DisruptionBudgetCheckHealthHookErrorTotal.WithLabelValues(namespacedName.Namespace, namespacedName.Name, namespacedName.Kind).Inc()
		return err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		DisruptionBudgetCheckHealthHookErrorTotal.WithLabelValues(namespacedName.Namespace, namespacedName.Name, namespacedName.Kind).Inc()
		return err
	}

	DisruptionBudgetCheckHealthHookStatusCodeTotal.WithLabelValues(namespacedName.Namespace, namespacedName.Name, namespacedName.Kind, strconv.Itoa(resp.StatusCode)).Inc()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("http server responded with non 2XX status code: %s", string(body))
}

func (r *ApplicationDisruptionBudgetResolver) GetSelectedNodes(ctx context.Context) (resolver.NodeSet, error) {
	nodeNames := resolver.NodeSet{}

	nodesFromPods, err := r.Resolver.GetNodesFromNamespacedPodSelector(ctx, r.ApplicationDisruptionBudget.Spec.PodSelector, r.ApplicationDisruptionBudget.Namespace)
	if err != nil {
		return nodeNames, err
	}
	nodesFromPVCs, err := r.Resolver.GetNodesFromNamespacedPVCSelector(ctx, r.ApplicationDisruptionBudget.Spec.PVCSelector, r.ApplicationDisruptionBudget.Namespace)
	if err != nil {
		return nodeNames, err
	}

	return nodesFromPods.Union(nodesFromPVCs), nil
}

func (r *ApplicationDisruptionBudgetResolver) ResolveDisruption(ctx context.Context) (int, []nodedisruptionv1alpha1.Disruption, error) {
	disruptions := []nodedisruptionv1alpha1.Disruption{}
	selectedNodes, err := r.GetSelectedNodes(ctx)
	if err != nil {
		return 0, disruptions, err
	}

	disruptionCount := 0

	opts := []client.ListOption{}
	nodeDisruptions := &nodedisruptionv1alpha1.NodeDisruptionList{}

	err = r.Client.List(ctx, nodeDisruptions, opts...)
	if err != nil {
		return 0, disruptions, err
	}

	for _, nd := range nodeDisruptions.Items {
		impactedNodes, err := r.Resolver.GetNodeFromNodeSelector(ctx, nd.Spec.NodeSelector)
		if err != nil {
			return 0, disruptions, err
		}

		if selectedNodes.Intersection(impactedNodes).Len() > 0 {
			if nd.Status.State == nodedisruptionv1alpha1.Granted {
				disruptionCount++
			}
			disruptions = append(disruptions, nodedisruptionv1alpha1.Disruption{
				Name:  nd.Name,
				State: string(nd.Status.State),
			})
		}
	}
	return disruptionCount, disruptions, nil
}
