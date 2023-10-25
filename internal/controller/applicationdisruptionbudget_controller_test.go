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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// +kubebuilder:docs-gen:collapse=Imports
var _ = Describe("ApplicationDisruptionBudget controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		ADBname      = "test-adb"
		ADBNamespace = "test-adb"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	var (
		podLabels = map[string]string{
			"testselectpod": "testadb",
		}

		_, cancelFn = context.WithCancel(context.Background())
	)

	Context("In a cluster with several nodes", func() {
		ctx := context.Background()
		It("Create namespace and pods", func() {
			By("Creating the namespace")
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ADBNamespace,
					Namespace: ADBNamespace,
				},
			}
			Expect(k8sClient.Create(ctx, ns)).Should(Succeed())

			By("Adding Pods")
			pod1 := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "podadb1",
					Namespace: ADBNamespace,
					Labels:    podLabels,
				},
				Spec: corev1.PodSpec{
					NodeName: "node1",
					Containers: []corev1.Container{
						{
							Name:  "testcontainer",
							Image: "ubuntu",
						},
					},
				},
				Status: corev1.PodStatus{},
			}
			Expect(k8sClient.Create(ctx, pod1)).Should(Succeed())
		})

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
				clearAllNodeDisruptionRessources()
			})

			When("there are no budgets in the cluster", func() {
				It("grants the node disruption", func() {
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

					By("Adding Pods")
					pod2 := &corev1.Pod{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "podadb2",
							Namespace: ADBNamespace,
							Labels:    podLabels,
						},
						Spec: corev1.PodSpec{
							NodeName: "node2",
							Containers: []corev1.Container{
								{
									Name:  "testcontainer",
									Image: "ubuntu",
								},
							},
						},
						Status: corev1.PodStatus{},
					}
					Expect(k8sClient.Create(ctx, pod2)).Should(Succeed())

					By("checking the ApplicationDisruptionBudget updated the status")
					Eventually(func() []string {
						err := k8sClient.Get(ctx, ADBLookupKey, createdADB)
						Expect(err).Should(Succeed())
						return createdADB.Status.WatchedNodes
					}, timeout, interval).Should(Equal([]string{"node1", "node2"}))

					By("Adding PVCs")
					ressources := make(corev1.ResourceList, 1)
					ressources[corev1.ResourceStorage] = *resource.NewQuantity(100, ressources.Memory().Format)
					pvc3 := &corev1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pvc3",
							Namespace: ADBNamespace,
							Labels:    podLabels,
						},
						Spec: corev1.PersistentVolumeClaimSpec{
							VolumeName:  "node3-pv-local",
							AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							Resources:   corev1.ResourceRequirements{Requests: ressources},
						},
						Status: corev1.PersistentVolumeClaimStatus{},
					}
					Expect(k8sClient.Create(ctx, pvc3)).Should(Succeed())

					By("checking the ApplicationDisruptionBudget updated the status")
					Eventually(func() []string {
						err := k8sClient.Get(ctx, ADBLookupKey, createdADB)
						Expect(err).Should(Succeed())
						return createdADB.Status.WatchedNodes
					}, timeout, interval).Should(Equal([]string{"node1", "node2", "node3"}))
				})
			})

		})

	})
})
