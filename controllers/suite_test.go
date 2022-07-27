// Copyright Contributors to the Open Cluster Management project

/*
Copyright 2021.

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

package controllers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	appsub "open-cluster-management.io/multicloud-operators-subscription/pkg/apis"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	netv1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/operator/v1"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	mchov1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var signalHandlerContext context.Context

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	// SetupSignalHandler can only be called once, so we'll save the
	// context it returns and reuse it each time we start a new
	// manager.
	signalHandlerContext = ctrl.SetupSignalHandler()

	By("bootstrap test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "config", "crd", "bases"),
			filepath.Join("..", "test", "unit-tests", "crds"),
		},
		CRDInstallOptions: envtest.CRDInstallOptions{
			CleanUpAfterUse: true,
		},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	Expect(mchov1.AddToScheme(scheme.Scheme)).Should(Succeed())
	Expect(appsub.AddToScheme(scheme.Scheme)).Should(Succeed())
	Expect(apiregistrationv1.AddToScheme(scheme.Scheme)).Should(Succeed())
	Expect(apixv1.AddToScheme(scheme.Scheme)).Should(Succeed())
	Expect(netv1.AddToScheme(scheme.Scheme)).Should(Succeed())
	Expect(olmv1.AddToScheme(scheme.Scheme)).Should(Succeed())
	Expect(subv1alpha1.AddToScheme(scheme.Scheme)).Should(Succeed())
	Expect(mcev1.AddToScheme(scheme.Scheme)).Should(Succeed())
	Expect(configv1.AddToScheme(scheme.Scheme)).Should(Succeed())
	Expect(consolev1.AddToScheme(scheme.Scheme)).Should(Succeed())
	Expect(promv1.AddToScheme(scheme.Scheme)).Should(Succeed())
	Expect(scheme.AddToScheme(scheme.Scheme)).Should(Succeed())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 scheme.Scheme,
		MetricsBindAddress:     "0",
		HealthProbeBindAddress: "0",
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&MultiClusterHubReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
		Log:    ctrl.Log.WithName("controllers").WithName("MultiClusterHub"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	Expect(os.Setenv("UNIT_TEST", "true")).To(Succeed())
	Expect(os.Setenv("POD_NAMESPACE", "open-cluster-management")).To(Succeed())
	Expect(os.Setenv("CRDS_PATH", "../bin/crds")).To(Succeed())
	Expect(os.Setenv("TEMPLATES_PATH", "../pkg/templates")).To(Succeed())
	Expect(os.Setenv("DIRECTORY_OVERRIDE", "../pkg/templates")).To(Succeed())
	Expect(os.Setenv("OPERATOR_PACKAGE", "advanced-cluster-management")).To(Succeed())
	for _, v := range utils.GetTestImages() {
		key := fmt.Sprintf("OPERAND_IMAGE_%s", strings.ToUpper(v))
		err := os.Setenv(key, "quay.io/test/test:test")
		Expect(err).NotTo(HaveOccurred())
	}

	go func() {
		// For explanation of GinkgoRecover in a go routine, see
		// https://onsi.github.io/ginkgo/#mental-model-how-ginkgo-handles-failure
		defer GinkgoRecover()
		err = k8sManager.Start(signalHandlerContext)
		Expect(err).ToNot(HaveOccurred())
	}()
})

var _ = AfterSuite(func() {
	// mch := &mchov1.MultiClusterHub{}
	// Expect(k8sClient.Get(ctx, testMulticlusterhub, mch)).To(Succeed())
	// fmt.Printf("%+v\n", mch.Spec)
	// fmt.Printf("%+v\n", mch.Status)

	// mce := &mcev1.MultiClusterEngine{}
	// Expect(k8sClient.Get(ctx, MCELookupKey, mce)).To(Succeed())
	// fmt.Printf("%+v\n", mce.Spec)
	// fmt.Printf("%+v\n", mce.Status)

	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
	Expect(os.Unsetenv("UNIT_TEST")).To(Succeed())
	Expect(os.Unsetenv("POD_NAMESPACE")).To(Succeed())
	Expect(os.Unsetenv("CRDS_PATH")).To(Succeed())
	Expect(os.Unsetenv("TEMPLATES_PATH")).To(Succeed())
	Expect(os.Unsetenv("DIRECTORY_OVERRIDE")).To(Succeed())
	Expect(os.Unsetenv("OPERATOR_PACKAGE")).To(Succeed())
})
