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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	nodedisruptionv1alpha1 "github.com/criteo/node-disruption-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:docs-gen:collapse=Imports

func clearAllNodeDisruptionRessources() {
	// It doesn't seem possible to wipe in all namespace so we walk through all of them
	namespaces := &corev1.NamespaceList{}
	err := k8sClient.List(context.Background(), namespaces)
	if err != nil {
		panic(err)
	}

	for _, namespace := range namespaces.Items {
		opts := client.DeleteAllOfOptions{
			ListOptions: client.ListOptions{Namespace: namespace.Name},
		}

		_ = k8sClient.DeleteAllOf(context.Background(), &nodedisruptionv1alpha1.ApplicationDisruptionBudget{}, &opts)
	}

	_ = k8sClient.DeleteAllOf(context.Background(), &nodedisruptionv1alpha1.NodeDisruptionBudget{})
	_ = k8sClient.DeleteAllOf(context.Background(), &nodedisruptionv1alpha1.NodeDisruption{})

}

func startReconcilerWithConfig(config NodeDisruptionReconcilerConfig) context.CancelFunc {
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
	err = (&ApplicationDisruptionBudgetReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())
	err = (&NodeDisruptionBudgetReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	managerCtx, cancel := context.WithCancel(context.Background())

	shutdownChan := make(chan bool, 1)

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(managerCtx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
		shutdownChan <- true
	}()

	return func() {
		cancel()
		// Ensure the manager is actually stopped to avoid starting a new manager too early
		<-shutdownChan
	}
}

func startDummyHTTPServer(handle http.HandlerFunc, listenAddr string) (cancelFn func()) {
	testServer := http.NewServeMux()
	srv := &http.Server{Addr: listenAddr, Handler: testServer}
	testServer.HandleFunc("/", handle)
	go func() {
		defer GinkgoRecover()
		_ = srv.ListenAndServe()
	}()
	return func() { _ = srv.Shutdown(context.Background()) }
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
		NDLookupKey = types.NamespacedName{Name: NDName, Namespace: NDNamespace}

		_, cancelFn = context.WithCancel(context.Background())
	)

	Context("In a cluster with pods", func() {
		ctx := context.Background()
		It("Create pods", func() {
			By("Adding Pods")
			pod1 := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: "default",
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
							NodeSelector: metav1.LabelSelector{MatchLabels: nodeLabels1},
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

			When("there are no budgets in the cluster", func() {
				It("calls the lifecycle hook", func() {
					mockHost := "localhost:8120"
					mockURL := "/testurl"

					By("Starting an http server to receive the hook")
					var (
						hookBody []byte
						hookURL  string
					)
					hookCallCount := 0

					checkHookFn := func(w http.ResponseWriter, req *http.Request) {
						var err error
						hookBody, err = io.ReadAll(req.Body)
						Expect(err).Should(Succeed())
						hookURL = req.URL.String()
						hookCallCount++
						w.WriteHeader(http.StatusOK)
					}

					httpCancel := startDummyHTTPServer(checkHookFn, mockHost)
					defer httpCancel()

					By("creating a budget that accepts one disruption")
					ndb := &nodedisruptionv1alpha1.ApplicationDisruptionBudget{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "nodedisruption.criteo.com/v1alpha1",
							Kind:       "ApplicationDisruptionBudget",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test",
							Namespace: "default",
						},
						Spec: nodedisruptionv1alpha1.ApplicationDisruptionBudgetSpec{
							PodSelector:    metav1.LabelSelector{MatchLabels: podLabels},
							MaxDisruptions: 1,
							HealthHook: nodedisruptionv1alpha1.HealthHookSpec{
								URL: fmt.Sprintf("http://%s%s", mockHost, mockURL),
							},
						},
					}
					Expect(k8sClient.Create(ctx, ndb)).Should(Succeed())

					By("checking the ApplicationDisruptionBudget in synchronized")
					ADBLookupKey := types.NamespacedName{Name: "test", Namespace: "default"}
					createdADB := &nodedisruptionv1alpha1.ApplicationDisruptionBudget{}
					Eventually(func() []string {
						err := k8sClient.Get(ctx, ADBLookupKey, createdADB)
						Expect(err).Should(Succeed())
						return createdADB.Status.WatchedNodes
					}, timeout, interval).Should(Equal([]string{"node1"}))

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
							NodeSelector: metav1.LabelSelector{MatchLabels: nodeLabels1},
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

					By("checking that the lifecyclehook was properly called")
					Expect(hookCallCount).Should(Equal(1))
					Expect(hookURL).Should(Equal(mockURL))
					HookDisruption := &nodedisruptionv1alpha1.NodeDisruption{}
					Expect(json.Unmarshal(hookBody, HookDisruption)).Should(Succeed())
					Expect(HookDisruption.Name).Should(Equal(disruption.Name))
				})
			})

			When("there is a budget that doesn't support any disruption", func() {
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
							NodeSelector:      metav1.LabelSelector{MatchLabels: nodeLabels1},
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
							NodeSelector: metav1.LabelSelector{MatchLabels: nodeLabels1},
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
							NodeSelector:      metav1.LabelSelector{MatchLabels: nodeLabels1},
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
							NodeSelector: metav1.LabelSelector{MatchLabels: nodeLabels1},
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
							NodeSelector: metav1.LabelSelector{MatchLabels: nodeLabels1},
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
				cancelFn()
			})

			When("RejectEmptyNodeDisruption is disabled", func() {
				It("grants the NodeDisruption", func() {
					By("starting a reconciler with RejectEmptyNodeDisruption disabled")
					cancelFn = startReconcilerWithConfig(NodeDisruptionReconcilerConfig{
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
					cancelFn = startReconcilerWithConfig(NodeDisruptionReconcilerConfig{
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
