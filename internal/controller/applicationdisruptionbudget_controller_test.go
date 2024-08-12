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
	"fmt"
	"math/rand"
	"time"

	nodedisruptionv1alpha1 "github.com/criteo/node-disruption-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// +kubebuilder:docs-gen:collapse=Imports
var _ = Describe("ApplicationDisruptionBudget controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		ADBname = "test-adb"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	var (
		ADBNamespace = "xxx" // Ramdomized in BeforEach

		podLabels = map[string]string{
			"testselectpod": "testadb",
		}

		_, cancelFn = context.WithCancel(context.Background())
	)

	Context("In a cluster with several nodes", Ordered, func() {
		ctx := context.Background()
		BeforeEach(func() {
			// Using randomized namespace because test env doesn't support well deletion
			// see https://book.kubebuilder.io/reference/envtest.html#namespace-usage-limitation
			ADBNamespace = fmt.Sprintf("test-%d", rand.Int()%10000)

			By("Creating the namespace")
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ADBNamespace,
					Namespace: ADBNamespace,
				},
			}
			Expect(k8sClient.Create(ctx, ns)).Should(Succeed())
		})

		AfterEach(func() {
			clearAllNodeDisruptionResources()
		})

		BeforeAll(func() {
			cancelFn = startReconcilerWithConfig(NodeDisruptionReconcilerConfig{
				RejectEmptyNodeDisruption: false,
				RetryInterval:             time.Second * 1,
			})
		})

		AfterAll(func() {
			cancelFn()
		})

		When("Pods or PVC changes", func() {
			It("updates the budget status", func() {
				By("Adding Pod")
				pod1 := newPod("podadb1", ADBNamespace, "node1", podLabels)
				Expect(k8sClient.Create(ctx, &pod1)).Should(Succeed())

				By("creating a budget that accepts one disruption")
				ndb := &nodedisruptionv1alpha1.ApplicationDisruptionBudget{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "nodedisruption.criteo.com/v1alpha1",
						Kind:       "ApplicationDisruptionBudget",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      ADBname,
						Namespace: ADBNamespace,
					},
					Spec: nodedisruptionv1alpha1.ApplicationDisruptionBudgetSpec{
						PodSelector:    metav1.LabelSelector{MatchLabels: podLabels},
						PVCSelector:    metav1.LabelSelector{MatchLabels: podLabels},
						MaxDisruptions: 1,
					},
				}
				Expect(k8sClient.Create(ctx, ndb)).Should(Succeed())

				By("checking the ApplicationDisruptionBudget in synchronized")
				ADBLookupKey := types.NamespacedName{Name: ADBname, Namespace: ADBNamespace}
				createdADB := &nodedisruptionv1alpha1.ApplicationDisruptionBudget{}
				Eventually(func() []string {
					err := k8sClient.Get(ctx, ADBLookupKey, createdADB)
					Expect(err).Should(Succeed())
					return createdADB.Status.WatchedNodes
				}, timeout, interval).Should(Equal([]string{"node1"}))

				By("Adding more Pods")
				pod2 := newPod("podadb2", ADBNamespace, "node2", podLabels)
				Expect(k8sClient.Create(ctx, &pod2)).Should(Succeed())

				By("checking the ApplicationDisruptionBudget updated the status")
				Eventually(func() []string {
					err := k8sClient.Get(ctx, ADBLookupKey, createdADB)
					Expect(err).Should(Succeed())
					return createdADB.Status.WatchedNodes
				}, timeout, interval).Should(Equal([]string{"node1", "node2"}))

				By("Adding a new PVC")
				pvc3 := newPVC("pvc3", ADBNamespace, "node3-pv-local", podLabels)
				Expect(k8sClient.Create(ctx, pvc3.DeepCopy())).Should(Succeed())
				Expect(k8sClient.Status().Update(ctx, pvc3.DeepCopy())).Should(Succeed())

				By("checking the ApplicationDisruptionBudget updated the status")
				Eventually(func() []string {
					err := k8sClient.Get(ctx, ADBLookupKey, createdADB)
					Expect(err).Should(Succeed())
					return createdADB.Status.WatchedNodes
				}, timeout, interval).Should(Equal([]string{"node1", "node2", "node3"}))
			})
		})

		When("Node disruption is created", func() {
			It("updates the budget status", func() {
				By("Adding pods and PVCs")
				pod1 := newPod("podadb1", ADBNamespace, "node1", podLabels)
				Expect(k8sClient.Create(ctx, &pod1)).Should(Succeed())
				pod2 := newPod("podadb2", ADBNamespace, "node2", podLabels)
				Expect(k8sClient.Create(ctx, &pod2)).Should(Succeed())
				// Add a pod without node, i.e. pending
				pod3 := newPod("podadb3", ADBNamespace, "", podLabels)
				Expect(k8sClient.Create(ctx, &pod3)).Should(Succeed())
				pvc3 := newPVC("pvc3", ADBNamespace, "node3-pv-local", podLabels)
				Expect(k8sClient.Create(ctx, pvc3.DeepCopy())).Should(Succeed())
				Expect(k8sClient.Status().Update(ctx, pvc3.DeepCopy())).Should(Succeed())

				By("creating a budget that accepts one disruption")
				ndb := &nodedisruptionv1alpha1.ApplicationDisruptionBudget{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "nodedisruption.criteo.com/v1alpha1",
						Kind:       "ApplicationDisruptionBudget",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      ADBname,
						Namespace: ADBNamespace,
					},
					Spec: nodedisruptionv1alpha1.ApplicationDisruptionBudgetSpec{
						PodSelector:    metav1.LabelSelector{MatchLabels: podLabels},
						PVCSelector:    metav1.LabelSelector{MatchLabels: podLabels},
						MaxDisruptions: 1,
					},
				}
				Expect(k8sClient.Create(ctx, ndb)).Should(Succeed())

				By("checking the ApplicationDisruptionBudget in synchronized")
				ADBLookupKey := types.NamespacedName{Name: ADBname, Namespace: ADBNamespace}
				createdADB := &nodedisruptionv1alpha1.ApplicationDisruptionBudget{}
				Eventually(func() []string {
					err := k8sClient.Get(ctx, ADBLookupKey, createdADB)
					Expect(err).Should(Succeed())
					return createdADB.Status.WatchedNodes
				}, timeout, interval).Should(Equal([]string{"node1", "node2", "node3"}))

				By("Adding Node Disruption")
				disruption := &nodedisruptionv1alpha1.NodeDisruption{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "nodedisruption.criteo.com/v1alpha1",
						Kind:       "NodeDisruption",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-adb-1",
						Namespace: "",
					},
					Spec: nodedisruptionv1alpha1.NodeDisruptionSpec{
						NodeSelector: metav1.LabelSelector{MatchLabels: nodeLabels1},
					},
				}
				Expect(k8sClient.Create(ctx, disruption.DeepCopy())).Should(Succeed())

				By("checking the ApplicationDisruptionBudget updated the status")
				Eventually(func() []nodedisruptionv1alpha1.Disruption {
					err := k8sClient.Get(ctx, ADBLookupKey, createdADB)
					Expect(err).Should(Succeed())
					return createdADB.Status.Disruptions
				}, timeout, interval).Should(Equal([]nodedisruptionv1alpha1.Disruption{{
					Name:  "test-adb-1",
					State: "granted",
				}}))

			})
		})

		When("PV doesn't have an affinity", func() {
			It("is ignored by ADB", func() {
				By("Creating PV without NodeAffinity")
				resources := make(corev1.ResourceList, 1)
				resources[corev1.ResourceStorage] = *resource.NewQuantity(100, resources.Storage().Format)

				PVWithoutAffinity := &corev1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: "remote-pv",
					},
					Spec: corev1.PersistentVolumeSpec{
						Capacity:               resources,
						AccessModes:            []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						PersistentVolumeSource: corev1.PersistentVolumeSource{CSI: &corev1.CSIPersistentVolumeSource{Driver: "test", VolumeHandle: "test"}},
						NodeAffinity:           nil,
					},
				}
				Expect(k8sClient.Create(ctx, PVWithoutAffinity)).Should(Succeed())

				By("Adding PVC")
				pvc := newPVC("pvc", ADBNamespace, "remote-pv", podLabels)
				Expect(k8sClient.Create(ctx, pvc.DeepCopy())).Should(Succeed())
				Expect(k8sClient.Status().Update(ctx, pvc.DeepCopy())).Should(Succeed())

				By("creating a budget that accepts one disruption")
				adb := &nodedisruptionv1alpha1.ApplicationDisruptionBudget{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "nodedisruption.criteo.com/v1alpha1",
						Kind:       "ApplicationDisruptionBudget",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      ADBname,
						Namespace: ADBNamespace,
					},
					Spec: nodedisruptionv1alpha1.ApplicationDisruptionBudgetSpec{
						PodSelector:    metav1.LabelSelector{MatchLabels: podLabels},
						PVCSelector:    metav1.LabelSelector{MatchLabels: podLabels},
						MaxDisruptions: 1,
					},
				}
				Expect(k8sClient.Create(ctx, adb)).Should(Succeed())

				By("checking the ApplicationDisruptionBudget updated the status and has seen no nodes")
				Eventually(func() int {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Namespace: ADBNamespace,
						Name:      ADBname,
					}, adb)
					Expect(err).Should(Succeed())
					return adb.Status.DisruptionsAllowed
				}, timeout, interval).Should(Equal(1))

				Expect(adb.Status.WatchedNodes).Should(BeEmpty())
			})
		})

		When("PVC is pending", func() {
			It("is ignored by ADB", func() {
				By("Adding PVC")
				pvc := newPVC("pvc", ADBNamespace, "remote-pv", podLabels)
				pvc.Status.Phase = corev1.ClaimPending
				Expect(k8sClient.Create(ctx, pvc.DeepCopy())).Should(Succeed())
				Expect(k8sClient.Status().Update(ctx, pvc.DeepCopy())).Should(Succeed())

				By("creating a budget that accepts one disruption")
				adb := &nodedisruptionv1alpha1.ApplicationDisruptionBudget{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "nodedisruption.criteo.com/v1alpha1",
						Kind:       "ApplicationDisruptionBudget",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      ADBname,
						Namespace: ADBNamespace,
					},
					Spec: nodedisruptionv1alpha1.ApplicationDisruptionBudgetSpec{
						PodSelector:    metav1.LabelSelector{MatchLabels: podLabels},
						PVCSelector:    metav1.LabelSelector{MatchLabels: podLabels},
						MaxDisruptions: 1,
					},
				}
				Expect(k8sClient.Create(ctx, adb)).Should(Succeed())

				By("checking the ApplicationDisruptionBudget updated the status and has seen no nodes")
				Eventually(func() int {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Namespace: ADBNamespace,
						Name:      ADBname,
					}, adb)
					Expect(err).Should(Succeed())
					return adb.Status.DisruptionsAllowed
				}, timeout, interval).Should(Equal(1))

				Expect(adb.Status.WatchedNodes).Should(BeEmpty())
			})
		})
	})
})
