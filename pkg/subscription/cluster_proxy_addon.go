// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package subscription

import (
	operatorsv1 "github.com/open-cluster-management/multiclusterhub-operator/api/v1"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ClusterProxyAddon overrides the cluster-proxy-addon chart
func ClusterProxyAddon(m *operatorsv1.MultiClusterHub, overrides map[string]string) *unstructured.Unstructured {
	sub := &Subscription{
		Name:      "cluster-proxy-addon",
		Namespace: m.Namespace,
		Overrides: map[string]interface{}{
			"pullSecret": m.Spec.ImagePullSecret,
			"hubconfig": map[string]interface{}{
				"replicaCount": utils.DefaultReplicaCount(m),
				"nodeSelector": m.Spec.NodeSelector,
			},
			"global": map[string]interface{}{
				"imageOverrides": overrides,
				"pullPolicy":     utils.GetImagePullPolicy(m),
			},
		},
	}
	setCustomCA(m, sub)

	return newSubscription(m, sub)
}