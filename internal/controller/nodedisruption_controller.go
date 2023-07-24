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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	nodedisruptionv1alpha1 "github.com/criteo/node-disruption-controller/api/v1alpha1"
	"github.com/golang-collections/collections/set"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeDisruptionReconciler reconciles a NodeDisruption object
type NodeDisruptionReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=nodedisruption.criteo.com,resources=nodedisruptions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nodedisruption.criteo.com,resources=nodedisruptions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nodedisruption.criteo.com,resources=nodedisruptions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the NodeDisruption object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *NodeDisruptionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Start reconcile of NodeDisruption")

	nd := &nodedisruptionv1alpha1.NodeDisruption{}
	err := r.Client.Get(ctx, req.NamespacedName, nd)

	if err != nil {
		return ctrl.Result{}, err
	}

	if nd.Spec.State == nodedisruptionv1alpha1.Pending {
		nd.Spec.State = nodedisruptionv1alpha1.Processing
		err = r.Update(ctx, nd.DeepCopy(), []client.UpdateOption{}...)
		if err != nil {
			return ctrl.Result{}, err
		}

		nd := &nodedisruptionv1alpha1.NodeDisruption{}
		err := r.Client.Get(ctx, req.NamespacedName, nd)

		if err != nil {
			return ctrl.Result{}, err
		}
	}

	resolver := NodeDisruptionResolver{
		NodeDisruption: nd,
		Client:         r.Client,
	}

	nodes, err := resolver.ResolveNodes(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	all_allowed := true
	reject_reason := ""

	// TODO: refactor this whole part once the interface is properly defined
	// Check ADB
	opts := []client.ListOption{}
	application_disruption_budgets := &nodedisruptionv1alpha1.ApplicationDisruptionBudgetList{}

	err = r.Client.List(ctx, application_disruption_budgets, opts...)
	if err != nil {
		return ctrl.Result{}, err
	}

	disrupted_adb := []nodedisruptionv1alpha1.NamespacedName{}
	for _, adb := range application_disruption_budgets.Items {
		adb_resolver := ApplicationDisruptionBudgetResolver{
			ApplicationDisruptionBudget: &adb,
			Client:                      r.Client,
		}
		impacted, allowed, err := adb_resolver.AllowDisruption(ctx, nodes)
		if err != nil {
			return ctrl.Result{}, err
		}

		if impacted {
			disrupted_adb = append(disrupted_adb, nodedisruptionv1alpha1.NamespacedName{Name: adb.Name, Namespace: adb.Namespace})

			if !allowed {
				all_allowed = false
				reject_reason = fmt.Sprintf("%s (in %s) doesn't allow more disruption", adb.Name, adb.Namespace)
				break
			}
		}
		err = adb_resolver.HealthCheck(ctx, *nd)
		if err != nil {
			all_allowed = false
			reject_reason = fmt.Sprintf("%s (in %s) is unhealthy: %s", adb.Name, adb.Namespace, err)
			break
		}
	}

	// Check NDB
	node_disruption_budgets := &nodedisruptionv1alpha1.NodeDisruptionBudgetList{}

	err = r.Client.List(ctx, node_disruption_budgets, opts...)
	if err != nil {
		return ctrl.Result{}, err
	}

	disrupted_ndb := []nodedisruptionv1alpha1.NamespacedName{}
	for _, ndb := range node_disruption_budgets.Items {
		ndb_resolver := NodeDisruptionBudgetResolver{
			NodeDisruptionBudget: &ndb,
			Client:               r.Client,
		}
		impacted, allowed, err := ndb_resolver.AllowDisruption(ctx, nodes)
		if err != nil {
			return ctrl.Result{}, err
		}

		if impacted {
			disrupted_ndb = append(disrupted_ndb, nodedisruptionv1alpha1.NamespacedName{Name: ndb.Name, Namespace: ndb.Namespace})

			if !allowed {
				all_allowed = false
				reject_reason = fmt.Sprintf("%s (in %s) doesn't allow more disruption", ndb.Name, ndb.Namespace)
				break
			}
		}
	}

	// Create a slice to store the set elements
	disrupted_nodes := make([]string, 0, nodes.Len())

	// Iterate over the set and append elements to the slice
	nodes.Do(func(item interface{}) {
		disrupted_nodes = append(disrupted_nodes, item.(string))
	})

	nd.Status.DisruptedADB = disrupted_adb
	nd.Status.DisruptedNDB = disrupted_ndb
	nd.Status.DisruptedNodes = disrupted_nodes

	if nd.Spec.State == nodedisruptionv1alpha1.Processing {
		if all_allowed {
			nd.Spec.State = nodedisruptionv1alpha1.Granted
		} else {
			nd.Spec.State = nodedisruptionv1alpha1.Rejected
			r.Recorder.Event(nd, "Normal", "Rejected", reject_reason)
		}
	}

	err = r.Update(ctx, nd.DeepCopy(), []client.UpdateOption{}...)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.Status().Update(ctx, nd, []client.SubResourceUpdateOption{}...)
	if err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Reconcilation successful", "state", nd.Spec.State)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeDisruptionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("node-disruption-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&nodedisruptionv1alpha1.NodeDisruption{}).
		Complete(r)
}

type NodeDisruptionResolver struct {
	NodeDisruption *nodedisruptionv1alpha1.NodeDisruption
	Client         client.Client
}

// Resolve the nodes impacted by the NodeDisruption
func (ndr *NodeDisruptionResolver) ResolveNodes(ctx context.Context) (*set.Set, error) {
	node_names := set.New()
	selector, err := metav1.LabelSelectorAsSelector(&ndr.NodeDisruption.Spec.NodeSelector)
	if err != nil {
		return node_names, err
	}
	opts := []client.ListOption{
		client.MatchingLabelsSelector{Selector: selector},
	}
	nodes := &corev1.NodeList{}
	err = ndr.Client.List(ctx, nodes, opts...)
	if err != nil {
		return node_names, err
	}

	for _, node := range nodes.Items {
		node_names.Insert(node.Name)
	}
	return node_names, nil
}
