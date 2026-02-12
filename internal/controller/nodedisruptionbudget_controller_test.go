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
)

// +kubebuilder:docs-gen:collapse=Imports
var _ = Describe("NodeDisruptionBudget controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		NDBname      = "test-ndb"
		NDBNamespace = "test-ndb"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	var (
		_, cancelFn = context.WithCancel(context.Background())
	)

	Context("In a cluster with several nodes", func() {
		ctx := context.Background()

		Context("With reconciler with default config", Ordered, func() {
			BeforeAll(func() {
				cancelFn = startReconcilerWithConfig(NodeDisruptionReconcilerConfig{
					RejectEmptyNodeDisruption: false,
					RetryInterval:             time.Second * 1,
				})
			})

			AfterAll(func() {
				cancelFn()
			})

			AfterEach(func() {
				clearAllNodeDisruptionResources()
			})

			When("Nodes changes in the cluster", func() {
				It("it updates the NDB", func() {
					By("creating a budget that accepts one disruption")
					ndb := &nodedisruptionv1alpha1.NodeDisruptionBudget{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "nodedisruption.criteo.com/v1alpha1",
							Kind:       "NodeDisruptionBudget",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      NDBname,
							Namespace: NDBNamespace,
						},
						Spec: nodedisruptionv1alpha1.NodeDisruptionBudgetSpec{
							NodeSelector:      metav1.LabelSelector{MatchLabels: nodeLabels1},
							MaxDisruptedNodes: 1,
						},
					}
					Expect(k8sClient.Create(ctx, ndb)).Should(Succeed())

					By("checking the NodeDisruptionBudget in synchronized")
					NDBLookupKey := types.NamespacedName{Name: NDBname, Namespace: NDBNamespace}
					createdNDB := &nodedisruptionv1alpha1.NodeDisruptionBudget{}
					Eventually(func() []string {
						err := k8sClient.Get(ctx, NDBLookupKey, createdNDB)
						Expect(err).Should(Succeed())
						return createdNDB.Status.WatchedNodes
					}, timeout, interval).Should(Equal([]string{"node1", "node2"}))

					By("Adding Node")
					labels := map[string]string{
						"testselect":             "test1",
						"kubernetes.io/hostname": "node4",
					}
					node4 := &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "node4",
							Labels: labels,
						},
					}
					Expect(k8sClient.Create(ctx, node4)).Should(Succeed())

					By("checking the NodeDisruptionBudget updated the status")
					Eventually(func() []string {
						err := k8sClient.Get(ctx, NDBLookupKey, createdNDB)
						Expect(err).Should(Succeed())
						return createdNDB.Status.WatchedNodes
					}, timeout, interval).Should(Equal([]string{"node1", "node2", "node4"}))

					By("Removing Node")
					Expect(k8sClient.Delete(ctx, node4)).Should(Succeed())

					By("checking the NodeDisruptionBudget updated the status")
					Eventually(func() []string {
						err := k8sClient.Get(ctx, NDBLookupKey, createdNDB)
						Expect(err).Should(Succeed())
						return createdNDB.Status.WatchedNodes
					}, timeout, interval).Should(Equal([]string{"node1", "node2"}))
				})
			})

			When("ND are created", func() {
				It("it updates the NDB", func() {
					By("creating a budget that accepts one disruption")
					ndb := &nodedisruptionv1alpha1.NodeDisruptionBudget{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "nodedisruption.criteo.com/v1alpha1",
							Kind:       "NodeDisruptionBudget",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      NDBname,
							Namespace: NDBNamespace,
						},
						Spec: nodedisruptionv1alpha1.NodeDisruptionBudgetSpec{
							NodeSelector:      metav1.LabelSelector{MatchLabels: nodeLabels1},
							MaxDisruptedNodes: 10,
						},
					}
					Expect(k8sClient.Create(ctx, ndb)).Should(Succeed())

					By("checking the NodeDisruptionBudget in synchronized")
					NDBLookupKey := types.NamespacedName{Name: NDBname, Namespace: NDBNamespace}
					createdNDB := &nodedisruptionv1alpha1.NodeDisruptionBudget{}
					Eventually(func() []string {
						err := k8sClient.Get(ctx, NDBLookupKey, createdNDB)
						Expect(err).Should(Succeed())
						return createdNDB.Status.WatchedNodes
					}, timeout, interval).Should(Equal([]string{"node1", "node2"}))

					By("Adding Node Disruption")
					disruption := &nodedisruptionv1alpha1.NodeDisruption{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "nodedisruption.criteo.com/v1alpha1",
							Kind:       "NodeDisruption",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-ndb-1",
							Namespace: "",
						},
						Spec: nodedisruptionv1alpha1.NodeDisruptionSpec{
							NodeSelector: metav1.LabelSelector{MatchLabels: nodeLabels1},
							Type:         "maintenance",
						},
					}
					Expect(k8sClient.Create(ctx, disruption.DeepCopy())).Should(Succeed())

					By("checking the NodeDisruptionBudget updated the status")
					Eventually(func() []nodedisruptionv1alpha1.Disruption {
						err := k8sClient.Get(ctx, NDBLookupKey, createdNDB)
						Expect(err).Should(Succeed())
						return createdNDB.Status.Disruptions
					}, timeout, interval).Should(Equal([]nodedisruptionv1alpha1.Disruption{{
						Name:  "test-ndb-1",
						State: "granted",
					}}))

				})
			})

			When("NodeSelector is empty", func() {
				It("watches no nodes", func() {
					By("creating a budget with empty NodeSelector")
					ndb := &nodedisruptionv1alpha1.NodeDisruptionBudget{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "nodedisruption.criteo.com/v1alpha1",
							Kind:       "NodeDisruptionBudget",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      NDBname,
							Namespace: NDBNamespace,
						},
						Spec: nodedisruptionv1alpha1.NodeDisruptionBudgetSpec{
							NodeSelector:        metav1.LabelSelector{},
							MaxDisruptedNodes:   10,
							MinUndisruptedNodes: 0,
						},
					}
					Expect(k8sClient.Create(ctx, ndb)).Should(Succeed())

					By("checking the NodeDisruptionBudget watches no nodes")
					NDBLookupKey := types.NamespacedName{Name: NDBname, Namespace: NDBNamespace}
					createdNDB := &nodedisruptionv1alpha1.NodeDisruptionBudget{}
					Eventually(func() []string {
						err := k8sClient.Get(ctx, NDBLookupKey, createdNDB)
						Expect(err).Should(Succeed())
						return createdNDB.Status.WatchedNodes
					}, timeout, interval).Should(BeEmpty())

					By("verifying disruptions calculation with no nodes")
					Eventually(func() int {
						err := k8sClient.Get(ctx, NDBLookupKey, createdNDB)
						Expect(err).Should(Succeed())
						return createdNDB.Status.DisruptionsAllowed
					}, timeout, interval).Should(Equal(0))
				})
			})

		})

	})
})
