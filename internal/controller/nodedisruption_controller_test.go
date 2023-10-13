/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// +kubebuilder:docs-gen:collapse=Apache License

package controller

import (
	"context"
	"time"

	nodedisruptionv1alpha1 "github.com/criteo/node-disruption-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	ctrl "sigs.k8s.io/controller-runtime"
)

// +kubebuilder:docs-gen:collapse=Imports

func clearAllNodeDisruptionRessources() {
	_ = k8sClient.DeleteAllOf(context.Background(), &nodedisruptionv1alpha1.NodeDisruption{})
	_ = k8sClient.DeleteAllOf(context.Background(), &nodedisruptionv1alpha1.NodeDisruptionBudget{})
	_ = k8sClient.DeleteAllOf(context.Background(), &nodedisruptionv1alpha1.ApplicationDisruptionBudget{})
}

func start_reconciler_with_config(config NodeDisruptionReconcilerConfig) (cancel_fn context.CancelFunc) {
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		MetricsBindAddress: "127.0.0.1:8081",
		PprofBindAddress:   "127.0.0.1:8082",
		Scheme:             scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())
	err = (&NodeDisruptionReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
		Config: config,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	manager_ctx, cancel_fn := context.WithCancel(context.Background())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(manager_ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
	return cancel_fn
}

var _ = Describe("NodeDisruption controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		NDName      = "test-nodedisruption"
		NDNamespace = "default"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	var (
		node_labels1 = map[string]string{
			"testselect": "test1",
		}
		node_labels2 = map[string]string{
			"testselect": "test2",
		}
		NDLookupKey = types.NamespacedName{Name: NDName, Namespace: NDNamespace}

		_, cancel_fn = context.WithCancel(context.Background())
	)

	Context("In a cluster with several nodes", func() {
		ctx := context.Background()
		It("Create the nodes", func() {
			By("Adding Nodes")
			node1 := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "node1",
					Labels: node_labels1,
				},
			}
			Expect(k8sClient.Create(ctx, node1)).Should(Succeed())
			node2 := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "node2",
					Labels: node_labels1,
				},
			}
			Expect(k8sClient.Create(ctx, node2)).Should(Succeed())
			node3 := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "node3",
					Labels: node_labels2,
				},
			}
			Expect(k8sClient.Create(ctx, node3)).Should(Succeed())
		})

		Context("With reconciler with default config", Ordered, func() {
			BeforeAll(func() {
				cancel_fn = start_reconciler_with_config(NodeDisruptionReconcilerConfig{
					RejectEmptyNodeDisruption: false,
					RetryInterval:             time.Second * 1,
				})
			})

			AfterAll(func() {
				cancel_fn()
			})

			defer AfterEach(func() {
				clearAllNodeDisruptionRessources()
			})

			When("there are no budgets in the cluster", func() {
				It("grants the node disruption", func() {
					By("creating a new NodeDisruption")
					disruption := &nodedisruptionv1alpha1.NodeDisruption{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "nodedisruption.criteo.com/v1alpha1",
							Kind:       "NodeDisruption",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      NDName,
							Namespace: NDNamespace,
						},
						Spec: nodedisruptionv1alpha1.NodeDisruptionSpec{
							NodeSelector: metav1.LabelSelector{MatchLabels: node_labels1},
						},
					}
					Expect(k8sClient.Create(ctx, disruption.DeepCopy())).Should(Succeed())

					NDLookupKey := types.NamespacedName{Name: NDName, Namespace: NDNamespace}
					createdDisruption := &nodedisruptionv1alpha1.NodeDisruption{}

					By("checking the NodeDisruption is being granted")
					Eventually(func() nodedisruptionv1alpha1.NodeDisruptionState {
						err := k8sClient.Get(ctx, NDLookupKey, createdDisruption)
						if err != nil {
							panic("should be able to get")
						}
						return createdDisruption.Status.State
					}, timeout, interval).Should(Equal(nodedisruptionv1alpha1.Granted))

					By("ensure the disrupted node selector is correct")
					Expect(createdDisruption.Status.DisruptedNodes).Should(Equal([]string{"node1", "node2"}))
				})
			})

			Context("there is a budget that doesn't support any disruption", func() {
				It("rejects the node disruption", func() {
					By("creating a budget that rejects everything")
					ndb := &nodedisruptionv1alpha1.NodeDisruptionBudget{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "nodedisruption.criteo.com/v1alpha1",
							Kind:       "NodeDisruptionBudget",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Spec: nodedisruptionv1alpha1.NodeDisruptionBudgetSpec{
							NodeSelector:      metav1.LabelSelector{MatchLabels: node_labels1},
							MaxDisruptedNodes: 0,
						},
					}
					Expect(k8sClient.Create(ctx, ndb)).Should(Succeed())

					By("creating a new NodeDisruption")
					disruption := &nodedisruptionv1alpha1.NodeDisruption{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "nodedisruption.criteo.com/v1alpha1",
							Kind:       "NodeDisruption",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      NDName,
							Namespace: NDNamespace,
						},
						Spec: nodedisruptionv1alpha1.NodeDisruptionSpec{
							NodeSelector: metav1.LabelSelector{MatchLabels: node_labels1},
						},
					}
					Expect(k8sClient.Create(ctx, disruption.DeepCopy())).Should(Succeed())

					By("checking the NodeDisruption is being rejected")
					createdDisruption := &nodedisruptionv1alpha1.NodeDisruption{}

					Eventually(func() nodedisruptionv1alpha1.NodeDisruptionState {
						err := k8sClient.Get(ctx, NDLookupKey, createdDisruption)
						if err != nil {
							panic("should be able to get")
						}
						return createdDisruption.Status.State
					}, timeout, interval).Should(Equal(nodedisruptionv1alpha1.Rejected))
				})
			})

			When("there is a budget that doesn't support any disruption and retry is activated", func() {
				It("rejects the node disruption", func() {
					By("creating a budget that rejects everything")
					ndb := &nodedisruptionv1alpha1.NodeDisruptionBudget{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "nodedisruption.criteo.com/v1alpha1",
							Kind:       "NodeDisruptionBudget",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Spec: nodedisruptionv1alpha1.NodeDisruptionBudgetSpec{
							NodeSelector:      metav1.LabelSelector{MatchLabels: node_labels1},
							MaxDisruptedNodes: 0,
						},
					}
					Expect(k8sClient.Create(ctx, ndb)).Should(Succeed())

					By("creating a new NodeDisruption with retry enabled")
					disruption := &nodedisruptionv1alpha1.NodeDisruption{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "nodedisruption.criteo.com/v1alpha1",
							Kind:       "NodeDisruption",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      NDName,
							Namespace: NDNamespace,
						},
						Spec: nodedisruptionv1alpha1.NodeDisruptionSpec{
							NodeSelector: metav1.LabelSelector{MatchLabels: node_labels1},
							Retry: nodedisruptionv1alpha1.RetrySpec{
								Enabled: true,
							},
						},
					}
					Expect(k8sClient.Create(ctx, disruption.DeepCopy())).Should(Succeed())

					By("checking the NodeDisruption is staying pending with retry")
					createdDisruption := &nodedisruptionv1alpha1.NodeDisruption{}

					Eventually(func() bool {
						err := k8sClient.Get(ctx, NDLookupKey, createdDisruption)
						if err != nil {
							panic("should be able to get")
						}
						return !createdDisruption.Status.NextRetryDate.IsZero()
					}, timeout, interval).Should(BeTrue())

					By("staying Pending")
					Expect(createdDisruption.Status.State).Should(Equal(nodedisruptionv1alpha1.Pending))

					By("making the ndb accept some disruption")
					Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test"}, ndb)).Should(Succeed())
					ndb.Spec.MaxDisruptedNodes = 2
					Expect(k8sClient.Update(ctx, ndb)).Should(Succeed())

					By("switching to granted after retry")
					Eventually(func() nodedisruptionv1alpha1.NodeDisruptionState {
						err := k8sClient.Get(ctx, NDLookupKey, createdDisruption)
						if err != nil {
							panic("should be able to get")
						}
						return createdDisruption.Status.State
					}, timeout, interval).Should(Equal(nodedisruptionv1alpha1.Granted))
				})
			})

			When("a node disruption's deadline is in the past", func() {
				It("It rejects the node disruption", func() {
					By("creating a new NodeDisruption with retry enabled and an old deadline")
					disruption := &nodedisruptionv1alpha1.NodeDisruption{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "nodedisruption.criteo.com/v1alpha1",
							Kind:       "NodeDisruption",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      NDName,
							Namespace: NDNamespace,
						},
						Spec: nodedisruptionv1alpha1.NodeDisruptionSpec{
							NodeSelector: metav1.LabelSelector{MatchLabels: node_labels1},
							Retry: nodedisruptionv1alpha1.RetrySpec{
								Enabled: true,
								// Deadline is 1s in the past
								Deadline: metav1.Time{Time: time.Now().Add(-time.Second)},
							},
						},
					}
					Expect(k8sClient.Create(ctx, disruption.DeepCopy())).Should(Succeed())

					By("switching to rejected without retrying")
					Eventually(func() nodedisruptionv1alpha1.NodeDisruptionState {
						err := k8sClient.Get(ctx, NDLookupKey, disruption)
						if err != nil {
							panic("should be able to get")
						}
						return disruption.Status.State
					}, timeout, interval).Should(Equal(nodedisruptionv1alpha1.Rejected))
				})
			})
		})

		Context("If a NodeDisruption's nodeSelector does not match any node", func() {
			var (
				nodeLabelsNoMatch = map[string]string{"testselect": "nope"}
				createdDisruption = &nodedisruptionv1alpha1.NodeDisruption{}
			)

			BeforeEach(func() {
				disruption := &nodedisruptionv1alpha1.NodeDisruption{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "nodedisruption.criteo.com/v1alpha1",
						Kind:       "NodeDisruption",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      NDName,
						Namespace: NDNamespace,
					},
					Spec: nodedisruptionv1alpha1.NodeDisruptionSpec{
						NodeSelector: metav1.LabelSelector{MatchLabels: nodeLabelsNoMatch},
					},
				}
				Expect(k8sClient.Create(ctx, disruption.DeepCopy())).Should(Succeed())
			})
			AfterEach(func() {
				clearAllNodeDisruptionRessources()
				cancel_fn()
			})

			When("RejectEmptyNodeDisruption is disabled", func() {
				It("grants the NodeDisruption", func() {
					By("starting a reconciler with RejectEmptyNodeDisruption disabled")
					cancel_fn = start_reconciler_with_config(NodeDisruptionReconcilerConfig{
						RejectEmptyNodeDisruption: false,
						RetryInterval:             time.Second * 1,
					})
					By("checking if the Node Disruption is granted")
					Eventually(func() nodedisruptionv1alpha1.NodeDisruptionState {
						err := k8sClient.Get(ctx, NDLookupKey, createdDisruption)
						if err != nil {
							panic("should be able to get")
						}
						return createdDisruption.Status.State
					}, timeout, interval).Should(Equal(nodedisruptionv1alpha1.Granted))
				})
			})

			When("RejectEmptyNodeDisruption is enabled", func() {
				It("rejects the NodeDisruption", func() {
					By("starting a reconciler with RejectEmptyNodeDisruption enabled")
					cancel_fn = start_reconciler_with_config(NodeDisruptionReconcilerConfig{
						RejectEmptyNodeDisruption: true,
						RetryInterval:             time.Second * 1,
					})
					By("checking if the Node Disruption is rejected")
					Eventually(func() nodedisruptionv1alpha1.NodeDisruptionState {
						err := k8sClient.Get(ctx, NDLookupKey, createdDisruption)
						if err != nil {
							panic("should be able to get")
						}
						return createdDisruption.Status.State
					}, timeout, interval).Should(Equal(nodedisruptionv1alpha1.Rejected))
				})
			})

		})
	})
})
