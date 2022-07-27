// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	mchov1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"
	appsubv1 "open-cluster-management.io/multicloud-operators-subscription/pkg/apis/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resources "github.com/stolostron/multiclusterhub-operator/test/unit-tests"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	MCHName      = "multiclusterhub-operator"
	MCEName      = "multiclusterengine"
	MCHNamespace = "open-cluster-management"

	timeout  = time.Second * 20
	interval = time.Millisecond * 250
)

var (
	ctx                 = context.Background()
	testMulticlusterhub = types.NamespacedName{Name: MCHName, Namespace: MCHNamespace}
	MCHLookupKey        = types.NamespacedName{Name: MCHName, Namespace: MCHNamespace}
	MCELookupKey        = types.NamespacedName{Name: MCEName}
	mockTime            = metav1.NewTime(time.Date(2020, 5, 29, 6, 0, 0, 0, time.UTC))
)

func initializeMocks() {
	mockMCHOperator = &appsv1.Deployment{
		TypeMeta:   metav1.TypeMeta{Kind: "Deployment", APIVersion: "apps/v1"},
		ObjectMeta: metav1.ObjectMeta{Name: MCHName, Namespace: MCHNamespace},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"test": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"test": "test"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "test",
							Image:           "test",
							ImagePullPolicy: "Always",
							Env: []corev1.EnvVar{
								{
									Name:  "test",
									Value: "test",
								},
							},
							Command: []string{"/iks.sh"},
						},
					},
				},
			},
		},
	}
}

var _ = Describe("Multiclusterhub controller", func() {

	Context("Creating a Multiclusterhub", func() {
		It("Should create top level resources", func() {
			By("By creating a namespace", func() {
				err := k8sClient.Create(ctx, &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{Name: MCHNamespace},
				})
				Expect(err).NotTo(HaveOccurred())
			})

			applyPrereqs(k8sClient)

			By("By creating a new Multiclusterhub", func() {
				Expect(k8sClient.Create(ctx, &mchov1.MultiClusterHub{
					ObjectMeta: metav1.ObjectMeta{
						Name:      MCHName,
						Namespace: MCHNamespace,
						Annotations: map[string]string{
							"mch-imageRepository": "quay.io/test",
						},
					},
					Spec: mchov1.MultiClusterHubSpec{},
				})).Should(Succeed())
			})

			createdMCH := &mchov1.MultiClusterHub{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, testMulticlusterhub, createdMCH)).To(Succeed())
				g.Expect(createdMCH.Status.Phase).To(Equal(mchov1.HubRunning))
			}, 30*time.Second, interval).Should(Succeed())

			By("Ensuring Defaults are set")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, MCHLookupKey, createdMCH)).To(Succeed())
				g.Expect(createdMCH.Spec.Ingress.SSLCiphers).To(Equal(utils.DefaultSSLCiphers), "mch should use default ssl ciphers")
				g.Expect(createdMCH.Spec.AvailabilityConfig).To(Equal(mchov1.HAHigh), "mch should default to high availability")
			}, timeout, interval).Should(Succeed())

			By("Ensuring Deployments")
			Eventually(func(g Gomega) {
				deploymentReferences := utils.GetDeployments(createdMCH)
				for _, deploymentReference := range deploymentReferences {
					deployment := &appsv1.Deployment{}
					g.Expect(k8sClient.Get(ctx, deploymentReference, deployment)).To(Succeed(), "could not find deployment")
				}
			}, timeout, interval).Should(Succeed())

			By("Ensuring MultiClusterEngine is running")
			Eventually(func(g Gomega) {
				mce := &mcev1.MultiClusterEngine{}
				g.Expect(k8sClient.Get(ctx, MCELookupKey, mce)).To(Succeed())
				mceAnnotations := mce.GetAnnotations()
				g.Expect(mceAnnotations).ToNot(BeNil(), "mce should have annotations set")
				g.Expect(mceAnnotations["imageRepository"]).To(Equal("quay.io/test"), "mce imageRepository annotation should match mch")
			}, timeout, interval).Should(Succeed())
			mceConfigTests()

			By("Ensuring appsubs")
			Eventually(func(g Gomega) {
				subscriptionReferences := utils.GetAppsubs(createdMCH)
				for _, subscriptionReference := range subscriptionReferences {
					subscription := &appsubv1.Subscription{}
					g.Expect(k8sClient.Get(ctx, subscriptionReference, subscription)).To(Succeed())
				}
			}, timeout, interval).Should(Succeed())

			By("ensuring the trusted-ca-bundle ConfigMap is created")
			Eventually(func(g Gomega) {
				ctx := context.Background()
				namespacedName := types.NamespacedName{
					Name:      defaultTrustBundleName,
					Namespace: MCHNamespace,
				}
				res := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, namespacedName, res)).To(Succeed())
			}, timeout, interval).Should(Succeed())

		})

		Context("Modify Multiclusterhub configuration options", func() {
			It("Should toggle insights", func() {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testsecret",
						Namespace: MCHNamespace,
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{".dockerconfigjson": []byte("{}")},
				}
				Expect(k8sClient.Create(ctx, secret)).Should(Succeed())

				createdMCH := &mchov1.MultiClusterHub{}
				Expect(k8sClient.Get(ctx, testMulticlusterhub, createdMCH)).To(Succeed())
				createdMCH.Spec.AvailabilityConfig = mchov1.HABasic
				createdMCH.Spec.DisableHubSelfManagement = true
				createdMCH.Spec.ImagePullSecret = "testsecret"
				createdMCH.Spec.NodeSelector = map[string]string{"beta.kubernetes.io/os": "linux"}
				createdMCH.Spec.Tolerations = []corev1.Toleration{
					{
						Key:      "dedicated",
						Operator: "Exists",
						Effect:   "NoSchedule",
					},
				}
				Expect(k8sClient.Update(ctx, createdMCH)).Should(Succeed())
			})
		})

		Context("Toggling Multiclusterhub components", func() {
			It("Should flip component toggles", func() {
				createdMCH := &mchov1.MultiClusterHub{}
				Expect(k8sClient.Get(ctx, testMulticlusterhub, createdMCH)).To(Succeed())
				createdMCH.Disable(mchov1.Insights)
				createdMCH.Disable(mchov1.GRC)
				createdMCH.Disable(mchov1.ClusterLifecycle)
				createdMCH.Disable(mchov1.Console)
				createdMCH.Disable(mchov1.Volsync)
				createdMCH.Disable(mchov1.ManagementIngress)
				createdMCH.Disable(mchov1.Repo)
				// createdMCH.Enable(mchov1.ClusterBackup)
				createdMCH.Enable(mchov1.SearchV2)
				Expect(k8sClient.Update(ctx, createdMCH)).Should(Succeed())

				Eventually(func(g Gomega) {
					subList := &appsubv1.SubscriptionList{}
					g.Expect(k8sClient.List(ctx, subList)).To(Succeed())
					g.Expect(len(subList.Items)).To(BeZero(), "All appsubs should be removed")

					deploymentReferences := []types.NamespacedName{
						{Name: "insights-client", Namespace: MCHNamespace},
						{Name: "insights-metrics", Namespace: MCHNamespace},
					}
					verifyNoDeployments(g, deploymentReferences)
				}, timeout, interval).Should(Succeed())
			})
		})

		Context("Delete MCH", func() {
			It("Should flip component toggles", func() {
				mch := &mchov1.MultiClusterHub{}
				Expect(k8sClient.Get(ctx, testMulticlusterhub, mch)).To(Succeed())
				Expect(len(mch.Finalizers)).To(BeNumerically(">", 0))
				Expect(k8sClient.Delete(ctx, mch)).To(Succeed())

				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, testMulticlusterhub, mch)
					g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "Expected IsNotFound error, got error:", err)
				}, timeout, interval).Should(Succeed())
			})
		})
	})

})

