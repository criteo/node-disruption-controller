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
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"reflect"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nodedisruptionv1alpha1 "github.com/criteo/node-disruption-controller/api/v1alpha1"
	"github.com/criteo/node-disruption-controller/pkg/resolver"

	"github.com/golang-collections/collections/set"
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
	adb := &nodedisruptionv1alpha1.ApplicationDisruptionBudget{}
	err := r.Client.Get(ctx, req.NamespacedName, adb)

	if err != nil {
		if errors.IsNotFound(err) {
			// If the ressource was not found, nothing has to be done
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

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
		err = resolver.UpdateStatus(ctx)
	}

	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationDisruptionBudgetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&nodedisruptionv1alpha1.ApplicationDisruptionBudget{}).
		Complete(r)
}

type ApplicationDisruptionBudgetResolver struct {
	ApplicationDisruptionBudget *nodedisruptionv1alpha1.ApplicationDisruptionBudget
	Client                      client.Client
	Resolver                    resolver.Resolver
}

// Sync ensure the budget's status is up to date
func (r *ApplicationDisruptionBudgetResolver) Sync(ctx context.Context) error {
	node_names, err := r.GetSelectedNodes(ctx)
	if err != nil {
		return err
	}

	nodes := NodeSetToStringList(node_names)

	disruption_nr, err := r.ResolveDisruption(ctx)
	if err != nil {
		return err
	}

	r.ApplicationDisruptionBudget.Status.WatchedNodes = nodes
	r.ApplicationDisruptionBudget.Status.CurrentDisruptions = disruption_nr
	r.ApplicationDisruptionBudget.Status.DisruptionsAllowed = r.ApplicationDisruptionBudget.Spec.MaxDisruptions - disruption_nr
	return nil
}

// Check if the budget would be impacted by an operation on the provided set of nodes
func (r *ApplicationDisruptionBudgetResolver) IsImpacted(nd NodeDisruption) bool {
	watched_nodes := NewNodeSetFromStringList(r.ApplicationDisruptionBudget.Status.WatchedNodes)
	return watched_nodes.Intersection(nd.ImpactedNodes).Len() > 0
}

// Return the number of disruption allowed considering a list of current node disruptions
func (r *ApplicationDisruptionBudgetResolver) TolerateDisruption(NodeDisruption) bool {
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

// Check health make a synchronous health check on the underlying ressource of a budget
func (r *ApplicationDisruptionBudgetResolver) CheckHealth(context.Context) error {
	if r.ApplicationDisruptionBudget.Spec.HealthURL == nil {
		return nil
	}
	resp, err := http.Get(*r.ApplicationDisruptionBudget.Spec.HealthURL)
	if err != nil {
		log.Fatalln(err)
		return err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	} else {
		return fmt.Errorf("http server responded with non 2XX status code: %s", string(body))
	}
}

func (r *ApplicationDisruptionBudgetResolver) GetSelectedNodes(ctx context.Context) (*set.Set, error) {
	node_names := set.New()

	nodes_from_pods, err := r.Resolver.GetNodesFromNamespacedPodSelector(ctx, r.ApplicationDisruptionBudget.Spec.PodSelector, r.ApplicationDisruptionBudget.Namespace)
	if err != nil {
		return node_names, err
	}
	nodes_from_PVCs, err := r.Resolver.GetNodesFromNamespacedPVCSelector(ctx, r.ApplicationDisruptionBudget.Spec.PVCSelector, r.ApplicationDisruptionBudget.Namespace)
	if err != nil {
		return node_names, err
	}

	return nodes_from_pods.Union(nodes_from_PVCs), nil
}

func (r *ApplicationDisruptionBudgetResolver) ResolveDisruption(ctx context.Context) (int, error) {
	selected_nodes, err := r.GetSelectedNodes(ctx)
	if err != nil {
		return 0, err
	}

	disruptions := 0

	opts := []client.ListOption{}
	node_disruptions := &nodedisruptionv1alpha1.NodeDisruptionList{}

	err = r.Client.List(ctx, node_disruptions, opts...)
	if err != nil {
		return 0, err
	}

	for _, nd := range node_disruptions.Items {
		if nd.Status.State != nodedisruptionv1alpha1.Granted {
			continue
		}
		node_disruption_resolver := NodeDisruptionResolver{
			NodeDisruption: &nd,
			Client:         r.Client,
		}
		disruption, err := node_disruption_resolver.GetDisruption(ctx)
		if err != nil {
			return 0, err
		}
		if selected_nodes.Intersection(disruption.ImpactedNodes).Len() > 0 {
			disruptions += 1
		}
	}
	return disruptions, nil
}
