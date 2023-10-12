/*

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
)

// +kubebuilder:docs-gen:collapse=Imports

func clearAllNodeDisruptionRessources() {
	_ = k8sClient.DeleteAllOf(context.Background(), &nodedisruptionv1alpha1.NodeDisruption{})
	_ = k8sClient.DeleteAllOf(context.Background(), &nodedisruptionv1alpha1.NodeDisruptionBudget{})
	_ = k8sClient.DeleteAllOf(context.Background(), &nodedisruptionv1alpha1.ApplicationDisruptionBudget{})
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

		AfterEach(func() {
			clearAllNodeDisruptionRessources()
		})

		Context("When there are no budgets in the cluster", func() {
			It("It grants the node disruption", func() {
				By("By creating a new NodeDisruption")
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

				By("By checking the NodeDisruption is being granted")
				Eventually(func() nodedisruptionv1alpha1.NodeDisruptionState {
					err := k8sClient.Get(ctx, NDLookupKey, createdDisruption)
					if err != nil {
						panic("should be able to get")
					}
					return createdDisruption.Status.State
				}, timeout, interval).Should(Equal(nodedisruptionv1alpha1.Granted))

				By("By ensure the disrupted node selector is correct")
				Expect(createdDisruption.Status.DisruptedNodes).Should(Equal([]string{"node1", "node2"}))
			})
		})

		Context("When there is a budget that doesn't support any disription", func() {
			It("It rejects the node disruption", func() {
				By("By creating a budget that rejects everything")
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

				By("By creating a new NodeDisruption")
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

				By("By checking the NodeDisruption is being rejected")
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
			})
      // TODO: find how to change controller config
			// When("RejectEmptyNodeDisruption is enabled", func() {
			// 	It("rejects the NodeDisruption", func() {
			// 		Eventually(func() nodedisruptionv1alpha1.NodeDisruptionState {
			// 			err := k8sClient.Get(ctx, NDLookupKey, createdDisruption)
			// 			if err != nil {
			// 				panic("should be able to get")
			// 			}
			// 			return createdDisruption.Status.State
			// 		}, timeout, interval).Should(Equal(nodedisruptionv1alpha1.Rejected))
			// 	})
			// })
			When("RejectEmptyNodeDisruption is disabled", func() {
				It("grants the NodeDisruption", func() {
					Eventually(func() nodedisruptionv1alpha1.NodeDisruptionState {
						err := k8sClient.Get(ctx, NDLookupKey, createdDisruption)
						if err != nil {
							panic("should be able to get")
						}
						return createdDisruption.Status.State
					}, timeout, interval).Should(Equal(nodedisruptionv1alpha1.Granted))
				})
			})
		})
	})
})