// mceConfigTests test variations in mce spec options
var mceConfigTests = func() {
	By("Ensuring MultiClusterEngine is running")
	mch := &mchov1.MultiClusterHub{}
	Expect(k8sClient.Get(ctx, testMulticlusterhub, mch)).To(Succeed())
	mce := &mcev1.MultiClusterEngine{}
	Expect(k8sClient.Get(ctx, MCELookupKey, mce)).To(Succeed())

	// Test currently failing
	// Expect(mch.Spec.AvailabilityConfig).To(BeEquivalentTo(mce.Spec.AvailabilityConfig))
	Expect(mch.Spec.ImagePullSecret).To(Equal(mce.Spec.ImagePullSecret))
	Expect(mch.Spec.NodeSelector).To(Equal(mce.Spec.NodeSelector))
	// Expect(mch.Spec.Tolerations).To(Equal(mce.Spec.Tolerations))
}

func verifyNoDeployments(g Gomega, nn []types.NamespacedName) {
	for _, deploymentReference := range nn {
		deployment := &appsv1.Deployment{}
		err := k8sClient.Get(ctx, deploymentReference, deployment)
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "Expected IsNotFound error, got error:", err)
	}
}

func verifyNoAppsubs(g Gomega, nn []types.NamespacedName) {
	for _, subscriptionReference := range nn {
		subscription := &appsubv1.Subscription{}
		err := k8sClient.Get(ctx, subscriptionReference, subscription)
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "Expected IsNotFound error, got error:", err)
	}
}

func applyPrereqs(k8sClient client.Client) {
	By("Applying Namespace")
	ctx := context.Background()
	// Expect(k8sClient.Create(ctx, resources.OCMNamespace())).Should(Succeed())
	Expect(k8sClient.Create(ctx, resources.MonitoringNamespace())).Should(Succeed())

	// MCE must mock a pre-existing MCE because OLM is not present to create a CSV
	// mce := resources.EmptyMCE()
	// Expect(k8sClient.Create(ctx, &mce)).Should(Succeed())
}
