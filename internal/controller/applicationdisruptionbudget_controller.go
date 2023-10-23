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
	nodeNames, err := r.GetSelectedNodes(ctx)
	if err != nil {
		return err
	}

	nodes := NodeSetToStringList(nodeNames)

	disruptionCount, err := r.ResolveDisruption(ctx)
	if err != nil {
		return err
	}

	r.ApplicationDisruptionBudget.Status.WatchedNodes = nodes
	r.ApplicationDisruptionBudget.Status.CurrentDisruptions = disruptionCount
	r.ApplicationDisruptionBudget.Status.DisruptionsAllowed = r.ApplicationDisruptionBudget.Spec.MaxDisruptions - disruptionCount
	return nil
}

// Check if the budget would be impacted by an operation on the provided set of nodes
func (r *ApplicationDisruptionBudgetResolver) IsImpacted(nd NodeDisruption) bool {
	watchedNodes := NewNodeSetFromStringList(r.ApplicationDisruptionBudget.Status.WatchedNodes)
	return watchedNodes.Intersection(nd.ImpactedNodes).Len() > 0
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
		return err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("http server responded with non 2XX status code: %s", string(body))
}

// Call a lifecycle hook in order to synchronously validate a Node Disruption
func (r *ApplicationDisruptionBudgetResolver) CallHealthHook(ctx context.Context, nd nodedisruptionv1alpha1.NodeDisruption) error {
	if r.ApplicationDisruptionBudget.Spec.HealthHook.URL == "" {
		return nil
	}

	client := &http.Client{}

	data, err := json.Marshal(nd)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.ApplicationDisruptionBudget.Spec.HealthHook.URL, bytes.NewReader(data))
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("http server responded with non 2XX status code: %s", string(body))
}

func (r *ApplicationDisruptionBudgetResolver) GetSelectedNodes(ctx context.Context) (*set.Set, error) {
	nodeNames := set.New()

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

func (r *ApplicationDisruptionBudgetResolver) ResolveDisruption(ctx context.Context) (int, error) {
	selectedNodes, err := r.GetSelectedNodes(ctx)
	if err != nil {
		return 0, err
	}

	disruptions := 0

	opts := []client.ListOption{}
	nodeDisruptions := &nodedisruptionv1alpha1.NodeDisruptionList{}

	err = r.Client.List(ctx, nodeDisruptions, opts...)
	if err != nil {
		return 0, err
	}

	for _, nd := range nodeDisruptions.Items {
		if nd.Status.State != nodedisruptionv1alpha1.Granted {
			continue
		}
		nodeDisruptionResolver := NodeDisruptionResolver{
			NodeDisruption: &nd,
			Client:         r.Client,
		}
		disruption, err := nodeDisruptionResolver.GetDisruption(ctx)
		if err != nil {
			return 0, err
		}
		if selectedNodes.Intersection(disruption.ImpactedNodes).Len() > 0 {
			disruptions++
		}
	}
	return disruptions, nil
}
